package bootstrap

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"filamenttracker/internal/config"
	spooldomain "filamenttracker/internal/domain/spool"
	"filamenttracker/internal/infrastructure/printer/bambu"
	"filamenttracker/internal/infrastructure/printer/mock"
	"filamenttracker/internal/infrastructure/secrets"
	"filamenttracker/internal/infrastructure/session"
	"filamenttracker/internal/repository/memory"
	postgresrepo "filamenttracker/internal/repository/postgres"
	adminusecase "filamenttracker/internal/usecase/admin"
	authusecase "filamenttracker/internal/usecase/auth"
	inventoryusecase "filamenttracker/internal/usecase/inventory"
	notificationusecase "filamenttracker/internal/usecase/notification"
	predictionusecase "filamenttracker/internal/usecase/prediction"
	printerusecase "filamenttracker/internal/usecase/printer"
	printjobusecase "filamenttracker/internal/usecase/printjob"
	productusecase "filamenttracker/internal/usecase/product"
	runtimeusecase "filamenttracker/internal/usecase/runtime"
	spoolusecase "filamenttracker/internal/usecase/spool"
	"filamenttracker/internal/worker"
)

// Notifier is the cross-cutting event publisher used by HTTP and Bambu monitor.
type Notifier interface {
	Publish(eventType, message string, payload map[string]any)
}

type notifyBridge struct {
	svc *notificationusecase.Service
}

func (n notifyBridge) Publish(eventType, message string, payload map[string]any) {
	if n.svc == nil {
		return
	}
	n.svc.Publish(eventType, message, payload)
}

// App is the composition root for the modular monolith.
type App struct {
	Config config.Settings
	Runtime *Runtime

	Secrets  *secrets.Box
	Sessions *session.Store
	Auth     *authusecase.Service
	AdminService *adminusecase.Service

	SpoolService      *spoolusecase.Service
	PrinterService    *printerusecase.Service
	ProductService    *productusecase.Service
	PrintJobService   *printjobusecase.Service
	InventoryService  *inventoryusecase.Service
	ForecastService      *predictionusecase.FilamentEstimator
	NotificationService *notificationusecase.Service
	RuntimeSynchronizer *runtimeusecase.Synchronizer

	Notifier Notifier
	Monitor  *bambu.Monitor
	Scheduler *worker.Scheduler

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewApp(cfg config.Settings) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	spooldomain.LowWeightGrams = cfg.SpoolLowWeightG

	runtime := NewRuntimeFromSettings(cfg)
	box, err := secrets.NewBoxFromSettings(cfg)
	if err != nil {
		_ = runtime.Close()
		return nil, err
	}

	var (
		spoolRepo        spooldomain.Repository
		printerRepo      printerusecase.Repository
		productRepo      productusecase.ProductRepository
		printJobRepo     printjobusecase.PrintJobRepository
		inventoryRepo    inventoryusecase.InventoryRepository
		cloudAccountRepo printerusecase.CloudAccountRepository
		orgRepo          adminusecase.OrgRepository
		siteRepo         adminusecase.SiteRepository
		userRepo         authusecase.UserRepository
		resetRepo        authusecase.PasswordResetRepository
	)

	if runtime.DB != nil {
		spoolRepo = postgresrepo.NewSpoolRepository(runtime.DB)
		printerRepo = postgresrepo.NewPrinterRepository(runtime.DB)
		productRepo = postgresrepo.NewProductRepository(runtime.DB)
		printJobRepo = postgresrepo.NewPrintJobRepository(runtime.DB)
		inventoryRepo = postgresrepo.NewInventoryRepository(runtime.DB)
		cloudAccountRepo = postgresrepo.NewCloudAccountRepository(runtime.DB)
		orgRepo = postgresrepo.NewOrganizationRepository(runtime.DB)
		siteRepo = postgresrepo.NewSiteRepository(runtime.DB)
		userRepo = postgresrepo.NewUserRepository(runtime.DB)
		resetRepo = postgresrepo.NewPasswordResetRepository(runtime.DB)
		log.Println("using postgres repositories")
	} else {
		spoolRepo = memory.NewRepository()
		printerRepo = memory.NewPrinterRepository()
		productRepo = memory.NewProductRepository()
		printJobRepo = memory.NewPrintJobRepository()
		inventoryRepo = memory.NewInventoryRepository()
		cloudAccountRepo = memory.NewCloudAccountRepository()
		orgRepo = memory.NewOrganizationRepository()
		siteRepo = memory.NewSiteRepository()
		userRepo = memory.NewUserRepository()
		resetRepo = memory.NewPasswordResetRepository()
		log.Println("using in-memory repositories")
	}

	sessions := session.NewStore(cfg.SessionTTL)
	notifierSvc := notificationusecase.NewServiceWithTTL(cfg.NotificationTTL)
	bridge := notifyBridge{notifierSvc}

	authService, err := authusecase.NewService(cfg, sessions, userRepo, resetRepo, bridge)
	if err != nil {
		_ = runtime.Close()
		return nil, err
	}
	adminService := adminusecase.NewService(orgRepo, siteRepo, userRepo, resetRepo, authService)
	ctxBootstrap := context.Background()
	if err := adminService.EnsureDefaults(ctxBootstrap); err != nil {
		_ = runtime.Close()
		return nil, fmt.Errorf("ensure defaults: %w", err)
	}
	if err := authService.BootstrapAdmin(ctxBootstrap); err != nil {
		_ = runtime.Close()
		return nil, fmt.Errorf("bootstrap admin: %w", err)
	}

	printerService := printerusecase.NewServiceWithDeps(printerRepo, mock.Adapter{}, cloudAccountRepo, bambu.NewCloudClientAdapter(), box)
	printJobService := printjobusecase.NewService(printJobRepo, spoolRepo, productRepo, printerRepo)
	monitor := bambu.NewMonitorWithSecrets(printerRepo, printJobService, bridge, box, cfg.BambuMonitorInterval)

	app := &App{
		Config:              cfg,
		Runtime:             runtime,
		Secrets:             box,
		Sessions:            sessions,
		Auth:                authService,
		AdminService:        adminService,
		SpoolService:        spoolusecase.NewService(spoolRepo),
		PrinterService:      printerService,
		ProductService:      productusecase.NewService(productRepo),
		PrintJobService:     printJobService,
		InventoryService:    inventoryusecase.NewService(spoolRepo, inventoryRepo),
		ForecastService:     predictionusecase.NewEstimator(),
		NotificationService: notifierSvc,
		RuntimeSynchronizer: runtimeusecase.NewSynchronizer(spoolRepo, printerRepo, printJobRepo, productRepo, runtime.Redis),
		Notifier:            bridge,
		Monitor:             monitor,
		Scheduler:           worker.NewSchedulerWithDeps(spoolRepo, printJobRepo, notifierSvc, cfg.SchedulerTick),
	}
	return app, nil
}

func (a *App) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	a.cancel = cancel

	a.Monitor.Start(ctx)
	_ = a.Scheduler.Run(ctx)

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ticker := time.NewTicker(a.Config.RuntimeSyncInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				syncCtx, syncCancel := context.WithTimeout(ctx, a.Config.RuntimeSyncTimeout)
				if err := a.RuntimeSynchronizer.Sync(syncCtx); err != nil {
					log.Printf("sync runtime state: %v", err)
				}
				syncCancel()
			}
		}
	}()

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ticker := time.NewTicker(a.Config.CloudSyncInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				syncCtx, syncCancel := context.WithTimeout(ctx, a.Config.CloudSyncTimeout)
				if _, err := a.PrinterService.SyncSavedCloudAccount(syncCtx); err != nil {
					log.Printf("cloud account sync: %v", err)
				} else if a.Monitor != nil {
					a.Monitor.PollOnce(syncCtx)
				}
				syncCancel()
			}
		}
	}()
}

func (a *App) Close() error {
	if a.cancel != nil {
		a.cancel()
	}
	a.wg.Wait()
	if a.Runtime != nil {
		return a.Runtime.Close()
	}
	return nil
}

func (a *App) PricingPublic() map[string]any {
	return map[string]any{
		"spool_low_weight_g":      a.Config.SpoolLowWeightG,
		"printer_power_kw":        a.Config.PrinterPowerKW,
		"labor_per_hour":          a.Config.LaborPerHour,
		"electricity_rate_person": a.Config.ElectricityPerson,
		"electricity_rate_legal":  a.Config.ElectricityLegal,
		"auth_enabled":            a.Auth != nil && a.Auth.Enabled(),
	}
}
