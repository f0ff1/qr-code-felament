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
}

type printJobRepository interface {
	List(ctx context.Context) ([]printjobdomain.PrintJob, error)
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
		tick: 5 * time.Second,
	}
}

func NewSchedulerWithDeps(spoolRepo spoolRepository, jobRepo printJobRepository, notifier notifier) *Scheduler {
	return &Scheduler{
		spoolRepo: spoolRepo,
		jobRepo:   jobRepo,
		notifier:  notifier,
		tick:      5 * time.Second,
	}
}

func (s *Scheduler) Run(ctx context.Context) error {
	if s.tick == 0 {
		s.tick = 5 * time.Second
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

func (s *Scheduler) Scan(ctx context.Context) {
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
