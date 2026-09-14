package pricing

import "math"

// Rates holds electricity/labor knobs (from config / env).
type Rates struct {
	PrinterPowerKW    float64
	LaborPerHour      float64
	ElectricityPerson float64
	ElectricityLegal  float64
}

func DefaultRates() Rates {
	return Rates{
		PrinterPowerKW:    0.35,
		LaborPerHour:      12.0,
		ElectricityPerson: 0.1176,
		ElectricityLegal:  0.18381,
	}
}

// Deprecated aliases for older call sites / tests.
const (
	DefaultPrinterPowerKW   = 0.35
	DefaultLaborPerHour     = 12.0
	LegalElectricityRate    = 0.18381
	PersonalElectricityRate = 0.1176
)

// ProductCost matches the web UI calculator using DefaultRates.
func ProductCost(weightGrams int, durationHours float64, spoolPricePerKg float64, legalEntity bool) float64 {
	return ProductCostWithRates(DefaultRates(), weightGrams, durationHours, spoolPricePerKg, legalEntity)
}

func ProductCostWithRates(rates Rates, weightGrams int, durationHours float64, spoolPricePerKg float64, legalEntity bool) float64 {
	if weightGrams < 0 {
		weightGrams = 0
	}
	if durationHours < 0 {
		durationHours = 0
	}
	if rates.PrinterPowerKW <= 0 {
		rates.PrinterPowerKW = DefaultPrinterPowerKW
	}
	if rates.LaborPerHour < 0 {
		rates.LaborPerHour = DefaultLaborPerHour
	}
	rate := rates.ElectricityPerson
	if legalEntity {
		rate = rates.ElectricityLegal
	}
	if rate <= 0 {
		if legalEntity {
			rate = LegalElectricityRate
		} else {
			rate = PersonalElectricityRate
		}
	}
	material := (float64(weightGrams) / 1000.0) * spoolPricePerKg
	electricity := durationHours * rates.PrinterPowerKW * rate
	labor := durationHours * rates.LaborPerHour
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
