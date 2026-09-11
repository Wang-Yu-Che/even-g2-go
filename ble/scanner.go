package ble

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"tinygo.org/x/bluetooth"
)

// Arm identifies one of the two G2 peripherals.
type Arm int

const (
	Left Arm = iota
	Right
)

func (a Arm) String() string {
	if a == Right {
		return "RIGHT"
	}
	return "LEFT"
}

// ScanResult describes an Even G2 advertisement.
type ScanResult struct {
	Arm     Arm
	Address string
	Name    string
	RSSI    int16
}

type scanAdapter interface {
	Enable() error
	Scan(func(*bluetooth.Adapter, bluetooth.ScanResult)) error
	StopScan() error
}

// Scanner discovers and classifies the two G2 arms.
type Scanner struct {
	adapter scanAdapter
}

// NewScanner returns a scanner backed by the host Bluetooth adapter.
func NewScanner() *Scanner {
	return &Scanner{adapter: bluetooth.DefaultAdapter}
}

// Scan reports Even G2 advertisements until ctx is cancelled.
func (s *Scanner) Scan(ctx context.Context, report func(ScanResult)) error {
	if report == nil {
		return errors.New("scan report callback is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.adapter.Enable(); err != nil {
		return fmt.Errorf("enable Bluetooth adapter: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	classifier := armClassifier{}
	scanDone := make(chan struct{})
	defer close(scanDone)

	go func() {
		select {
		case <-ctx.Done():
			// Scan is blocking on Darwin. StopScan releases it.
			_ = s.adapter.StopScan()
		case <-scanDone:
		}
	}()

	err := s.adapter.Scan(func(_ *bluetooth.Adapter, raw bluetooth.ScanResult) {
		select {
		case <-ctx.Done():
			return
		default:
		}

		name := raw.LocalName()
		if !strings.Contains(strings.ToUpper(name), "EVEN") {
			return
		}

		arm, ok := classifier.classify(name)
		if !ok {
			return
		}
		report(ScanResult{
			Arm:     arm,
			Address: raw.Address.String(),
			Name:    name,
			RSSI:    raw.RSSI,
		})
	})
	if err != nil {
		return fmt.Errorf("scan Bluetooth devices: %w", err)
	}
	return nil
}

type armClassifier struct {
	mu        sync.Mutex
	leftUsed  bool
	rightUsed bool
}

func (c *armClassifier) classify(name string) (Arm, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	upper := strings.ToUpper(name)
	switch {
	case strings.Contains(upper, "_L_") || strings.Contains(upper, "LEFT"):
		c.leftUsed = true
		return Left, true
	case strings.Contains(upper, "_R_") || strings.Contains(upper, "RIGHT"):
		c.rightUsed = true
		return Right, true
	case !c.leftUsed:
		c.leftUsed = true
		return Left, true
	case !c.rightUsed:
		c.rightUsed = true
		return Right, true
	default:
		return Left, false
	}
}
