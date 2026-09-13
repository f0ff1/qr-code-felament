package printjob

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"filamenttracker/internal/domain"
	"filamenttracker/internal/domain/filament"
	printerdomain "filamenttracker/internal/domain/printer"
	printjobdomain "filamenttracker/internal/domain/printjob"
	productdomain "filamenttracker/internal/domain/product"
	spooldomain "filamenttracker/internal/domain/spool"
	"filamenttracker/internal/usecase/pricing"

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
	Create(ctx context.Context, p productdomain.Product) error
	Update(ctx context.Context, p productdomain.Product) error
}

type PrinterRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (printerdomain.Printer, error)
}

type BambuSnapshot struct {
	ExternalTaskID       string
	FileName             string
	Progress             float64
	Status               printjobdomain.Status
	RemainingMin         int
	EstimatedDurationSec int
	EstimatedWeight      int
	LayerCurrent         int
	LayerTotal           int
	MaterialHint         string
	ColorHint            string
	BrandHint            string
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

	need := productEntity.EstimatedWeight
	reserved, err := s.reserveAvailable(ctx, &spoolEntity, need)
	if err != nil {
		return printjobdomain.PrintJob{}, err
	}

	job := printjobdomain.NewPrintJob(printerEntity.ID, productEntity.ID, spoolEntity.ID, need)
	job.Start()
	job.ConsumedWeight = reserved
	job.FileName = productEntity.Name
	if err := s.repo.Create(ctx, job); err != nil {
		_ = s.refundFilament(ctx, spoolEntity.ID, reserved)
		return printjobdomain.PrintJob{}, err
	}
	return job, nil
}

func (s *Service) SyncFromBambu(ctx context.Context, printer printerdomain.Printer, snap BambuSnapshot) (printjobdomain.PrintJob, bool, error) {
	snap = sanitizeSnapshot(snap)
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
	} else if isTerminal(existing.Status) && (snap.Status == printjobdomain.StatusPrinting || snap.Status == printjobdomain.StatusPreparing || snap.Status == printjobdomain.StatusCalibrating || snap.Status == printjobdomain.StatusPaused || snap.Status == printjobdomain.StatusDraft) {
		job, createErr := s.createFromBambu(ctx, printer, snap)
		if createErr != nil {
			return printjobdomain.PrintJob{}, false, createErr
		}
		existing = job
		created = true
	}

	changed := false
	if existing.Source == printjobdomain.SourceBambu {
		liveProgress := sanitizeJobProgress(snap.Status, snap.Progress, snap.RemainingMin)
		if math.Abs(liveProgress-existing.Progress) >= 0.1 {
			existing.Progress = liveProgress
			changed = true
		}
	} else if snap.Progress > existing.Progress {
		existing.Progress = snap.Progress
		changed = true
	}
	if snap.FileName != "" && existing.FileName != snap.FileName {
		existing.FileName = snap.FileName
		changed = true
	}
	if linked, linkErr := s.ensureDraftLinked(ctx, &existing, printer, snap); linkErr != nil {
		return printjobdomain.PrintJob{}, created, linkErr
	} else if linked {
		changed = true
	}
	if weightChanged, weightErr := s.applyJobWeight(ctx, &existing, snap.EstimatedWeight); weightErr != nil {
		return printjobdomain.PrintJob{}, created, weightErr
	} else if weightChanged {
		changed = true
	}
	if layersChanged := applyLayers(&existing, snap); layersChanged {
		changed = true
	}
	if timingChanged := applyTiming(&existing, snap); timingChanged {
		changed = true
	}
	if productSynced := s.syncLinkedProduct(ctx, &existing, snap); productSynced {
		changed = true
	}

	switch snap.Status {
	case printjobdomain.StatusPreparing, printjobdomain.StatusCalibrating, printjobdomain.StatusPrinting, printjobdomain.StatusPaused, printjobdomain.StatusQueued:
		if existing.Status != snap.Status && !isTerminal(existing.Status) {
			existing.Status = snap.Status
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

	return existing, created, nil
}

func applyTiming(job *printjobdomain.PrintJob, snap BambuSnapshot) bool {
	changed := false
	remaining := snap.RemainingMin
	if remaining < 0 {
		remaining = 0
	}
	if isTerminal(snap.Status) {
		remaining = 0
	}
	if remaining != job.RemainingMinutes {
		job.RemainingMinutes = remaining
		changed = true
	}

	durationSec := snap.EstimatedDurationSec
	if durationSec <= 0 && remaining > 0 {
		durationSec = pricing.EstimateTotalMinutes(remaining, snap.Progress) * 60
	}
	if durationSec > 0 && (job.EstimatedDurationSec == 0 || absInt(durationSec-job.EstimatedDurationSec) >= 30) {
		// Prefer longer/more complete estimates once known; allow refine within ±30s noise filter.
		if job.EstimatedDurationSec == 0 || durationSec > job.EstimatedDurationSec || absInt(durationSec-job.EstimatedDurationSec) >= 60 {
			job.EstimatedDurationSec = durationSec
			changed = true
		}
	}
	return changed
}

func applyLayers(job *printjobdomain.PrintJob, snap BambuSnapshot) bool {
	changed := false
	layerCurrent := snap.LayerCurrent
	layerTotal := snap.LayerTotal
	if layerTotal <= 0 && job.LayerTotal > 0 {
		layerTotal = job.LayerTotal
	}
	// Never invent current layer from progress% — Bambu layer_num is non-linear vs %.
	if layerCurrent <= 0 && job.LayerCurrent > 0 && layerTotal == job.LayerTotal {
		layerCurrent = job.LayerCurrent
	}
	if isTerminal(snap.Status) && layerTotal > 0 {
		layerCurrent = layerTotal
	}
	if layerCurrent != job.LayerCurrent {
		job.LayerCurrent = layerCurrent
		changed = true
	}
	if layerTotal != job.LayerTotal {
		job.LayerTotal = layerTotal
		changed = true
	}
	return changed
}

func (s *Service) applyJobWeight(ctx context.Context, job *printjobdomain.PrintJob, newWeight int) (bool, error) {
	if newWeight <= 0 {
		return false, nil
	}
	changed := false
	if job.EstimatedWeight != newWeight {
		job.EstimatedWeight = newWeight
		changed = true
	}

	if job.IsDraft || job.SpoolID == uuid.Nil {
		return changed, nil
	}

	delta := newWeight - job.ConsumedWeight
	if delta == 0 {
		return changed, nil
	}
	if delta > 0 {
		spoolEntity, err := s.spoolRepo.GetByID(ctx, job.SpoolID)
		if err != nil {
			return false, err
		}
		got, err := s.reserveAvailable(ctx, &spoolEntity, delta)
		if err != nil {
			return false, err
		}
		if got > 0 {
			job.ConsumedWeight += got
			changed = true
		}
	} else {
		if err := s.refundFilament(ctx, job.SpoolID, -delta); err != nil {
			return false, err
		}
		job.ConsumedWeight = newWeight
		changed = true
	}
	return changed, nil
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func sanitizeJobProgress(status printjobdomain.Status, progress float64, remainingMin int) float64 {
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	live := status == printjobdomain.StatusPreparing || status == printjobdomain.StatusCalibrating || status == printjobdomain.StatusPrinting || status == printjobdomain.StatusPaused || status == printjobdomain.StatusQueued || status == printjobdomain.StatusDraft
	if live && remainingMin > 0 && progress >= 100 {
		return 99
	}
	return progress
}

func (s *Service) createFromBambu(ctx context.Context, printer printerdomain.Printer, snap BambuSnapshot) (printjobdomain.PrintJob, error) {
	productID, spoolID, matchedWeight, draft := s.matchResources(ctx, printer, snap)
	// Prefer Bambu-reported filament usage; fall back to matched product, then time heuristic.
	estimated := snap.EstimatedWeight
	if estimated <= 0 {
		estimated = matchedWeight
	}
	if estimated <= 0 {
		estimated = estimateWeight(snap.RemainingMin, snap.Progress)
	}
	if productID == uuid.Nil {
		createdProduct, err := s.autoCreateProduct(ctx, snap, spoolID, estimated)
		if err != nil {
			return printjobdomain.PrintJob{}, err
		}
		productID = createdProduct.ID
		if createdProduct.EstimatedWeight > 0 {
			estimated = createdProduct.EstimatedWeight
		}
		// Product exists; consume filament once spool is matched.
		if spoolID != uuid.Nil {
			draft = false
		}
	}

	durationSec := snap.EstimatedDurationSec
	if durationSec <= 0 && snap.RemainingMin > 0 {
		durationSec = pricing.EstimateTotalMinutes(snap.RemainingMin, snap.Progress) * 60
	}

	layerCurrent, layerTotal := snap.LayerCurrent, snap.LayerTotal

	job := printjobdomain.PrintJob{
		ID:                   uuid.New(),
		PrinterID:            printer.ID,
		ProductID:            productID,
		SpoolID:              spoolID,
		Status:               snap.Status,
		Source:               printjobdomain.SourceBambu,
		ExternalTaskID:       snap.ExternalTaskID,
		FileName:             snap.FileName,
		IsDraft:              draft,
		Progress:             sanitizeJobProgress(snap.Status, snap.Progress, snap.RemainingMin),
		RemainingMinutes:     snap.RemainingMin,
		EstimatedDurationSec: durationSec,
		LayerCurrent:         layerCurrent,
		LayerTotal:           layerTotal,
		StartedAt:            time.Now(),
		EstimatedWeight:      estimated,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}
	if job.Status == "" || job.Status == printjobdomain.StatusQueued {
		job.Status = printjobdomain.StatusPrinting
	}
	// IsDraft means "катушка не подтверждена" — printer status stays from Cloud.

	if !draft && spoolID != uuid.Nil && estimated > 0 {
		spoolEntity, err := s.spoolRepo.GetByID(ctx, spoolID)
		if err != nil {
			return printjobdomain.PrintJob{}, err
		}
		reserved, err := s.reserveAvailable(ctx, &spoolEntity, estimated)
		if err != nil {
			return printjobdomain.PrintJob{}, err
		}
		job.ConsumedWeight = reserved
	}

	if err := s.repo.Create(ctx, job); err != nil {
		if job.ConsumedWeight > 0 {
			_ = s.refundFilament(ctx, job.SpoolID, job.ConsumedWeight)
		}
		return printjobdomain.PrintJob{}, err
	}
	return job, nil
}

func productDisplayName(fileName string) string {
	name := normalizeName(fileName)
	if name == "" || name == "unknown" {
		return "Печать Bambu"
	}
	parts := strings.Fields(name)
	for i, part := range parts {
		runes := []rune(part)
		if len(runes) == 0 {
			continue
		}
		parts[i] = strings.ToUpper(string(runes[0])) + string(runes[1:])
	}
	return strings.Join(parts, " ")
}

func sanitizeSnapshot(snap BambuSnapshot) BambuSnapshot {
	snap.ExternalTaskID = sanitizeUTF8(snap.ExternalTaskID)
	snap.FileName = sanitizeUTF8(snap.FileName)
	snap.MaterialHint = sanitizeUTF8(snap.MaterialHint)
	snap.ColorHint = sanitizeUTF8(snap.ColorHint)
	snap.BrandHint = sanitizeUTF8(snap.BrandHint)
	return snap
}

// sanitizeUTF8 strips invalid bytes so Postgres UTF8 columns never reject Bambu strings.
func sanitizeUTF8(raw string) string {
	if raw == "" || utf8.ValidString(raw) {
		return raw
	}
	return strings.ToValidUTF8(raw, "")
}

func normalizeName(raw string) string {
	base := strings.ToLower(strings.TrimSpace(sanitizeUTF8(raw)))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.ReplaceAll(base, "_", " ")
	base = strings.ReplaceAll(base, "-", " ")
	return strings.Join(strings.Fields(base), " ")
}

func (s *Service) autoCreateProduct(ctx context.Context, snap BambuSnapshot, spoolID uuid.UUID, estimated int) (productdomain.Product, error) {
	material, _ := parseFilamentHint(snap.MaterialHint, snap.BrandHint)
	if material == "" {
		material = "PLA"
	}
	durationSec := snap.EstimatedDurationSec
	if durationSec <= 0 && snap.RemainingMin > 0 {
		durationSec = pricing.EstimateTotalMinutes(snap.RemainingMin, snap.Progress) * 60
	}
	if durationSec <= 0 {
		durationSec = 3600
	}
	if estimated <= 0 {
		estimated = estimateWeight(snap.RemainingMin, snap.Progress)
	}

	pricePerKg := 0.0
	if spoolID != uuid.Nil {
		if spoolEntity, err := s.spoolRepo.GetByID(ctx, spoolID); err == nil {
			pricePerKg = spoolEntity.Price
		}
	}
	hours := float64(durationSec) / 3600.0
	pricePerson := pricing.ProductCost(estimated, hours, pricePerKg, false)
	priceLegal := pricing.ProductCost(estimated, hours, pricePerKg, true)

	desc := "Автоматически из Bambu Cloud"
	if snap.FileName != "" {
		desc = "Автоматически из Bambu: " + snap.FileName
	}
	product := productdomain.NewAutoProduct(
		productDisplayName(snap.FileName),
		desc,
		material,
		estimated,
		time.Duration(durationSec)*time.Second,
		pricePerson,
		priceLegal,
	)
	if err := s.productRepo.Create(ctx, product); err != nil {
		return productdomain.Product{}, err
	}
	return product, nil
}

func (s *Service) syncLinkedProduct(ctx context.Context, job *printjobdomain.PrintJob, snap BambuSnapshot) bool {
	if job.ProductID == uuid.Nil {
		return false
	}
	product, err := s.productRepo.GetByID(ctx, job.ProductID)
	if err != nil {
		return false
	}
	changed := false
	autoManaged := strings.HasPrefix(product.Description, "Автоматически")

	if job.EstimatedWeight > 0 && product.EstimatedWeight != job.EstimatedWeight && (autoManaged || product.EstimatedWeight == 0) {
		product.EstimatedWeight = job.EstimatedWeight
		changed = true
	}
	durationSec := job.EstimatedDurationSec
	if durationSec <= 0 && job.RemainingMinutes > 0 {
		durationSec = pricing.EstimateTotalMinutes(job.RemainingMinutes, job.Progress) * 60
	}
	if durationSec > 0 {
		currentSec := int(product.EstimatedPrintTime.Seconds())
		if currentSec == 0 || (autoManaged && absInt(durationSec-currentSec) >= 60) {
			product.EstimatedPrintTime = time.Duration(durationSec) * time.Second
			changed = true
		}
	}
	if snap.FileName != "" && autoManaged {
		name := productDisplayName(snap.FileName)
		if name != "" && product.Name != name {
			product.Name = name
			changed = true
		}
	}

	if changed {
		pricePerKg := 0.0
		if job.SpoolID != uuid.Nil {
			if spoolEntity, err := s.spoolRepo.GetByID(ctx, job.SpoolID); err == nil {
				pricePerKg = spoolEntity.Price
			}
		}
		hours := product.EstimatedPrintTime.Hours()
		product.Price = pricing.ProductCost(product.EstimatedWeight, hours, pricePerKg, false)
		product.PriceLegal = pricing.ProductCost(product.EstimatedWeight, hours, pricePerKg, true)
		if product.BillingMode == "" || strings.HasPrefix(product.Description, "Автоматически") {
			product.BillingMode = productdomain.BillingBoth
		}
		product.UpdatedAt = time.Now()
		if err := s.productRepo.Update(ctx, product); err != nil {
			return false
		}
	}
	return false
}

// ensureDraftLinked rematches Cloud drafts to a warehouse spool and reserves filament.
func (s *Service) ensureDraftLinked(ctx context.Context, job *printjobdomain.PrintJob, printer printerdomain.Printer, snap BambuSnapshot) (bool, error) {
	if !job.IsDraft {
		return false, nil
	}

	productID, spoolID, matchedWeight, draft := s.matchResources(ctx, printer, snap)
	if job.ProductID != uuid.Nil {
		productID = job.ProductID
	}
	if productID == uuid.Nil {
		createdProduct, err := s.autoCreateProduct(ctx, snap, spoolID, job.EstimatedWeight)
		if err != nil {
			return false, err
		}
		productID = createdProduct.ID
		if createdProduct.EstimatedWeight > 0 && job.EstimatedWeight <= 0 {
			job.EstimatedWeight = createdProduct.EstimatedWeight
		}
		if spoolID != uuid.Nil {
			draft = false
		}
	}
	if spoolID == uuid.Nil || draft {
		return false, nil
	}

	weight := job.EstimatedWeight
	if weight <= 0 {
		weight = snap.EstimatedWeight
	}
	if weight <= 0 {
		weight = matchedWeight
	}
	if weight <= 0 {
		weight = estimateWeight(snap.RemainingMin, snap.Progress)
	}
	if weight <= 0 {
		return false, nil
	}

	changed := false
	if job.ProductID != productID {
		job.ProductID = productID
		changed = true
	}
	if job.SpoolID != spoolID {
		job.SpoolID = spoolID
		changed = true
	}
	if job.EstimatedWeight != weight {
		job.EstimatedWeight = weight
		changed = true
	}

	job.IsDraft = false
	changed = true

	if job.ConsumedWeight < weight {
		spoolEntity, err := s.spoolRepo.GetByID(ctx, spoolID)
		if err != nil {
			return false, err
		}
		need := weight - job.ConsumedWeight
		got, err := s.reserveAvailable(ctx, &spoolEntity, need)
		if err != nil {
			return false, err
		}
		job.ConsumedWeight += got
		changed = true
	}
	return changed, nil
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

	reserved, err := s.reserveAvailable(ctx, &spoolEntity, weight)
	if err != nil {
		return printjobdomain.PrintJob{}, err
	}

	job.ProductID = productID
	job.SpoolID = spoolID
	job.EstimatedWeight = weight
	job.ConsumedWeight = reserved
	job.IsDraft = false
	if job.Status == printjobdomain.StatusDraft {
		job.Status = printjobdomain.StatusPrinting
	}
	job.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, job); err != nil {
		_ = s.refundFilament(ctx, spoolID, reserved)
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

	// Auto-consume when warehouse spool matched, or printer has a default spool.
	if spoolID != uuid.Nil && (materialHint != "" || productID != uuid.Nil || printer.DefaultSpoolID == spoolID) {
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
		v = strings.TrimSpace(sanitizeUTF8(v))
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
		unused := unusedReservedGrams(*job)
		if unused > 0 {
			_ = s.refundFilament(ctx, job.SpoolID, unused)
		}
		used := job.ConsumedWeight - unused
		if used < 0 {
			used = 0
		}
		job.ConsumedWeight = used
	}
	if failed {
		job.Fail()
	} else {
		job.Cancel()
	}
	job.IsDraft = false
	return nil
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
		unused := unusedReservedGrams(job)
		if unused > 0 {
			if err := s.refundFilament(ctx, job.SpoolID, unused); err != nil {
				return err
			}
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
