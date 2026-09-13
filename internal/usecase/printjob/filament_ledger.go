package printjob

import (
	"context"
	"fmt"
	"math"

	"filamenttracker/internal/domain"
	printjobdomain "filamenttracker/internal/domain/printjob"
	spooldomain "filamenttracker/internal/domain/spool"

	"github.com/google/uuid"
)

// unusedReservedGrams is the reserved filament not yet extruded (by job progress %).
func unusedReservedGrams(job printjobdomain.PrintJob) int {
	reserved := job.ConsumedWeight
	if reserved <= 0 {
		return 0
	}
	progress := job.Progress
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	planned := job.EstimatedWeight
	if planned <= 0 {
		planned = reserved
	}
	used := int(math.Round(float64(planned) * progress / 100.0))
	if used < 0 {
		used = 0
	}
	if used > reserved {
		used = reserved
	}
	return reserved - used
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

// reserveAvailable reserves up to want grams; returns how much was actually reserved.
func (s *Service) reserveAvailable(ctx context.Context, spool *spooldomain.Spool, want int) (int, error) {
	if want <= 0 {
		return 0, nil
	}
	got := want
	if spool.CurrentWeight < got {
		got = spool.CurrentWeight
	}
	if got <= 0 {
		return 0, nil
	}
	if err := s.reserveFilament(ctx, spool, got); err != nil {
		return 0, err
	}
	return got, nil
}

func NeedsFilamentTopUp(job printjobdomain.PrintJob) bool {
	if job.IsDraft || isTerminal(job.Status) {
		return false
	}
	if !printjobdomain.IsActive(job.Status) {
		return false
	}
	return job.EstimatedWeight > 0 && job.ConsumedWeight < job.EstimatedWeight
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
