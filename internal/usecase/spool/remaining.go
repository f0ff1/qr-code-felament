package spool

import (
	printjobdomain "filamenttracker/internal/domain/printjob"
	spooldomain "filamenttracker/internal/domain/spool"
)

// LiveRemaining computes display remaining weight while active jobs hold a reserve.
// DB CurrentWeight is "future remaining" after full job reserve at print start.
// Live remaining = future + not-yet-extruded portion of reserved filament.
func LiveRemaining(spoolEntity spooldomain.Spool, jobs []printjobdomain.PrintJob) int {
	futureRemaining := spoolEntity.CurrentWeight
	notYetUsed := 0
	for _, job := range jobs {
		if job.SpoolID != spoolEntity.ID || !printjobdomain.IsActive(job.Status) {
			continue
		}
		weight := job.EstimatedWeight
		if weight <= 0 {
			weight = job.ConsumedWeight
		}
		if weight <= 0 {
			continue
		}
		progress := job.Progress
		if progress < 0 {
			progress = 0
		}
		if progress > 100 {
			progress = 100
		}
		// Bambu can briefly report 100% while remaining time is still > 0 (stale MQTT).
		if job.RemainingMinutes > 0 && progress >= 100 {
			progress = 99
		}
		notYetUsed += int(float64(weight) * (1.0 - progress/100.0))
	}
	if futureRemaining+notYetUsed < 0 {
		return 0
	}
	return futureRemaining + notYetUsed
}
