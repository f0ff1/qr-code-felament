package bambu

import (
	"testing"

	printjobdomain "filamenttracker/internal/domain/printjob"
)

func TestMapGcodeState(t *testing.T) {
	cases := []struct {
		in     string
		status printjobdomain.Status
		active bool
	}{
		{"RUNNING", printjobdomain.StatusPrinting, true},
		{"PAUSE", printjobdomain.StatusPaused, true},
		{"FINISH", printjobdomain.StatusCompleted, true},
		{"FAILED", printjobdomain.StatusFailed, true},
		{"IDLE", printjobdomain.StatusQueued, false},
	}
	for _, tc := range cases {
		status, active := mapGcodeState(tc.in)
		if status != tc.status || active != tc.active {
			t.Fatalf("%s => %s/%v, want %s/%v", tc.in, status, active, tc.status, tc.active)
		}
	}
}

func TestMapCloudPrintStatus(t *testing.T) {
	status, active := mapCloudPrintStatus("RUNNING")
	if status != printjobdomain.StatusPrinting || !active {
		t.Fatalf("RUNNING => %s/%v", status, active)
	}
	status, active = mapCloudPrintStatus("IDLE")
	if active {
		t.Fatalf("IDLE should be inactive, got %s", status)
	}
}

func TestNormalizeCloudRegion(t *testing.T) {
	if got := NormalizeCloudRegion("China"); got != "cn" {
		t.Fatalf("China => %s", got)
	}
	if got := NormalizeCloudRegion("eu"); got != "us" {
		t.Fatalf("eu => %s", got)
	}
}
