//go:build darwin

package ble

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"tinygo.org/x/bluetooth"
)

const (
	serviceUUID   = "6E400001-B5A3-F393-E0A9-E50E24DCCA9E"
	writeUUID     = "00002760-08C2-11E1-9073-0E8AC72E5401"
	notifyUUID    = "00002760-08C2-11E1-9073-0E8AC72E5402"
	micNotifyUUID = "00002760-08C2-11E1-9073-0E8AC72E6402"
)

// DarwinTransport implements Transport using CoreBluetooth through
// tinygo.org/x/bluetooth.
type DarwinTransport struct {
	adapter *bluetooth.Adapter
	scanner *Scanner

	mu          sync.Mutex
	discovered  map[Arm]ScanResult
	connections map[Arm]*armConnection
}

var _ Transport = (*DarwinTransport)(nil)

type armConnection struct {
	address   string
	device    bluetooth.Device
	write     bluetooth.DeviceCharacteristic
	notify    bluetooth.DeviceCharacteristic
	micNotify bluetooth.DeviceCharacteristic

	writeMu            sync.Mutex
	notifyMu           sync.Mutex
	notifications      chan []byte
	audioNotifications chan []byte
	closed             bool
	closeOnce          sync.Once
}

// NewTransport returns the macOS BLE transport.
func NewTransport() *DarwinTransport {
	adapter := bluetooth.DefaultAdapter
	t := &DarwinTransport{
		adapter:     adapter,
		scanner:     &Scanner{adapter: adapter},
		discovered:  make(map[Arm]ScanResult, 2),
		connections: make(map[Arm]*armConnection, 2),
	}
	adapter.SetConnectHandler(func(device bluetooth.Device, connected bool) {
		if !connected {
			t.handleDisconnect(device.Address.String())
		}
	})
	return t
}

// Scan discovers G2 arms and remembers their addresses for Connect.
func (t *DarwinTransport) Scan(ctx context.Context, report func(ScanResult)) error {
	if report == nil {
		return errors.New("scan report callback is nil")
	}
	return t.scanner.Scan(ctx, func(result ScanResult) {
		t.mu.Lock()
		if _, exists := t.discovered[result.Arm]; !exists {
			t.discovered[result.Arm] = result
		}
		t.mu.Unlock()
		report(result)
	})
}

// Connect connects one discovered arm, discovers all GATT entries, selects the
// control characteristics, and enables notifications.
func (t *DarwinTransport) Connect(ctx context.Context, arm Arm) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	t.mu.Lock()
	result, found := t.discovered[arm]
	_, connected := t.connections[arm]
	t.mu.Unlock()
	if connected {
		return nil
	}
	if !found {
		if arm == Left {
			return ErrLeftArmNotFound
		}
		return ErrRightArmNotFound
	}

	uuid, err := bluetooth.ParseUUID(result.Address)
	if err != nil {
		return fmt.Errorf("parse %s arm address %q: %w", arm, result.Address, err)
	}
	address := bluetooth.Address{UUID: uuid}
	device, err := t.adapter.Connect(address, bluetooth.ConnectionParams{})
	if err != nil {
		return fmt.Errorf("connect %s arm: %w", arm, err)
	}
	fail := func(err error) error {
		_ = device.Disconnect()
		return err
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}

	services, err := device.DiscoverServices(nil)
	if err != nil {
		return fail(fmt.Errorf("discover %s arm services: %w", arm, err))
	}

	var write bluetooth.DeviceCharacteristic
	var notify bluetooth.DeviceCharacteristic
	var micNotify bluetooth.DeviceCharacteristic
	serviceFound := false
	for _, service := range services {
		if sameUUID(service.UUID().String(), serviceUUID) {
			serviceFound = true
		}
		characteristics, discoverErr := service.DiscoverCharacteristics(nil)
		if discoverErr != nil {
			continue
		}
		for _, characteristic := range characteristics {
			switch {
			case sameUUID(characteristic.UUID().String(), writeUUID):
				write = characteristic
			case sameUUID(characteristic.UUID().String(), notifyUUID):
				notify = characteristic
			case sameUUID(characteristic.UUID().String(), micNotifyUUID):
				micNotify = characteristic
			}
		}
	}
	if !serviceFound {
		return fail(fmt.Errorf("%w on %s arm", ErrServiceNotFound, arm))
	}
	if write == (bluetooth.DeviceCharacteristic{}) {
		return fail(fmt.Errorf("%w on %s arm: write %s", ErrCharacteristicNotFound, arm, writeUUID))
	}
	if notify == (bluetooth.DeviceCharacteristic{}) {
		return fail(fmt.Errorf("%w on %s arm: notify %s", ErrCharacteristicNotFound, arm, notifyUUID))
	}

	connection := &armConnection{
		address:            result.Address,
		device:             device,
		write:              write,
		notify:             notify,
		micNotify:          micNotify,
		notifications:      make(chan []byte, 32),
		audioNotifications: make(chan []byte, 64),
	}
	if err := connection.notify.EnableNotifications(connection.deliver); err != nil {
		return fail(fmt.Errorf("subscribe %s arm notifications: %w", arm, err))
	}
	if connection.micNotify != (bluetooth.DeviceCharacteristic{}) {
		if err := connection.micNotify.EnableNotifications(connection.deliverAudio); err != nil {
			connection.micNotify = bluetooth.DeviceCharacteristic{}
		}
	}
	if err := ctx.Err(); err != nil {
		connection.close(true)
		return err
	}

	t.mu.Lock()
	if existing := t.connections[arm]; existing != nil {
		t.mu.Unlock()
		connection.close(true)
		return nil
	}
	t.connections[arm] = connection
	t.mu.Unlock()
	return nil
}

func sameUUID(left, right string) bool {
	normalize := func(value string) string {
		return strings.ToUpper(strings.ReplaceAll(value, "-", ""))
	}
	return normalize(left) == normalize(right)
}

// Write sends one packet through the arm's control characteristic without a
// response. The library waits for CoreBluetooth TX capacity before writing.
func (t *DarwinTransport) Write(ctx context.Context, arm Arm, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	connection, err := t.connection(arm)
	if err != nil {
		return err
	}

	connection.writeMu.Lock()
	defer connection.writeMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	connected, err := connection.device.Connected()
	if err != nil {
		return fmt.Errorf("check %s arm connection: %w", arm, err)
	}
	if !connected {
		return fmt.Errorf("%w: %s arm", ErrDisconnected, arm)
	}
	written, err := connection.write.WriteWithoutResponse(data)
	if err != nil {
		return fmt.Errorf("write %s arm: %w", arm, err)
	}
	if written != len(data) {
		return fmt.Errorf("write %s arm: wrote %d of %d bytes", arm, written, len(data))
	}
	return nil
}

// Subscribe returns the notification stream enabled during Connect.
func (t *DarwinTransport) Subscribe(ctx context.Context, arm Arm) (<-chan []byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	connection, err := t.connection(arm)
	if err != nil {
		return nil, err
	}
	return connection.notifications, nil
}

// SubscribeAudio returns raw LC3 microphone frames from the 6402 characteristic.
func (t *DarwinTransport) SubscribeAudio(ctx context.Context, arm Arm) (<-chan []byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	connection, err := t.connection(arm)
	if err != nil {
		return nil, err
	}
	if connection.micNotify == (bluetooth.DeviceCharacteristic{}) {
		return nil, fmt.Errorf("%w on %s arm: microphone notify %s", ErrCharacteristicNotFound, arm, micNotifyUUID)
	}
	return connection.audioNotifications, nil
}

// Close disconnects both arms and closes their notification streams.
func (t *DarwinTransport) Close() error {
	t.mu.Lock()
	connections := make([]*armConnection, 0, len(t.connections))
	for arm, connection := range t.connections {
		connections = append(connections, connection)
		delete(t.connections, arm)
	}
	t.mu.Unlock()

	var closeErrors []error
	for _, connection := range connections {
		if err := connection.close(true); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	return errors.Join(closeErrors...)
}

func (t *DarwinTransport) connection(arm Arm) (*armConnection, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	connection := t.connections[arm]
	if connection == nil {
		return nil, fmt.Errorf("%w: %s arm", ErrDisconnected, arm)
	}
	return connection, nil
}

func (c *armConnection) deliver(data []byte) {
	c.notifyMu.Lock()
	defer c.notifyMu.Unlock()
	if c.closed {
		return
	}
	packet := append([]byte(nil), data...)
	select {
	case c.notifications <- packet:
	default:
	}
}

func (c *armConnection) deliverAudio(data []byte) {
	c.notifyMu.Lock()
	defer c.notifyMu.Unlock()
	if c.closed {
		return
	}
	select {
	case c.audioNotifications <- append([]byte(nil), data...):
	default:
	}
}

func (c *armConnection) close(disconnect bool) error {
	var err error
	c.closeOnce.Do(func() {
		if disconnect {
			err = c.device.Disconnect()
		}
		c.notifyMu.Lock()
		defer c.notifyMu.Unlock()
		c.closed = true
		close(c.notifications)
		if c.audioNotifications != nil {
			close(c.audioNotifications)
		}
	})
	return err
}

func (t *DarwinTransport) handleDisconnect(address string) {
	t.mu.Lock()
	var disconnected *armConnection
	for arm, connection := range t.connections {
		if sameUUID(connection.address, address) {
			disconnected = connection
			delete(t.connections, arm)
			break
		}
	}
	t.mu.Unlock()
	if disconnected != nil {
		_ = disconnected.close(false)
	}
}
