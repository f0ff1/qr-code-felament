package spool

import "testing"

func TestEffectiveStatus(t *testing.T) {
	tests := []struct {
		name   string
		weight int
		inUse  bool
		want   Status
	}{
		{name: "active print takes priority", weight: 500, inUse: true, want: StatusInUse},
		{name: "sufficient filament", weight: 500, inUse: false, want: StatusAvailable},
		{name: "almost empty filament", weight: 180, inUse: false, want: StatusLow},
		{name: "empty filament", weight: 0, inUse: false, want: StatusEmpty},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spool := Spool{CurrentWeight: tt.weight}
			if got := spool.EffectiveStatus(tt.inUse); got != tt.want {
				t.Fatalf("EffectiveStatus(%d, %v) = %s, want %s", tt.weight, tt.inUse, got, tt.want)
			}
		})
	}
}
