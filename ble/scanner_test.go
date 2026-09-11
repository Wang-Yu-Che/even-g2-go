package ble

import (
	"context"
	"errors"
	"testing"
	"time"

	"tinygo.org/x/bluetooth"
)

func TestArmClassifier(t *testing.T) {
	tests := []struct {
		name       string
		devices    []string
		want       []Arm
		classified []bool
	}{
		{
			name:       "G2 suffixes",
			devices:    []string{"Even G2_L_1234", "Even G2_R_5678"},
			want:       []Arm{Left, Right},
			classified: []bool{true, true},
		},
		{
			name:       "case insensitive words",
			devices:    []string{"even left", "even right"},
			want:       []Arm{Left, Right},
			classified: []bool{true, true},
		},
		{
			name:       "fallback fills empty slots",
			devices:    []string{"Even unknown 1", "Even unknown 2", "Even unknown 3"},
			want:       []Arm{Left, Right, Left},
			classified: []bool{true, true, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classifier := armClassifier{}
			for i, device := range tt.devices {
				got, ok := classifier.classify(device)
				if ok != tt.classified[i] {
					t.Fatalf("classify(%q) ok = %t, want %t", device, ok, tt.classified[i])
				}
				if got != tt.want[i] {
					t.Fatalf("classify(%q) = %s, want %s", device, got, tt.want[i])
				}
			}
		})
	}
}

func TestArmString(t *testing.T) {
	if Left.String() != "LEFT" || Right.String() != "RIGHT" {
		t.Fatalf("unexpected arm labels: %q %q", Left, Right)
	}
}

func TestScanDoesNotStartAfterContextExpiresDuringEnable(t *testing.T) {
	adapter := &delayedAdapter{enableDelay: 10 * time.Millisecond}
	scanner := &Scanner{adapter: adapter}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	err := scanner.Scan(ctx, func(ScanResult) {})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Scan() error = %v, want context deadline exceeded", err)
	}
	if adapter.scanCalled {
		t.Fatal("Scan() started adapter scan after context expired")
	}
}

type delayedAdapter struct {
	enableDelay time.Duration
	scanCalled  bool
}

func (a *delayedAdapter) Enable() error {
	time.Sleep(a.enableDelay)
	return nil
}

func (a *delayedAdapter) Scan(func(*bluetooth.Adapter, bluetooth.ScanResult)) error {
	a.scanCalled = true
	return nil
}

func (a *delayedAdapter) StopScan() error {
	return nil
}
