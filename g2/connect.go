package g2

import (
	"context"
	"errors"
	"io"
	"sync"
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
	client, err := connectWithTransport(ctx, transport, options)
	if err != nil {
		_ = transport.Close()
		return nil, err
	}
	return client, nil
}

func connectWithTransport(ctx context.Context, transport ble.Transport, options ConnectOptions) (*Client, error) {
	if options.ScanTimeout <= 0 {
		options.ScanTimeout = 20 * time.Second
	}
	if err := discoverBoth(ctx, transport, options.ScanTimeout); err != nil {
		return nil, err
	}
	for _, arm := range []ble.Arm{ble.Left, ble.Right} {
		if err := transport.Connect(ctx, arm); err != nil {
			return nil, err
		}
	}
	client := NewClient(transport, options.Debug, options.Output)
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
	c.stopDisplayRefresh()
	c.stopHeartbeat()
	c.stopNotifications()
	c.notificationWG.Wait()
	for _, arm := range []ble.Arm{ble.Left, ble.Right} {
		if err := c.transport.Connect(ctx, arm); err != nil {
			return err
		}
	}
	if err := c.attachTransportNotifications(ctx); err != nil {
		return err
	}
	return c.Authenticate(ctx)
}

func (c *Client) attachTransportNotifications(ctx context.Context) error {
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
			c.audioAvailable = true
			c.AttachAudioNotifications(ctx, arm, notifications)
		}
	}
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

func discoverBoth(ctx context.Context, transport ble.Transport, timeout time.Duration) error {
	scanCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	found := make(map[ble.Arm]bool, 2)
	var mu sync.Mutex
	err := transport.Scan(scanCtx, func(result ble.ScanResult) {
		mu.Lock()
		found[result.Arm] = true
		complete := found[ble.Left] && found[ble.Right]
		mu.Unlock()
		if complete {
			cancel()
		}
	})
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	mu.Lock()
	left, right := found[ble.Left], found[ble.Right]
	mu.Unlock()
	if !left {
		return ble.ErrLeftArmNotFound
	}
	if !right {
		return ble.ErrRightArmNotFound
	}
	return nil
}
