package pricing

import "testing"

func TestProductCostMatchesUIFormula(t *testing.T) {
	// 220g @ 35 ₽/kg, 2.5h, legal entity
	got := ProductCost(220, 2.5, 35, true)
	material := (220.0 / 1000.0) * 35
	electricity := 2.5 * 0.35 * 0.18381
	labor := 2.5 * 12
	want := material + electricity + labor
	want = float64(int(want*100+0.5)) / 100
	if got != want {
		t.Fatalf("ProductCost = %v, want %v", got, want)
	}
}

func TestEstimateTotalMinutes(t *testing.T) {
	if got := EstimateTotalMinutes(30, 50); got != 60 {
		t.Fatalf("50%% + 30m remaining => %d, want 60", got)
	}
}
