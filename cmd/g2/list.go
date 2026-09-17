package main

import (
	"context"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

type listDisplay interface {
	DisplayList(context.Context, string, []string) error
	ShowText(context.Context, string, string) error
}

type listNavigation struct {
	rows           []string
	detail         bool
	lastNavigation time.Time
}

func (n *listNavigation) handle(ctx context.Context, display listDisplay, event protocol.EvenHubEvent, now time.Time) error {
	if event.Type == protocol.EvenHubEventDoubleClick {
		if !n.detail || (event.Kind != protocol.EvenHubEventSystem && event.Kind != protocol.EvenHubEventText && event.Kind != protocol.EvenHubEventList) {
			return nil
		}
		if err := display.DisplayList(ctx, "hud", n.rows); err != nil {
			return err
		}
		n.detail = false
		n.lastNavigation = now
		return nil
	}
	if n.detail || event.Kind != protocol.EvenHubEventList || event.Type != protocol.EvenHubEventClick || event.Name != "hud" {
		return nil
	}
	// Firmware can report the same physical tap more than once, including
	// trailing clicks after returning from the detail page.
	if now.Sub(n.lastNavigation) < 350*time.Millisecond {
		return nil
	}
	if event.ItemIndex < 0 || event.ItemIndex >= len(n.rows) {
		return nil
	}
	if err := display.ShowText(ctx, "hud", n.rows[event.ItemIndex]+"\n\n双击返回列表"); err != nil {
		return err
	}
	n.detail = true
	n.lastNavigation = now
	return nil
}
