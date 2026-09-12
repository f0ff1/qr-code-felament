package pricing

import "math"

const (
	DefaultPrinterPowerKW = 0.35
	DefaultLaborPerHour   = 12.0
	LegalElectricityRate  = 0.18381
	PersonalElectricityRate = 0.1176
)

// ProductCost matches the web UI calculator:
// material (₽/kg) + electricity (kW·h·rate) + labor (₽/h).
func ProductCost(weightGrams int, durationHours float64, spoolPricePerKg float64, legalEntity bool) float64 {
	if weightGrams < 0 {
		weightGrams = 0
	}
	if durationHours < 0 {
		durationHours = 0
	}
	rate := PersonalElectricityRate
	if legalEntity {
		rate = LegalElectricityRate
	}
	material := (float64(weightGrams) / 1000.0) * spoolPricePerKg
	electricity := durationHours * DefaultPrinterPowerKW * rate
	labor := durationHours * DefaultLaborPerHour
	return math.Round((material+electricity+labor)*100) / 100
}

func DurationHours(remainingMin int, progress float64) float64 {
	totalMin := EstimateTotalMinutes(remainingMin, progress)
	return float64(totalMin) / 60.0
}

func EstimateTotalMinutes(remainingMin int, progress float64) int {
	if remainingMin <= 0 {
		return 0
	}
	p := progress
	if p >= 100 {
		p = 99
	}
	if p > 1 {
		return int(float64(remainingMin) / (1.0 - p/100.0))
	}
	return remainingMin
}
