package g2

import (
	"context"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

type DashboardWidget = protocol.DashboardWidget

const (
	DashboardNews      = protocol.DashboardNews
	DashboardStock     = protocol.DashboardStock
	DashboardSchedule  = protocol.DashboardSchedule
	DashboardQuicklist = protocol.DashboardQuicklist
	DashboardHealth    = protocol.DashboardHealth
)

type DashboardConfig struct {
	WidgetOrder []DashboardWidget `json:"widgetOrder"`
	HalfDay     bool              `json:"halfDay"`
	Celsius     bool              `json:"celsius"`
}

type DashboardScheduleItem = protocol.DashboardScheduleItem

// ShowDashboard releases the EvenHub page and restores the stock head-up dashboard.
func (c *Client) ShowDashboard(ctx context.Context) error {
	if err := c.ShutdownNative(ctx); err != nil {
		return err
	}
	magic := c.nextEvenHubMagic()
	_, err := c.sendSettings(ctx, protocol.BuildSetHeadUpSwitch(magic, true), magic)
	return err
}

// ConfigureDashboard changes the stock dashboard. It remains experimental until local hardware confirmation.
func (c *Client) ConfigureDashboard(ctx context.Context, config DashboardConfig) error {
	magic := c.nextEvenHubMagic()
	payload, err := protocol.BuildDashboardConfig(magic, config.WidgetOrder, config.HalfDay, config.Celsius)
	if err != nil {
		return err
	}
	return c.sendDashboard(ctx, payload)
}

// PushDashboardSchedule replaces Schedule widget entries.
func (c *Client) PushDashboardSchedule(ctx context.Context, items []DashboardScheduleItem) error {
	if len(items) == 0 {
		return c.ClearDashboardSchedule(ctx)
	}
	for index, item := range items {
		payload, err := protocol.BuildDashboardSchedule(c.nextEvenHubMagic(), item, len(items), index)
		if err != nil {
			return err
		}
		if err := c.sendDashboard(ctx, payload); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) ClearDashboardSchedule(ctx context.Context) error {
	return c.sendDashboard(ctx, protocol.BuildDashboardScheduleClear(c.nextEvenHubMagic()))
}

func (c *Client) sendDashboard(ctx context.Context, payload []byte) error {
	if c.Left.State() != Ready || c.Right.State() != Ready {
		return ErrNotReady
	}
	c.evenHubMu.Lock()
	sequence := c.evenHubSequence
	c.evenHubSequence++
	c.evenHubMu.Unlock()
	frames, err := protocol.FrameEvenHub(sequence, protocol.DashboardServiceID, protocol.EvenHubRequest, payload, c.evenHubChunkSize)
	if err != nil {
		return err
	}
	c.evenHubWriteMu.Lock()
	defer c.evenHubWriteMu.Unlock()
	for _, arm := range []ble.Arm{ble.Left, ble.Right} {
		for _, frame := range frames {
			c.logPacket("TX", arm, frame)
			if err := c.transport.Write(ctx, arm, frame); err != nil {
				return err
			}
		}
	}
	return nil
}
