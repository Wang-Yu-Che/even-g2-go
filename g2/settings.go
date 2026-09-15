package g2

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

var (
	ErrNotReady           = errors.New("G2 client is not ready")
	ErrSettingsAckTimeout = errors.New("G2 settings acknowledgement timed out")
)

type DeviceSettings = protocol.DeviceSettingsSnapshot

type BrightnessOptions struct {
	Level int  `json:"level"`
	Auto  bool `json:"auto"`
}
type ScreenPosition struct {
	Height int `json:"height"`
	Depth  int `json:"depth"`
}

type settingsReply struct {
	payload []byte
	err     error
}

// RequestDeviceSettings reads battery, firmware and persisted display settings.
func (c *Client) RequestDeviceSettings(ctx context.Context) (DeviceSettings, error) {
	magic := c.nextEvenHubMagic()
	reply, err := c.sendSettings(ctx, protocol.BuildSettingsQuery(magic), magic)
	if err != nil {
		return DeviceSettings{}, err
	}
	settings, err := protocol.ParseDeviceSettings(reply)
	if err != nil {
		return DeviceSettings{}, err
	}
	c.settingsMu.Lock()
	c.settings = settings
	c.settingsMu.Unlock()
	c.settingsKnown.Store(true)
	c.publishStatus()
	return settings, nil
}

func (c *Client) SetBrightness(ctx context.Context, options BrightnessOptions) error {
	magic := c.nextEvenHubMagic()
	payload, err := protocol.BuildSetBrightness(magic, options.Level, options.Auto)
	if err != nil {
		return err
	}
	_, err = c.sendSettings(ctx, payload, magic)
	return err
}

func (c *Client) SetHeadUp(ctx context.Context, enabled bool, angle int) error {
	switchMagic := c.nextEvenHubMagic()
	angleMagic := c.nextEvenHubMagic()
	anglePayload, err := protocol.BuildSetHeadUpAngle(angleMagic, angle)
	if err != nil {
		return err
	}
	if _, err := c.sendSettings(ctx, protocol.BuildSetHeadUpSwitch(switchMagic, enabled), switchMagic); err != nil {
		return err
	}
	_, err = c.sendSettings(ctx, anglePayload, angleMagic)
	return err
}

func (c *Client) SetScreenPosition(ctx context.Context, position ScreenPosition) error {
	heightMagic := c.nextEvenHubMagic()
	height, err := protocol.BuildSetScreenHeight(heightMagic, position.Height)
	if err != nil {
		return err
	}
	depthMagic := c.nextEvenHubMagic()
	depth, err := protocol.BuildSetScreenDepth(depthMagic, position.Depth)
	if err != nil {
		return err
	}
	if _, err := c.sendSettings(ctx, height, heightMagic); err != nil {
		return err
	}
	_, err = c.sendSettings(ctx, depth, depthMagic)
	return err
}

func (c *Client) sendSettings(ctx context.Context, payload []byte, magic int) ([]byte, error) {
	if c.Left.State() != Ready || c.Right.State() != Ready {
		return nil, ErrNotReady
	}
	waiter := make(chan settingsReply, 1)
	c.evenHubMu.Lock()
	if _, exists := c.pendingSettings[magic]; exists {
		c.evenHubMu.Unlock()
		return nil, fmt.Errorf("settings magic %d is already pending", magic)
	}
	c.pendingSettings[magic] = waiter
	sequence := c.evenHubSequence
	c.evenHubSequence++
	c.evenHubMu.Unlock()
	frames, err := protocol.FrameEvenHub(sequence, protocol.G2SettingsServiceID, protocol.EvenHubRequest, payload, c.evenHubChunkSize)
	if err == nil {
		c.evenHubWriteMu.Lock()
		for _, arm := range []ble.Arm{ble.Left, ble.Right} {
			for _, frame := range frames {
				c.logPacket("TX", arm, frame)
				if err = c.transport.Write(ctx, arm, frame); err != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
		c.evenHubWriteMu.Unlock()
	}
	if err != nil {
		c.removePendingSettings(magic)
		return nil, err
	}
	timer := time.NewTimer(c.evenHubAckTimeout)
	defer timer.Stop()
	select {
	case reply := <-waiter:
		return reply.payload, reply.err
	case <-ctx.Done():
		c.removePendingSettings(magic)
		return nil, ctx.Err()
	case <-timer.C:
		c.removePendingSettings(magic)
		return nil, fmt.Errorf("%w: magic=%d", ErrSettingsAckTimeout, magic)
	}
}

func (c *Client) removePendingSettings(magic int) {
	c.evenHubMu.Lock()
	delete(c.pendingSettings, magic)
	c.evenHubMu.Unlock()
}
