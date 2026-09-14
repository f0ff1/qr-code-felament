package worker

import (
	"context"
	"fmt"
	"time"

	printjobdomain "filamenttracker/internal/domain/printjob"
	spooldomain "filamenttracker/internal/domain/spool"
	notificationusecase "filamenttracker/internal/usecase/notification"

	"github.com/google/uuid"
)

type spoolRepository interface {
	List(ctx context.Context) ([]spooldomain.Spool, error)
	Update(ctx context.Context, s spooldomain.Spool) error
}

type printJobRepository interface {
	List(ctx context.Context) ([]printjobdomain.PrintJob, error)
	Update(ctx context.Context, j printjobdomain.PrintJob) error
}

type notifier interface {
	Publish(eventType, message string, payload map[string]any) notificationusecase.Event
}

type Scheduler struct {
	spoolRepo    spoolRepository
	jobRepo      printJobRepository
	notifier     notifier
	tick         time.Duration
	lowAlerted   map[string]bool
	emptyAlerted map[string]bool
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		tick:         10 * time.Second,
		lowAlerted:   make(map[string]bool),
		emptyAlerted: make(map[string]bool),
	}
}

func NewSchedulerWithDeps(spoolRepo spoolRepository, jobRepo printJobRepository, notifier notifier, tick time.Duration) *Scheduler {
	if tick <= 0 {
		tick = 10 * time.Second
	}
	return &Scheduler{
		spoolRepo:    spoolRepo,
		jobRepo:      jobRepo,
		notifier:     notifier,
		tick:         tick,
		lowAlerted:   make(map[string]bool),
		emptyAlerted: make(map[string]bool),
	}
}

func (s *Scheduler) Run(ctx context.Context) error {
	if s.tick == 0 {
		s.tick = 10 * time.Second
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(s.tick):
				s.Scan(ctx)
			}
		}
	}()
	return nil
}

func isActiveJobStatus(status printjobdomain.Status) bool {
	return printjobdomain.IsActive(status)
}

func (s *Scheduler) syncRuntimeState(ctx context.Context) {
	var jobs []printjobdomain.PrintJob
	if s.jobRepo != nil {
		var err error
		jobs, err = s.jobRepo.List(ctx)
		if err == nil {
			for i := range jobs {
				job := jobs[i]
				if !isActiveJobStatus(job.Status) {
					continue
				}
				job.UpdatedAt = time.Now()
				if err := s.jobRepo.Update(ctx, job); err != nil {
					continue
				}
			}
		}
	}

	if s.spoolRepo != nil {
		spools, err := s.spoolRepo.List(ctx)
		if err == nil {
			for i := range spools {
				spool := spools[i]
				hasActiveJob := false
				for _, job := range jobs {
					if job.SpoolID == spool.ID && isActiveJobStatus(job.Status) {
						hasActiveJob = true
						break
					}
				}

				if hasActiveJob {
					if spool.Status != spooldomain.StatusInUse {
						spool.Status = spooldomain.StatusInUse
						spool.UpdatedAt = time.Now()
						_ = s.spoolRepo.Update(ctx, spool)
					}
					s.checkSpoolWeightAlerts(spool)
					continue
				}

				nextStatus := spooldomain.StatusAvailable
				switch {
				case spool.CurrentWeight <= 0:
					nextStatus = spooldomain.StatusEmpty
				case spool.CurrentWeight < spooldomain.LowWeightGrams:
					nextStatus = spooldomain.StatusLow
				default:
					nextStatus = spooldomain.StatusAvailable
				}

				if spool.Status != nextStatus {
					prev := spool.Status
					spool.Status = nextStatus
					spool.UpdatedAt = time.Now()
					_ = s.spoolRepo.Update(ctx, spool)
					s.notifySpoolStatusChange(spool, prev, nextStatus)
				} else {
					s.checkSpoolWeightAlerts(spool)
				}
			}
		}
	}
}

func (s *Scheduler) Scan(ctx context.Context) {
	s.syncRuntimeState(ctx)
}

func (s *Scheduler) notifySpoolStatusChange(spool spooldomain.Spool, prev, next spooldomain.Status) {
	if s.notifier == nil {
		return
	}
	label := fmt.Sprintf("%s / %s", spool.Material, spool.Color)
	switch next {
	case spooldomain.StatusLow:
		s.publishSpoolLow(spool, label)
	case spooldomain.StatusEmpty:
		s.publishSpoolEmpty(spool, label)
	case spooldomain.StatusAvailable, spooldomain.StatusInUse:
		if spool.CurrentWeight >= spooldomain.LowWeightGrams {
			delete(s.lowAlerted, spool.ID.String())
		}
		if spool.CurrentWeight > 0 {
			delete(s.emptyAlerted, spool.ID.String())
		}
		_ = prev
	}
}

func (s *Scheduler) publishSpoolLow(spool spooldomain.Spool, label string) {
	id := spool.ID.String()
	if s.lowAlerted[id] {
		return
	}
	s.lowAlerted[id] = true
	s.notifier.Publish("spool_low", fmt.Sprintf("Катушка %s заканчивается (%d г)", label, spool.CurrentWeight), map[string]any{
		"spool_id":       id,
		"site_id":        spool.SiteID.String(),
		"current_weight": spool.CurrentWeight,
		"material":       string(spool.Material),
		"color":          spool.Color,
	})
}

func (s *Scheduler) publishSpoolEmpty(spool spooldomain.Spool, label string) {
	id := spool.ID.String()
	if s.emptyAlerted[id] {
		return
	}
	s.emptyAlerted[id] = true
	s.notifier.Publish("spool_empty", fmt.Sprintf("Катушка %s закончилась", label), map[string]any{
		"spool_id": id,
		"site_id":  spool.SiteID.String(),
		"material": string(spool.Material),
		"color":    spool.Color,
	})
}

func (s *Scheduler) checkSpoolWeightAlerts(spool spooldomain.Spool) {
	if s.notifier == nil {
		return
	}
	label := fmt.Sprintf("%s / %s", spool.Material, spool.Color)
	id := spool.ID.String()
	switch {
	case spool.CurrentWeight <= 0:
		s.publishSpoolEmpty(spool, label)
	case spool.CurrentWeight < spooldomain.LowWeightGrams:
		s.publishSpoolLow(spool, label)
	default:
		delete(s.lowAlerted, id)
		delete(s.emptyAlerted, id)
	}
}

func (s *Scheduler) PublishEvent(eventType, message string, payload map[string]any) {
	if s.notifier != nil {
		s.notifier.Publish(eventType, message, payload)
	}
}

func (s *Scheduler) BuildEventID() string {
	return uuid.NewString()
}
