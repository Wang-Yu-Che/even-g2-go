package g2

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
)

var (
	ErrNoDevice        = errors.New("no complete G2 device found")
	ErrMultipleDevices = errors.New("multiple G2 devices found; select one explicitly")
)

// Device is one complete G2 consisting of its left and right BLE peripherals.
type Device struct {
	ID    string
	Name  string
	Left  ble.ScanResult
	Right ble.ScanResult
}

// ScanOptions configures device discovery.
type ScanOptions struct {
	Timeout time.Duration
}

// ScanDevices discovers complete G2 pairs in stable ID order.
func ScanDevices(ctx context.Context, options ScanOptions) ([]Device, error) {
	transport := ble.NewTransport()
	defer transport.Close()
	return scanDevices(ctx, transport, options)
}

func scanDevices(ctx context.Context, transport ble.Transport, options ScanOptions) ([]Device, error) {
	arms, err := scanArms(ctx, transport, options)
	if err != nil {
		return nil, err
	}
	left := arms[ble.Left]
	right := arms[ble.Right]
	if len(left) == 0 || len(right) == 0 {
		return nil, nil
	}
	if len(left) != 1 || len(right) != 1 {
		return nil, ErrMultipleDevices
	}
	device, err := NewDevice(left[0], right[0])
	if err != nil {
		return nil, err
	}
	return []Device{device}, nil
}

// ScanArms returns all uniquely addressed left and right arm candidates.
// When more than one pair is nearby, the caller must select the matching arms.
func ScanArms(ctx context.Context, options ScanOptions) (map[ble.Arm][]ble.ScanResult, error) {
	transport := ble.NewTransport()
	defer transport.Close()
	return scanArms(ctx, transport, options)
}

func scanArms(ctx context.Context, transport ble.Transport, options ScanOptions) (map[ble.Arm][]ble.ScanResult, error) {
	if options.Timeout <= 0 {
		options.Timeout = 20 * time.Second
	}
	scanCtx, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	results := map[ble.Arm]map[string]ble.ScanResult{ble.Left: {}, ble.Right: {}}
	var mu sync.Mutex
	err := transport.Scan(scanCtx, func(result ble.ScanResult) {
		mu.Lock()
		results[result.Arm][result.Address] = result
		mu.Unlock()
	})
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return nil, err
	}

	mu.Lock()
	arms := map[ble.Arm][]ble.ScanResult{ble.Left: {}, ble.Right: {}}
	for arm, candidates := range results {
		for _, result := range candidates {
			arms[arm] = append(arms[arm], result)
		}
	}
	mu.Unlock()
	return arms, nil
}

// NewDevice validates an explicitly selected left/right pair.
func NewDevice(left, right ble.ScanResult) (Device, error) {
	if left.Arm != ble.Left {
		return Device{}, errors.New("left candidate is not a left arm")
	}
	if right.Arm != ble.Right {
		return Device{}, errors.New("right candidate is not a right arm")
	}
	return Device{ID: left.Address + ":" + right.Address, Name: "Even G2", Left: left, Right: right}, nil
}
