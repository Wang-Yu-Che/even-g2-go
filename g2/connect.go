package g2

import (
	"context"
	"io"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
)

// ConnectOptions configures the high-level macOS connection flow.
type ConnectOptions struct {
	ScanTimeout    time.Duration
	Debug          bool
	Output         io.Writer
	AutoReconnect  bool
	ReconnectDelay time.Duration
}

// Connect scans, connects both arms, subscribes to notifications, and authenticates.
func Connect(ctx context.Context, options ConnectOptions) (*Client, error) {
	transport := ble.NewTransport()
	devices, err := scanDevices(ctx, transport, ScanOptions{Timeout: options.ScanTimeout})
	if err != nil {
		_ = transport.Close()
		return nil, err
	}
	if len(devices) == 0 {
		_ = transport.Close()
		return nil, ErrNoDevice
	}
	if len(devices) > 1 {
		_ = transport.Close()
		return nil, ErrMultipleDevices
	}
	client, err := connectWithTransport(ctx, transport, devices[0], options)
	if err != nil {
		_ = transport.Close()
		return nil, err
	}
	return client, nil
}

// ConnectDevice connects an explicitly selected G2 pair.
func ConnectDevice(ctx context.Context, device Device, options ConnectOptions) (*Client, error) {
	transport := ble.NewTransport()
	client, err := connectWithTransport(ctx, transport, device, options)
	if err != nil {
		_ = transport.Close()
		return nil, err
	}
	return client, nil
}

func connectWithTransport(ctx context.Context, transport ble.Transport, device Device, options ConnectOptions) (*Client, error) {
	for _, selected := range []struct {
		arm    ble.Arm
		result ble.ScanResult
	}{{ble.Left, device.Left}, {ble.Right, device.Right}} {
		if err := transport.Connect(ctx, selected.arm, selected.result); err != nil {
			return nil, err
		}
	}
	client := NewClient(transport, options.Debug, options.Output)
	client.device = device
	if err := client.attachTransportNotifications(ctx); err != nil {
		_ = client.Close()
		return nil, err
	}
	if err := client.Authenticate(ctx); err != nil {
		return nil, err
	}
	if options.AutoReconnect {
		delay := options.ReconnectDelay
		if delay <= 0 {
			delay = time.Second
		}
		client.startAutoReconnect(ctx, delay)
	}
	return client, nil
}

// Reconnect reconnects both previously discovered arms and authenticates again.
func (c *Client) Reconnect(ctx context.Context) error {
	c.setState(Reconnecting)
	c.stopDisplayRefresh()
	c.stopHeartbeat()
	c.stopNotifications()
	c.notificationWG.Wait()
	for _, selected := range []struct {
		arm    ble.Arm
		result ble.ScanResult
	}{{ble.Left, c.device.Left}, {ble.Right, c.device.Right}} {
		if err := c.transport.Connect(ctx, selected.arm, selected.result); err != nil {
			return err
		}
	}
	if err := c.attachTransportNotifications(ctx); err != nil {
		return err
	}
	return c.Authenticate(ctx)
}

// Disconnect stops background work and disconnects the selected device.
// The client may be connected again with Reconnect.
func (c *Client) Disconnect() error {
	c.stopAutoReconnect()
	c.stopDisplayRefresh()
	c.stopLifecycle()
	c.stopHeartbeat()
	c.stopNotifications()
	c.failPendingAcks(ble.ErrDisconnected)
	err := c.transport.Close()
	c.notificationWG.Wait()
	c.setState(Disconnected)
	return err
}

func (c *Client) attachTransportNotifications(ctx context.Context) error {
	c.audioAvailable.Store(false)
	for _, arm := range []ble.Arm{ble.Left, ble.Right} {
		notifications, err := c.transport.Subscribe(ctx, arm)
		if err != nil {
			return err
		}
		c.AttachNotifications(ctx, arm, notifications)
	}
	if audioTransport, ok := c.transport.(ble.AudioTransport); ok {
		for _, arm := range []ble.Arm{ble.Left, ble.Right} {
			notifications, err := audioTransport.SubscribeAudio(ctx, arm)
			if err != nil {
				continue
			}
			c.audioAvailable.Store(true)
			c.AttachAudioNotifications(ctx, arm, notifications)
		}
	}
	if fileTransport, ok := c.transport.(ble.FileTransport); ok {
		notifications, err := fileTransport.SubscribeFile(ctx, ble.Right)
		if err == nil {
			c.resetFileService()
			c.AttachFileNotifications(ctx, notifications)
		}
	}
	c.publishStatus()
	return nil
}

func (c *Client) startAutoReconnect(parent context.Context, delay time.Duration) {
	ctx, cancel := context.WithCancel(parent)
	c.reconnectMu.Lock()
	c.reconnectCancel = cancel
	c.reconnectWG.Add(1)
	c.reconnectMu.Unlock()
	go func() {
		defer c.reconnectWG.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case <-c.disconnects:
			}
			for {
				if err := sleepContext(ctx, delay); err != nil {
					return
				}
				if err := c.Reconnect(ctx); err == nil {
					for len(c.disconnects) > 0 {
						<-c.disconnects
					}
					break
				}
			}
		}
	}()
}

func (c *Client) stopAutoReconnect() {
	c.reconnectMu.Lock()
	cancel := c.reconnectCancel
	c.reconnectCancel = nil
	c.reconnectMu.Unlock()
	if cancel != nil {
		cancel()
		c.reconnectWG.Wait()
	}
}
