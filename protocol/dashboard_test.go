package protocol

import (
	"slices"
	"testing"
)

func TestBuildDashboardConfig(t *testing.T) {
	payload, err := BuildDashboardConfig(42, []DashboardWidget{DashboardSchedule, DashboardNews}, true, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x08, 0x02, 0x10, 0x2A, 0x22, 0x15, 0x12, 0x13, 0x08, 0x04, 0x10, 0x03, 0x1A, 0x03, 0x01, 0x02, 0x03, 0x20, 0x02, 0x2A, 0x02, 0x03, 0x01, 0x30, 0x01, 0x38, 0x01}
	if !slices.Equal(payload, want) {
		t.Fatalf("config = % X, want % X", payload, want)
	}
}

func TestBuildDashboardScheduleValidation(t *testing.T) {
	_, err := BuildDashboardSchedule(1, DashboardScheduleItem{}, 1, 1)
	if err == nil {
		t.Fatal("invalid schedule index accepted")
	}
}
