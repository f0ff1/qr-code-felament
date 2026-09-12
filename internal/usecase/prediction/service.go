package prediction

import (
	"fmt"
	"time"
)

type FilamentEstimator struct{}

func NewEstimator() *FilamentEstimator {
	return &FilamentEstimator{}
}

func (e *FilamentEstimator) CanFinishPrint(available, required int) bool {
	return available >= required
}

func (e *FilamentEstimator) EstimateRemaining(remainingWeight int, consumptionRate float64) time.Duration {
	if consumptionRate <= 0 || remainingWeight <= 0 {
		return 0
	}

	seconds := float64(remainingWeight) / consumptionRate * 3600
	return time.Duration(seconds * float64(time.Second))
}

func (e *FilamentEstimator) EstimateRequiredWeight(available, required int) int {
	return required - available
}

func (e *FilamentEstimator) Explain(available, required int, consumptionRate float64) string {
	if available >= required {
		return fmt.Sprintf("Enough filament: %dg available, %dg required", available, required)
	}
	missing := required - available
	return fmt.Sprintf("Insufficient filament: missing %dg, consumption rate %.2f g/h", missing, consumptionRate)
}
