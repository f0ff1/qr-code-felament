package printjob

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"filamenttracker/internal/domain"
	"filamenttracker/internal/domain/filament"
	printerdomain "filamenttracker/internal/domain/printer"
	printjobdomain "filamenttracker/internal/domain/printjob"
	productdomain "filamenttracker/internal/domain/product"
	spooldomain "filamenttracker/internal/domain/spool"

	"github.com/google/uuid"
)

type PrintJobRepository interface {
	Create(ctx context.Context, j printjobdomain.PrintJob) error
	GetByID(ctx context.Context, id uuid.UUID) (printjobdomain.PrintJob, error)
	GetByExternalTaskID(ctx context.Context, printerID uuid.UUID, externalTaskID string) (printjobdomain.PrintJob, error)
	List(ctx context.Context) ([]printjobdomain.PrintJob, error)
	Update(ctx context.Context, j printjobdomain.PrintJob) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type Service struct {
	repo        PrintJobRepository
	spoolRepo   spooldomain.Repository
	productRepo ProductRepository
	printerRepo PrinterRepository
}

type ProductRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (productdomain.Product, error)
	List(ctx context.Context) ([]productdomain.Product, error)
}

type PrinterRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (printerdomain.Printer, error)
}

type BambuSnapshot struct {
	ExternalTaskID  string
	FileName        string
	Progress        float64
	Status          printjobdomain.Status
	RemainingMin    int
	EstimatedWeight int
	MaterialHint    string
	ColorHint       string
	BrandHint       string
}

func NewService(repo PrintJobRepository, spoolRepo spooldomain.Repository, productRepo ProductRepository, printerRepo PrinterRepository) *Service {
	return &Service{repo: repo, spoolRepo: spoolRepo, productRepo: productRepo, printerRepo: printerRepo}
}

func (s *Service) Start(ctx context.Context, printerID, productID, spoolID uuid.UUID) (printjobdomain.PrintJob, error) {
	printerEntity, err := s.printerRepo.GetByID(ctx, printerID)
	if err != nil {
		return printjobdomain.PrintJob{}, err
	}
	productEntity, err := s.productRepo.GetByID(ctx, productID)
	if err != nil {
		return printjobdomain.PrintJob{}, err
	}
	spoolEntity, err := s.spoolRepo.GetByID(ctx, spoolID)
	if err != nil {
		return printjobdomain.PrintJob{}, err
	}
	if productEntity.EstimatedWeight > spoolEntity.CurrentWeight {
		return printjobdomain.PrintJob{}, fmt.Errorf("%w: insufficient filament for product", domain.ErrInvalid)
	}

	if err := s.reserveFilament(ctx, &spoolEntity, productEntity.EstimatedWeight); err != nil {
		return printjobdomain.PrintJob{}, err
	}

	job := printjobdomain.NewPrintJob(printerEntity.ID, productEntity.ID, spoolEntity.ID, productEntity.EstimatedWeight)
	job.Start()
	job.ConsumedWeight = productEntity.EstimatedWeight
	job.FileName = productEntity.Name
	if err := s.repo.Create(ctx, job); err != nil {
		_ = s.refundFilament(ctx, spoolEntity.ID, productEntity.EstimatedWeight)
		return printjobdomain.PrintJob{}, err
	}
	return job, nil
}

func (s *Service) SyncFromBambu(ctx context.Context, printer printerdomain.Printer, snap BambuSnapshot) (printjobdomain.PrintJob, bool, error) {
	if snap.ExternalTaskID == "" {
		snap.ExternalTaskID = "local-" + printer.ID.String()
	}
	if snap.FileName == "" {
		snap.FileName = "unknown"
	}

	existing, err := s.repo.GetByExternalTaskID(ctx, printer.ID, snap.ExternalTaskID)
	created := false
	if err != nil {
		job, createErr := s.createFromBambu(ctx, printer, snap)
		if createErr != nil {
			return printjobdomain.PrintJob{}, false, createErr
		}
		existing = job
		created = true
	} else if isTerminal(existing.Status) && (snap.Status == printjobdomain.StatusPrinting || snap.Status == printjobdomain.StatusPreparing || snap.Status == printjobdomain.StatusPaused || snap.Status == printjobdomain.StatusDraft) {
		job, createErr := s.createFromBambu(ctx, printer, snap)
		if createErr != nil {
			return printjobdomain.PrintJob{}, false, createErr
		}
		existing = job
		created = true
	}

	changed := false
	if snap.Progress > existing.Progress {
		existing.Progress = snap.Progress
		changed = true
	}
	if snap.FileName != "" && existing.FileName != snap.FileName {
		existing.FileName = snap.FileName
		changed = true
	}
	if snap.EstimatedWeight > 0 && existing.EstimatedWeight == 0 {
		existing.EstimatedWeight = snap.EstimatedWeight
		changed = true
	}

	prevStatus := existing.Status
	switch snap.Status {
	case printjobdomain.StatusPreparing, printjobdomain.StatusPrinting, printjobdomain.StatusPaused, printjobdomain.StatusQueued:
		if existing.Status != snap.Status && !isTerminal(existing.Status) {
			existing.Status = snap.Status
			if existing.IsDraft && (snap.Status == printjobdomain.StatusPrinting || snap.Status == printjobdomain.StatusPreparing) {
				existing.Status = printjobdomain.StatusDraft
			}
			changed = true
		}
	case printjobdomain.StatusCompleted:
		if !isTerminal(existing.Status) {
			if err := s.finalizeComplete(ctx, &existing, snap.EstimatedWeight); err != nil {
				return printjobdomain.PrintJob{}, created, err
			}
			changed = true
		}
	case printjobdomain.StatusFailed:
		if !isTerminal(existing.Status) {
			if err := s.finalizeCancelOrFail(ctx, &existing, true); err != nil {
				return printjobdomain.PrintJob{}, created, err
			}
			changed = true
		}
	case printjobdomain.StatusCancelled:
		if !isTerminal(existing.Status) {
			if err := s.finalizeCancelOrFail(ctx, &existing, false); err != nil {
				return printjobdomain.PrintJob{}, created, err
			}
			changed = true
		}
	}

	if changed || created {
		existing.UpdatedAt = time.Now()
		if err := s.repo.Update(ctx, existing); err != nil {
			return printjobdomain.PrintJob{}, created, err
		}
	}

	_ = prevStatus
	return existing, created, nil
}

func (s *Service) createFromBambu(ctx context.Context, printer printerdomain.Printer, snap BambuSnapshot) (printjobdomain.PrintJob, error) {
	productID, spoolID, estimated, draft := s.matchResources(ctx, printer, snap)
	if estimated <= 0 {
		estimated = estimateWeight(snap.RemainingMin, snap.Progress)
	}

	job := printjobdomain.PrintJob{
		ID:              uuid.New(),
		PrinterID:       printer.ID,
		ProductID:       productID,
		SpoolID:         spoolID,
		Status:          snap.Status,
		Source:          printjobdomain.SourceBambu,
		ExternalTaskID:  snap.ExternalTaskID,
		FileName:        snap.FileName,
		IsDraft:         draft,
		Progress:        snap.Progress,
		StartedAt:       time.Now(),
		EstimatedWeight: estimated,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if job.Status == "" || job.Status == printjobdomain.StatusQueued {
		job.Status = printjobdomain.StatusPrinting
	}
	if draft {
		job.Status = printjobdomain.StatusDraft
	}

	if !draft && spoolID != uuid.Nil && estimated > 0 {
		spoolEntity, err := s.spoolRepo.GetByID(ctx, spoolID)
		if err == nil {
			if err := s.reserveFilament(ctx, &spoolEntity, estimated); err == nil {
				job.ConsumedWeight = estimated
			}
		}
	}

	if err := s.repo.Create(ctx, job); err != nil {
		if job.ConsumedWeight > 0 {
			_ = s.refundFilament(ctx, job.SpoolID, job.ConsumedWeight)
		}
		return printjobdomain.PrintJob{}, err
	}
	return job, nil
}

func (s *Service) ConfirmDraft(ctx context.Context, jobID, productID, spoolID uuid.UUID) (printjobdomain.PrintJob, error) {
	job, err := s.repo.GetByID(ctx, jobID)
	if err != nil {
		return printjobdomain.PrintJob{}, err
	}
	if !job.IsDraft {
		return printjobdomain.PrintJob{}, fmt.Errorf("%w: job is not a draft", domain.ErrInvalid)
	}
	productEntity, err := s.productRepo.GetByID(ctx, productID)
	if err != nil {
		return printjobdomain.PrintJob{}, err
	}
	spoolEntity, err := s.spoolRepo.GetByID(ctx, spoolID)
	if err != nil {
		return printjobdomain.PrintJob{}, err
	}

	weight := productEntity.EstimatedWeight
	if weight <= 0 {
		weight = job.EstimatedWeight
	}
	if weight <= 0 {
		weight = 50
	}
	if weight > spoolEntity.CurrentWeight {
		return printjobdomain.PrintJob{}, fmt.Errorf("%w: insufficient filament", domain.ErrInvalid)
	}

	if err := s.reserveFilament(ctx, &spoolEntity, weight); err != nil {
		return printjobdomain.PrintJob{}, err
	}

	job.ProductID = productID
	job.SpoolID = spoolID
	job.EstimatedWeight = weight
	job.ConsumedWeight = weight
	job.IsDraft = false
	if job.Status == printjobdomain.StatusDraft {
		job.Status = printjobdomain.StatusPrinting
	}
	job.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, job); err != nil {
		_ = s.refundFilament(ctx, spoolID, weight)
		return printjobdomain.PrintJob{}, err
	}
	return job, nil
}

func (s *Service) matchResources(ctx context.Context, printer printerdomain.Printer, snap BambuSnapshot) (productID, spoolID uuid.UUID, estimated int, draft bool) {
	draft = true
	products, err := s.productRepo.List(ctx)
	if err == nil {
		fileKey := normalizeName(snap.FileName)
		for _, product := range products {
			nameKey := normalizeName(product.Name)
			if nameKey != "" && (strings.Contains(fileKey, nameKey) || strings.Contains(nameKey, fileKey)) {
				productID = product.ID
				estimated = product.EstimatedWeight
				break
			}
		}
	}

	materialHint, brandHint := parseFilamentHint(snap.MaterialHint, snap.BrandHint)
	colorAliases := colorAliases(snap.ColorHint)

	spools, err := s.spoolRepo.List(ctx)
	if err == nil {
		var bestID uuid.UUID
		bestScore := -1
		for _, spool := range spools {
			if spool.CurrentWeight <= 0 {
				continue
			}
			score := 0
			if materialHint != "" && materialsMatch(string(spool.Material), materialHint) {
				score += 2
			} else if materialHint != "" {
				continue
			}
			if len(colorAliases) > 0 {
				if colorMatches(spool.Color, colorAliases) {
					score += 2
				} else {
					continue
				}
			}
			if brandHint != "" {
				if manufacturersMatch(spool.Manufacturer, brandHint) {
					score += 2
				} else {
					continue
				}
			}
			if printer.DefaultSpoolID != uuid.Nil && spool.ID == printer.DefaultSpoolID {
				score += 1
			}
			if score > bestScore {
				bestScore = score
				bestID = spool.ID
			}
		}
		// Material+color (4) or material+brand (4) is enough; prefer all three (6).
		if bestID != uuid.Nil && bestScore >= 4 {
			spoolID = bestID
		} else if bestID != uuid.Nil && materialHint != "" && len(colorAliases) == 0 && brandHint == "" && bestScore >= 2 {
			spoolID = bestID
		}
	}

	if spoolID == uuid.Nil && printer.DefaultSpoolID != uuid.Nil {
		if spool, err := s.spoolRepo.GetByID(ctx, printer.DefaultSpoolID); err == nil && spool.CurrentWeight > 0 {
			spoolID = printer.DefaultSpoolID
		}
	}

	// Auto-consume when warehouse spool matched by cloud material (+ color/brand when present).
	if spoolID != uuid.Nil && (materialHint != "" || productID != uuid.Nil) {
		draft = false
	}
	if productID != uuid.Nil && spoolID != uuid.Nil {
		draft = false
	}
	return productID, spoolID, estimated, draft
}

func parseFilamentHint(materialRaw, brandRaw string) (material, brand string) {
	material = normalizeMaterial(materialRaw)
	brand = normalizeBrand(brandRaw)
	if brand == "" {
		brand = extractBrand(materialRaw)
	}
	if brand == "" {
		brand = extractBrand(brandRaw)
	}
	return material, brand
}

func extractBrand(raw string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	if lower == "" {
		return ""
	}
	switch {
	case strings.Contains(lower, "bambu lab"), strings.Contains(lower, "bambulab"), strings.HasPrefix(lower, "bambu"):
		return "bambu"
	case strings.Contains(lower, "generic"):
		return "generic"
	case strings.Contains(lower, "polymaker"):
		return "polymaker"
	case strings.Contains(lower, "overture"):
		return "overture"
	case strings.Contains(lower, "esun"):
		return "esun"
	case strings.Contains(lower, "sunlu"):
		return "sunlu"
	default:
		return ""
	}
}

func normalizeBrand(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = strings.ReplaceAll(raw, "-", " ")
	raw = strings.Join(strings.Fields(raw), " ")
	switch raw {
	case "":
		return ""
	case "bambu lab", "bambulab", "bambu":
		return "bambu"
	case "generic":
		return "generic"
	case "polymaker":
		return "polymaker"
	case "overture":
		return "overture"
	case "esun":
		return "esun"
	case "sunlu":
		return "sunlu"
	default:
		return raw
	}
}

func manufacturersMatch(spoolManufacturer, brandHint string) bool {
	a := normalizeBrand(spoolManufacturer)
	b := normalizeBrand(brandHint)
	if a == "" || b == "" {
		return false
	}
	return a == b
}

func normalizeMaterial(raw string) string {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	raw = strings.ReplaceAll(raw, "-", "")
	raw = strings.ReplaceAll(raw, "_", " ")
	// Cloud / Handy often send "Generic PLA", "Bambu PLA Matte", etc.
	fields := strings.Fields(raw)
	joined := strings.Join(fields, "")

	switch {
	case strings.Contains(joined, "PETG"):
		return "PETG"
	case strings.Contains(joined, "PLA"):
		return "PLA"
	case strings.Contains(joined, "ABS"):
		return "ABS"
	case strings.Contains(joined, "ASA"):
		return "ASA"
	case strings.Contains(joined, "TPU"):
		return "TPU"
	case strings.Contains(joined, "PC"):
		return "PC"
	case strings.HasPrefix(joined, "PLA"):
		return "PLA"
	default:
		return joined
	}
}

func materialsMatch(spoolMaterial, hint string) bool {
	return strings.EqualFold(normalizeMaterial(spoolMaterial), normalizeMaterial(hint))
}

func colorAliases(raw string) []string {
	return filament.ExpandColorAliases(raw)
}

func colorMatches(spoolColor string, aliases []string) bool {
	spoolColor = strings.ToLower(strings.TrimSpace(spoolColor))
	spoolExpanded := filament.ExpandColorAliases(spoolColor)
	for _, alias := range aliases {
		alias = strings.ToLower(strings.TrimSpace(alias))
		if alias == "" {
			continue
		}
		if spoolColor == alias || strings.Contains(spoolColor, alias) || strings.Contains(alias, spoolColor) {
			return true
		}
		for _, expanded := range spoolExpanded {
			if strings.EqualFold(expanded, alias) {
				return true
			}
		}
	}
	return false
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func normalizeName(raw string) string {
	base := strings.ToLower(strings.TrimSpace(raw))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.ReplaceAll(base, "-", " ")
	return strings.Join(strings.Fields(base), " ")
}

func estimateWeight(remainingMin int, progress float64) int {
	if progress >= 100 {
		progress = 99
	}
	totalMin := float64(remainingMin)
	if progress > 1 {
		totalMin = float64(remainingMin) / (1.0 - progress/100.0)
	}
	if totalMin <= 0 {
		return 80
	}
	// ~12g filament per hour as conservative fallback.
	grams := int(totalMin / 60.0 * 12.0)
	if grams < 20 {
		grams = 20
	}
	if grams > 800 {
		grams = 800
	}
	return grams
}

func (s *Service) finalizeComplete(ctx context.Context, job *printjobdomain.PrintJob, actualWeight int) error {
	if actualWeight > 0 && job.SpoolID != uuid.Nil {
		delta := actualWeight - job.ConsumedWeight
		if delta > 0 {
			spoolEntity, err := s.spoolRepo.GetByID(ctx, job.SpoolID)
			if err == nil {
				_ = s.reserveFilament(ctx, &spoolEntity, delta)
				job.ConsumedWeight = actualWeight
			}
		} else if delta < 0 {
			_ = s.refundFilament(ctx, job.SpoolID, -delta)
			job.ConsumedWeight = actualWeight
		}
		job.EstimatedWeight = actualWeight
	}
	job.Complete()
	return nil
}

func (s *Service) finalizeCancelOrFail(ctx context.Context, job *printjobdomain.PrintJob, failed bool) error {
	if job.ConsumedWeight > 0 && job.SpoolID != uuid.Nil {
		_ = s.refundFilament(ctx, job.SpoolID, job.ConsumedWeight)
		job.ConsumedWeight = 0
	}
	if failed {
		job.Fail()
	} else {
		job.Cancel()
	}
	job.IsDraft = false
	return nil
}

func (s *Service) reserveFilament(ctx context.Context, spool *spooldomain.Spool, weight int) error {
	if weight <= 0 {
		return nil
	}
	if spool.CurrentWeight < weight {
		return fmt.Errorf("%w: insufficient filament", domain.ErrInvalid)
	}
	spool.Consume(weight)
	return s.spoolRepo.Update(ctx, *spool)
}

func (s *Service) refundFilament(ctx context.Context, spoolID uuid.UUID, weight int) error {
	if weight <= 0 || spoolID == uuid.Nil {
		return nil
	}
	spoolEntity, err := s.spoolRepo.GetByID(ctx, spoolID)
	if err != nil {
		return err
	}
	spoolEntity.Refill(weight)
	return s.spoolRepo.Update(ctx, spoolEntity)
}

func isTerminal(status printjobdomain.Status) bool {
	return status == printjobdomain.StatusCompleted || status == printjobdomain.StatusFailed || status == printjobdomain.StatusCancelled
}

func (s *Service) Pause(ctx context.Context, jobID uuid.UUID) error {
	job, err := s.repo.GetByID(ctx, jobID)
	if err != nil {
		return err
	}
	job.Pause()
	return s.repo.Update(ctx, job)
}

func (s *Service) Resume(ctx context.Context, jobID uuid.UUID) error {
	job, err := s.repo.GetByID(ctx, jobID)
	if err != nil {
		return err
	}
	job.Resume()
	return s.repo.Update(ctx, job)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	job, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if !isTerminal(job.Status) && job.ConsumedWeight > 0 {
		if err := s.refundFilament(ctx, job.SpoolID, job.ConsumedWeight); err != nil {
			return err
		}
	}

	return s.repo.Delete(ctx, id)
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (printjobdomain.PrintJob, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context) ([]printjobdomain.PrintJob, error) {
	return s.repo.List(ctx)
}
