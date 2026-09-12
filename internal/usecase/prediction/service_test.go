package prediction

import (
	"testing"
	"time"
)

func TestEstimatorCanFinishPrint(t *testing.T) {
	e := NewEstimator()
	if !e.CanFinishPrint(500, 420) {
		t.Fatal("CanFinishPrint() expected true when available >= required")
	}
	if e.CanFinishPrint(350, 420) {
		t.Fatal("CanFinishPrint() expected false when available < required")
	}
}

func TestEstimatorEstimateRemaining(t *testing.T) {
	e := NewEstimator()
	remaining := e.EstimateRemaining(550, 300)
	if remaining <= 0 {
		t.Fatal("EstimateRemaining() expected positive duration")
	}
	if remaining > 2*time.Hour {
		t.Fatalf("EstimateRemaining() = %v, want under 2h for 550g at 300g/h", remaining)
	}
}

func TestEstimatorExplain(t *testing.T) {
	e := NewEstimator()
	msg := e.Explain(350, 420, 300)
	if msg == "" {
		t.Fatal("Explain() returned empty message")
	}
}
