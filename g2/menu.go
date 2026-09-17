package g2

import (
	"context"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

type MenuItem = protocol.MenuItem

// SetMenu publishes third-party entries to the glasses dashboard menu.
func (c *Client) SetMenu(ctx context.Context, items []MenuItem) error {
	if c.Left.State() != Ready || c.Right.State() != Ready {
		return ErrNotReady
	}
	payload, appIDs, err := protocol.BuildMenuInfo(c.nextEvenHubMagic(), items)
	if err != nil {
		return err
	}
	if err := c.sendMenu(ctx, payload); err != nil {
		return err
	}
	c.menuMu.Lock()
	c.menuItems = append([]protocol.MenuItem(nil), items...)
	c.menuAppIDs = appIDs
	c.menuMu.Unlock()
	return nil
}

func (c *Client) restoreMenu(ctx context.Context) error {
	c.menuMu.RLock()
	items := append([]protocol.MenuItem(nil), c.menuItems...)
	c.menuMu.RUnlock()
	if len(items) == 0 {
		return nil
	}
	payload, appIDs, err := protocol.BuildMenuInfo(c.nextEvenHubMagic(), items)
	if err != nil {
		return err
	}
	if err := c.sendMenu(ctx, payload); err != nil {
		return err
	}
	c.menuMu.Lock()
	c.menuAppIDs = appIDs
	c.menuMu.Unlock()
	return nil
}

func (c *Client) sendMenu(ctx context.Context, payload []byte) error {
	return c.sendService(ctx, protocol.MenuServiceID, payload, ble.Right)
}

func (c *Client) associateActiveMenuApp(payload []byte) []byte {
	c.menuMu.RLock()
	appID := c.activeMenuAppID
	c.menuMu.RUnlock()
	return protocol.AssociateEvenHubApp(payload, appID)
}
