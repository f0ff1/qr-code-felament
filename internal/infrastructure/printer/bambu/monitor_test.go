package bambu

import (
	"testing"

	printjobdomain "filamenttracker/internal/domain/printjob"
	printjobusecase "filamenttracker/internal/usecase/printjob"
)

func TestMapGcodeState(t *testing.T) {
	cases := []struct {
		in     string
		status printjobdomain.Status
		active bool
	}{
		{"RUNNING", printjobdomain.StatusPrinting, true},
		{"PREPARE", printjobdomain.StatusPreparing, true},
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
	status, active := mapCloudPrintStatus("ACTIVE")
	if status != printjobdomain.StatusPrinting || !active {
		t.Fatalf("ACTIVE => %s/%v", status, active)
	}
	status, active = mapCloudPrintStatus("PREPARE")
	if status != printjobdomain.StatusPreparing || !active {
		t.Fatalf("PREPARE => %s/%v", status, active)
	}
	status, active = mapCloudPrintStatus("SUCCESS")
	if status != printjobdomain.StatusCompleted || !active {
		t.Fatalf("SUCCESS => %s/%v", status, active)
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

func TestMergeCloudSnapshotsPrefersMqttProgressAndRemaining(t *testing.T) {
	rest := printjobusecase.BambuSnapshot{
		ExternalTaskID:       "cloud-1",
		FileName:             "from-rest",
		Status:               printjobdomain.StatusPrinting,
		Progress:             0,
		EstimatedDurationSec: 7200,
		RemainingMin:         120,
	}
	mqtt := printjobusecase.BambuSnapshot{
		ExternalTaskID: "99",
		FileName:       "from-mqtt",
		Status:         printjobdomain.StatusPrinting,
		Progress:       42,
		RemainingMin:   70,
	}
	snap, active := mergeCloudSnapshots(rest, true, mqtt, true)
	if !active || snap.Status != printjobdomain.StatusPrinting {
		t.Fatalf("expected active printing, got %s/%v", snap.Status, active)
	}
	if snap.Progress != 42 || snap.RemainingMin != 70 || snap.FileName != "from-mqtt" {
		t.Fatalf("expected mqtt live timing, got %+v", snap)
	}
	if snap.EstimatedDurationSec != 7200 {
		t.Fatalf("should keep cloud costTime duration, got %d", snap.EstimatedDurationSec)
	}
}

func TestMergeCloudSnapshotsIgnoresStaleFinishedMqttOnLiveRest(t *testing.T) {
	rest := printjobusecase.BambuSnapshot{
		Status:       printjobdomain.StatusPrinting,
		Progress:     0,
		RemainingMin: 80,
	}
	mqtt := printjobusecase.BambuSnapshot{
		Status:       printjobdomain.StatusCompleted,
		Progress:     100,
		RemainingMin: 0,
	}
	snap, active := mergeCloudSnapshots(rest, true, mqtt, true)
	if !active || snap.Status != printjobdomain.StatusPrinting {
		t.Fatalf("expected live printing from REST, got %s/%v", snap.Status, active)
	}
	if snap.Progress != 0 {
		t.Fatalf("stale MQTT 100%% must not overwrite live print progress, got %v", snap.Progress)
	}
}

func TestMergeCloudSnapshotsMergesLayers(t *testing.T) {
	rest := printjobusecase.BambuSnapshot{
		Status:          printjobdomain.StatusPrinting,
		EstimatedWeight: 180,
	}
	mqtt := printjobusecase.BambuSnapshot{
		Status:       printjobdomain.StatusPrinting,
		Progress:     20,
		LayerCurrent: 18,
		LayerTotal:   90,
	}
	snap, active := mergeCloudSnapshots(rest, true, mqtt, true)
	if !active || snap.LayerCurrent != 18 || snap.LayerTotal != 90 || snap.EstimatedWeight != 180 {
		t.Fatalf("unexpected merge: %+v active=%v", snap, active)
	}
}
