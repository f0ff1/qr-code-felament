package printer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"filamenttracker/internal/domain"
	cloudaccount "filamenttracker/internal/domain/cloudaccount"
	printerdomain "filamenttracker/internal/domain/printer"
	printjobdomain "filamenttracker/internal/domain/printjob"
	"filamenttracker/internal/infrastructure/printer/bambu"
	"filamenttracker/internal/infrastructure/secrets"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, p printerdomain.Printer) error
	GetByID(ctx context.Context, id uuid.UUID) (printerdomain.Printer, error)
	List(ctx context.Context) ([]printerdomain.Printer, error)
	Update(ctx context.Context, p printerdomain.Printer) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type CloudAccountRepository interface {
	Upsert(ctx context.Context, account cloudaccount.Account) error
	GetLatest(ctx context.Context) (cloudaccount.Account, error)
	DeleteAll(ctx context.Context) error
}

type CreateInput struct {
	Name            string
	Model           string
	LANHost         string
	LANSerial       string
	LANAccessCode   string
	LANEnabled      bool
	CloudEnabled    bool
	CloudEmail      string
	CloudPassword   string
	CloudToken      string
	CloudRegion     string
	CloudVerifyCode string
	DefaultSpoolID  uuid.UUID
}

type Service struct {
	repo     Repository
	accounts CloudAccountRepository
	adapter  printerdomain.Adapter
	secrets  *secrets.Box
}

func NewService(repo Repository, adapter printerdomain.Adapter, accounts CloudAccountRepository) *Service {
	return &Service{repo: repo, adapter: adapter, accounts: accounts, secrets: secrets.NewBoxFromEnv()}
}

func NewServiceWithSecrets(repo Repository, adapter printerdomain.Adapter, accounts CloudAccountRepository, box *secrets.Box) *Service {
	if box == nil {
		box = secrets.NewBoxFromEnv()
	}
	return &Service{repo: repo, adapter: adapter, accounts: accounts, secrets: box}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (printerdomain.Printer, error) {
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.Model) == "" {
		return printerdomain.Printer{}, fmt.Errorf("%w: printer name and model are required", domain.ErrInvalid)
	}
	if input.LANEnabled && input.CloudEnabled {
		return printerdomain.Printer{}, fmt.Errorf("%w: choose either LAN or Cloud connection", domain.ErrInvalid)
	}
	if err := validateConnection(input); err != nil {
		return printerdomain.Printer{}, err
	}

	p := printerdomain.NewPrinter(strings.TrimSpace(input.Name), strings.TrimSpace(input.Model))
	applyConnection(&p, input)

	if p.CloudEnabled {
		if _, err := authenticateCloud(&p, input.CloudVerifyCode); err != nil {
			return printerdomain.Printer{}, err
		}
		if err := s.sealPrinterCloudSecrets(&p); err != nil {
			return printerdomain.Printer{}, err
		}
	}

	if err := s.repo.Create(ctx, p); err != nil {
		return printerdomain.Printer{}, err
	}
	return p, nil
}

func (s *Service) UpdateLAN(ctx context.Context, id uuid.UUID, input CreateInput) (printerdomain.Printer, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return printerdomain.Printer{}, err
	}
	if input.LANEnabled && input.CloudEnabled {
		return printerdomain.Printer{}, fmt.Errorf("%w: choose either LAN or Cloud connection", domain.ErrInvalid)
	}
	if strings.TrimSpace(input.Name) != "" {
		p.Name = strings.TrimSpace(input.Name)
	}
	if strings.TrimSpace(input.Model) != "" {
		p.Model = strings.TrimSpace(input.Model)
	}
	if input.CloudEnabled && strings.TrimSpace(input.CloudPassword) == "" {
		input.CloudPassword = s.openSecret(p.CloudPassword)
	}
	if input.CloudEnabled && strings.TrimSpace(input.CloudToken) == "" {
		input.CloudToken = s.openSecret(p.CloudToken)
	}
	if input.LANEnabled && strings.TrimSpace(input.LANAccessCode) == "" {
		input.LANAccessCode = p.LANAccessCode
	}
	if err := validateConnection(input); err != nil {
		return printerdomain.Printer{}, err
	}
	applyConnection(&p, input)
	p.UpdatedAt = time.Now()

	if p.CloudEnabled && !p.CloudLinked() {
		if _, err := authenticateCloud(&p, input.CloudVerifyCode); err != nil {
			return printerdomain.Printer{}, err
		}
	}
	if p.CloudEnabled {
		if err := s.sealPrinterCloudSecrets(&p); err != nil {
			return printerdomain.Printer{}, err
		}
	}

	if err := s.repo.Update(ctx, p); err != nil {
		return printerdomain.Printer{}, err
	}
	return p, nil
}

type CloudSyncInput struct {
	Email      string
	Password   string
	Region     string
	VerifyCode string
	Token      string
}

type CloudSyncResult struct {
	Printers    []printerdomain.Printer
	Devices     []bambu.CloudDevice
	NeedsVerify bool
	Token       string
}

func (s *Service) SyncFromCloud(ctx context.Context, input CloudSyncInput) (CloudSyncResult, error) {
	region := bambu.NormalizeCloudRegion(input.Region)
	token := strings.TrimSpace(input.Token)
	email := strings.TrimSpace(input.Email)
	password := strings.TrimSpace(input.Password)

	if saved, err := s.loadSavedAccount(ctx); err == nil {
		if email == "" {
			email = saved.Email
		}
		if password == "" {
			password = saved.Password
		}
		if token == "" && input.VerifyCode == "" {
			token = saved.Token
		}
		if strings.TrimSpace(input.Region) == "" {
			region = bambu.NormalizeCloudRegion(saved.Region)
		}
	}

	if token == "" {
		if strings.TrimSpace(input.VerifyCode) != "" {
			verified, err := bambu.CloudVerify(email, input.VerifyCode, region)
			if err != nil {
				return CloudSyncResult{}, fmt.Errorf("%w: %v", domain.ErrInvalid, err)
			}
			token = verified
		} else {
			if email == "" || password == "" {
				return CloudSyncResult{}, fmt.Errorf("%w: cloud email and password are required", domain.ErrInvalid)
			}
			login, err := bambu.CloudLogin(email, password, region)
			if err != nil {
				return CloudSyncResult{}, fmt.Errorf("%w: %v", domain.ErrInvalid, err)
			}
			if login.NeedsVerify {
				// Keep credentials for the verify step; no token yet.
				_ = s.saveAccount(ctx, email, password, "", region)
				return CloudSyncResult{NeedsVerify: true}, nil
			}
			token = login.Token
		}
	}

	devices, err := bambu.ListCloudDevices(token, region)
	if err != nil {
		// Token may be expired — retry with stored password once.
		if password != "" && email != "" {
			login, loginErr := bambu.CloudLogin(email, password, region)
			if loginErr == nil && !login.NeedsVerify && login.Token != "" {
				token = login.Token
				devices, err = bambu.ListCloudDevices(token, region)
			}
		}
		if err != nil {
			return CloudSyncResult{}, fmt.Errorf("%w: list cloud devices: %v", domain.ErrInvalid, err)
		}
	}
	if len(devices) == 0 {
		return CloudSyncResult{Token: token, Devices: devices}, fmt.Errorf("%w: no printers bound to this Bambu account", domain.ErrNotFound)
	}

	if err := s.saveAccount(ctx, email, password, token, region); err != nil {
		return CloudSyncResult{}, err
	}

	existing, err := s.repo.List(ctx)
	if err != nil {
		return CloudSyncResult{}, err
	}
	bySerial := make(map[string]printerdomain.Printer, len(existing))
	for _, p := range existing {
		if serial := strings.ToUpper(strings.TrimSpace(p.LANSerial)); serial != "" {
			bySerial[serial] = p
		}
	}

	out := make([]printerdomain.Printer, 0, len(devices))
	for _, device := range devices {
		if device.Serial == "" {
			continue
		}
		key := strings.ToUpper(device.Serial)
		model := device.Model
		if model == "" {
			model = "Bambu"
		}
		name := device.Name
		if name == "" {
			name = device.Serial
		}

		if p, ok := bySerial[key]; ok {
			p.Name = name
			p.Model = model
			p.LANSerial = device.Serial
			p.LANEnabled = false
			p.CloudEnabled = true
			p.CloudEmail = email
			if password != "" {
				p.CloudPassword = password
			} else {
				p.CloudPassword = s.openSecret(p.CloudPassword)
			}
			p.CloudToken = token
			p.CloudRegion = region
			if device.AccessCode != "" {
				p.LANAccessCode = device.AccessCode
			}
			p.Status = cloudDevicePrinterStatus(device)
			p.UpdatedAt = time.Now()
			if err := s.sealPrinterCloudSecrets(&p); err != nil {
				return CloudSyncResult{}, err
			}
			if err := s.repo.Update(ctx, p); err != nil {
				return CloudSyncResult{}, err
			}
			out = append(out, p)
			continue
		}

		p := printerdomain.NewPrinter(name, model)
		p.LANSerial = device.Serial
		p.LANAccessCode = device.AccessCode
		p.CloudEnabled = true
		p.CloudEmail = email
		p.CloudPassword = password
		p.CloudToken = token
		p.CloudRegion = region
		p.Status = cloudDevicePrinterStatus(device)
		if err := s.sealPrinterCloudSecrets(&p); err != nil {
			return CloudSyncResult{}, err
		}
		if err := s.repo.Create(ctx, p); err != nil {
			return CloudSyncResult{}, err
		}
		out = append(out, p)
	}

	return CloudSyncResult{
		Printers: out,
		Devices:  devices,
		Token:    token,
	}, nil
}

func (s *Service) SyncSavedCloudAccount(ctx context.Context) (CloudSyncResult, error) {
	account, err := s.loadSavedAccount(ctx)
	if err != nil {
		return CloudSyncResult{}, err
	}
	return s.SyncFromCloud(ctx, CloudSyncInput{
		Email:    account.Email,
		Password: account.Password,
		Token:    account.Token,
		Region:   account.Region,
	})
}

func (s *Service) GetCloudAccount(ctx context.Context) (cloudaccount.Account, error) {
	account, err := s.loadSavedAccount(ctx)
	if err != nil {
		return cloudaccount.Account{}, err
	}
	out := account
	out.Password = ""
	out.Token = ""
	if account.Linked() {
		// Sentinel so Account.Linked() stays true without leaking the real token.
		out.Token = "linked"
	}
	return out, nil
}

func (s *Service) LogoutCloud(ctx context.Context) error {
	if s.accounts != nil {
		if err := s.accounts.DeleteAll(ctx); err != nil {
			return err
		}
	}
	printers, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, p := range printers {
		if !p.CloudEnabled && p.CloudEmail == "" && p.CloudToken == "" && p.CloudPassword == "" {
			continue
		}
		p.CloudEnabled = false
		p.CloudEmail = ""
		p.CloudPassword = ""
		p.CloudToken = ""
		p.CloudRegion = "us"
		p.UpdatedAt = now
		if err := s.repo.Update(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) loadSavedAccount(ctx context.Context) (cloudaccount.Account, error) {
	if s.accounts == nil {
		return cloudaccount.Account{}, fmt.Errorf("%w: cloud account store", domain.ErrNotFound)
	}
	account, err := s.accounts.GetLatest(ctx)
	if err != nil {
		return cloudaccount.Account{}, err
	}
	account.Password = s.openSecret(account.Password)
	account.Token = s.openSecret(account.Token)
	return account, nil
}

func (s *Service) saveAccount(ctx context.Context, email, password, token, region string) error {
	if s.accounts == nil {
		return nil
	}
	account, err := s.accounts.GetLatest(ctx)
	if err != nil {
		account = cloudaccount.New(email, "", "", region)
	} else {
		if email != "" {
			account.Email = email
		}
		account.Region = bambu.NormalizeCloudRegion(region)
		account.UpdatedAt = time.Now()
	}
	if password != "" {
		sealed, sealErr := s.sealSecret(password)
		if sealErr != nil {
			return sealErr
		}
		account.Password = sealed
	} else if account.Password != "" && !strings.HasPrefix(account.Password, "enc:v1:") {
		// Re-seal legacy plaintext on touch.
		if sealed, sealErr := s.sealSecret(account.Password); sealErr == nil {
			account.Password = sealed
		}
	}
	if token != "" {
		sealed, sealErr := s.sealSecret(token)
		if sealErr != nil {
			return sealErr
		}
		account.Token = sealed
	}
	if account.ID == uuid.Nil {
		account.ID = uuid.New()
	}
	if account.CreatedAt.IsZero() {
		account.CreatedAt = time.Now()
	}
	return s.accounts.Upsert(ctx, account)
}

func (s *Service) sealSecret(value string) (string, error) {
	if s.secrets == nil {
		return value, nil
	}
	return s.secrets.Seal(value)
}

func (s *Service) openSecret(value string) string {
	if s.secrets == nil {
		return value
	}
	return s.secrets.MustOpen(value)
}

func (s *Service) sealPrinterCloudSecrets(p *printerdomain.Printer) error {
	if p.CloudPassword != "" {
		sealed, err := s.sealSecret(p.CloudPassword)
		if err != nil {
			return err
		}
		p.CloudPassword = sealed
	}
	if p.CloudToken != "" {
		sealed, err := s.sealSecret(p.CloudToken)
		if err != nil {
			return err
		}
		p.CloudToken = sealed
	}
	return nil
}

func cloudDevicePrinterStatus(device bambu.CloudDevice) printerdomain.PrinterStatus {
	if !device.Online {
		return printerdomain.StatusOffline
	}
	status, active := bambu.MapCloudPrintStatus(device.PrintStatus)
	if !active {
		return printerdomain.StatusIdle
	}
	switch status {
	case printjobdomain.StatusPreparing:
		return printerdomain.StatusPreparing
	case printjobdomain.StatusPaused:
		return printerdomain.StatusPaused
	case printjobdomain.StatusFailed:
		return printerdomain.StatusError
	case printjobdomain.StatusCompleted:
		return printerdomain.StatusCompleted
	default:
		return printerdomain.StatusPrinting
	}
}

func (s *Service) VerifyCloud(ctx context.Context, id uuid.UUID, code string) (printerdomain.Printer, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return printerdomain.Printer{}, err
	}
	if !p.CloudEnabled {
		return printerdomain.Printer{}, fmt.Errorf("%w: cloud is not enabled for this printer", domain.ErrInvalid)
	}
	token, err := bambu.CloudVerify(p.CloudEmail, code, p.CloudRegion)
	if err != nil {
		return printerdomain.Printer{}, fmt.Errorf("%w: %v", domain.ErrInvalid, err)
	}
	p.CloudToken = token
	p.UpdatedAt = time.Now()
	if err := s.sealPrinterCloudSecrets(&p); err != nil {
		return printerdomain.Printer{}, err
	}
	if err := s.repo.Update(ctx, p); err != nil {
		return printerdomain.Printer{}, err
	}
	return p, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (printerdomain.Printer, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]printerdomain.Printer, error) {
	return s.repo.List(ctx)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) SyncStatus(ctx context.Context, id uuid.UUID) (printerdomain.Printer, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return printerdomain.Printer{}, err
	}
	status, err := s.adapter.GetStatus(ctx, p)
	if err != nil {
		return printerdomain.Printer{}, err
	}
	p.Status = status
	p.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, p); err != nil {
		return printerdomain.Printer{}, err
	}
	return p, nil
}

func (s *Service) GetProgress(ctx context.Context, id uuid.UUID) (float64, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return 0, err
	}
	return s.adapter.GetProgress(ctx, p)
}

func validateConnection(input CreateInput) error {
	if input.LANEnabled {
		if strings.TrimSpace(input.LANHost) == "" || strings.TrimSpace(input.LANSerial) == "" || strings.TrimSpace(input.LANAccessCode) == "" {
			return fmt.Errorf("%w: LAN host, serial and access code are required when LAN is enabled", domain.ErrInvalid)
		}
	}
	if input.CloudEnabled {
		if strings.TrimSpace(input.LANSerial) == "" {
			return fmt.Errorf("%w: printer serial is required for Cloud", domain.ErrInvalid)
		}
		hasToken := strings.TrimSpace(input.CloudToken) != ""
		hasCreds := strings.TrimSpace(input.CloudEmail) != "" && strings.TrimSpace(input.CloudPassword) != ""
		if !hasToken && !hasCreds {
			return fmt.Errorf("%w: cloud email/password or access token is required", domain.ErrInvalid)
		}
	}
	return nil
}

func applyConnection(p *printerdomain.Printer, input CreateInput) {
	p.LANHost = strings.TrimSpace(input.LANHost)
	p.LANSerial = strings.TrimSpace(input.LANSerial)
	p.LANAccessCode = strings.TrimSpace(input.LANAccessCode)
	p.LANEnabled = input.LANEnabled && !input.CloudEnabled
	p.CloudEnabled = input.CloudEnabled
	p.CloudEmail = strings.TrimSpace(input.CloudEmail)
	p.CloudPassword = strings.TrimSpace(input.CloudPassword)
	p.CloudToken = strings.TrimSpace(input.CloudToken)
	p.CloudRegion = bambu.NormalizeCloudRegion(input.CloudRegion)
	p.DefaultSpoolID = input.DefaultSpoolID
	if !p.CloudEnabled {
		p.CloudEmail = ""
		p.CloudPassword = ""
		p.CloudToken = ""
		p.CloudRegion = "us"
	}
	if !p.LANEnabled {
		p.LANHost = ""
		p.LANAccessCode = ""
		if !p.CloudEnabled {
			p.LANSerial = ""
		}
	}
}

func authenticateCloud(p *printerdomain.Printer, verifyCode string) (bambu.CloudLoginResult, error) {
	if strings.TrimSpace(p.CloudToken) != "" {
		return bambu.CloudLoginResult{Token: p.CloudToken}, nil
	}
	if strings.TrimSpace(verifyCode) != "" {
		token, err := bambu.CloudVerify(p.CloudEmail, verifyCode, p.CloudRegion)
		if err != nil {
			return bambu.CloudLoginResult{}, fmt.Errorf("%w: %v", domain.ErrInvalid, err)
		}
		p.CloudToken = token
		return bambu.CloudLoginResult{Token: token}, nil
	}
	result, err := bambu.CloudLogin(p.CloudEmail, p.CloudPassword, p.CloudRegion)
	if err != nil {
		return bambu.CloudLoginResult{}, fmt.Errorf("%w: %v", domain.ErrInvalid, err)
	}
	if result.NeedsVerify {
		return result, nil
	}
	p.CloudToken = result.Token
	return result, nil
}
