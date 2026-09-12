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
	spoolRepo spoolRepository
	jobRepo   printJobRepository
	notifier  notifier
	tick      time.Duration
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		tick: 10 * time.Second,
	}
}

func NewSchedulerWithDeps(spoolRepo spoolRepository, jobRepo printJobRepository, notifier notifier) *Scheduler {
	return &Scheduler{
		spoolRepo: spoolRepo,
		jobRepo:   jobRepo,
		notifier:  notifier,
		tick:      10 * time.Second,
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
	return status == printjobdomain.StatusQueued || status == printjobdomain.StatusPrinting || status == printjobdomain.StatusPaused
}

func (s *Scheduler) syncRuntimeState(ctx context.Context) {
	if s.jobRepo != nil {
		jobs, err := s.jobRepo.List(ctx)
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
				if s.jobRepo != nil {
					jobs, listErr := s.jobRepo.List(ctx)
					if listErr == nil {
						for _, job := range jobs {
							if job.SpoolID == spool.ID && isActiveJobStatus(job.Status) {
								spool.Status = spooldomain.StatusInUse
								spool.UpdatedAt = time.Now()
								_ = s.spoolRepo.Update(ctx, spool)
								break
							}
						}
					}
				}
				if spool.Status == spooldomain.StatusInUse {
					continue
				}
				if spool.CurrentWeight <= 0 {
					if spool.Status != spooldomain.StatusEmpty {
						spool.Status = spooldomain.StatusEmpty
						spool.UpdatedAt = time.Now()
						_ = s.spoolRepo.Update(ctx, spool)
					}
					continue
				}
				if spool.CurrentWeight < 200 {
					if spool.Status != spooldomain.StatusLow {
						spool.Status = spooldomain.StatusLow
						spool.UpdatedAt = time.Now()
						_ = s.spoolRepo.Update(ctx, spool)
					}
					continue
				}
				if spool.Status != spooldomain.StatusAvailable {
					spool.Status = spooldomain.StatusAvailable
					spool.UpdatedAt = time.Now()
					_ = s.spoolRepo.Update(ctx, spool)
				}
			}
		}
	}
}

func (s *Scheduler) Scan(ctx context.Context) {
	s.syncRuntimeState(ctx)

	if s.spoolRepo != nil {
		spools, err := s.spoolRepo.List(ctx)
		if err == nil {
			for _, spool := range spools {
				if spool.CurrentWeight <= 50 && spool.Status != spooldomain.StatusEmpty {
					if s.notifier != nil {
						s.notifier.Publish("spool_low", fmt.Sprintf("Spool %s is running low", spool.ID), map[string]any{"spool_id": spool.ID.String(), "current_weight": spool.CurrentWeight})
					}
				}
			}
		}
	}

	if s.jobRepo != nil {
		jobs, err := s.jobRepo.List(ctx)
		if err == nil {
			for _, job := range jobs {
				if job.Status == printjobdomain.StatusCompleted {
					if s.notifier != nil {
						s.notifier.Publish("print_completed", "Print job completed", map[string]any{"job_id": job.ID.String(), "printer_id": job.PrinterID.String()})
					}
				}
			}
		}
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
