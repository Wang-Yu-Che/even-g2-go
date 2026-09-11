package g2

import (
	"context"
	"errors"
	"fmt"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

var ErrEvenHubRejected = errors.New("EvenHub command rejected")

type nativeShape int

const (
	nativeShapeNone nativeShape = iota
	nativeShapeList
	nativeShapeText
)

// DisplayList shows a full-lens native tappable list.
func (c *Client) DisplayList(ctx context.Context, name string, rows []string) error {
	c.nativeMu.Lock()
	defer c.nativeMu.Unlock()
	if err := c.prepareNative(ctx); err != nil {
		return err
	}
	magic := c.nextEvenHubMagic()
	payload, err := protocol.BuildEvenHubRebuildList(name, rows, magic)
	if err != nil {
		return err
	}
	if err := c.sendNativeCommand(ctx, payload, magic); err != nil {
		return err
	}
	c.nativeShape = nativeShapeList
	c.startHeartbeat(ctx)
	return nil
}

// ShowText switches the full-lens native container to text.
func (c *Client) ShowText(ctx context.Context, name, content string) error {
	c.nativeMu.Lock()
	defer c.nativeMu.Unlock()
	return c.showTextLocked(ctx, name, content)
}

// UpdateText replaces text in place, or creates the text shape when necessary.
func (c *Client) UpdateText(ctx context.Context, name, content string) error {
	c.nativeMu.Lock()
	defer c.nativeMu.Unlock()
	if err := c.prepareNative(ctx); err != nil {
		return err
	}
	if c.nativeShape != nativeShapeText {
		return c.showTextPrepared(ctx, name, content)
	}
	magic := c.nextEvenHubMagic()
	payload, err := protocol.BuildEvenHubTextUpgrade(name, content, magic)
	if err != nil {
		return err
	}
	if err := c.sendNativeCommand(ctx, payload, magic); err != nil {
		return err
	}
	c.startHeartbeat(ctx)
	return nil
}

func (c *Client) showTextLocked(ctx context.Context, name, content string) error {
	if err := c.prepareNative(ctx); err != nil {
		return err
	}
	return c.showTextPrepared(ctx, name, content)
}

func (c *Client) showTextPrepared(ctx context.Context, name, content string) error {
	magic := c.nextEvenHubMagic()
	payload, err := protocol.BuildEvenHubRebuildText(name, content, magic)
	if err != nil {
		return err
	}
	if err := c.sendNativeCommand(ctx, payload, magic); err != nil {
		return err
	}
	c.nativeShape = nativeShapeText
	c.startHeartbeat(ctx)
	return nil
}

func (c *Client) prepareNative(ctx context.Context) error {
	if c.Left.State() != Ready || c.Right.State() != Ready {
		return ble.ErrDisconnected
	}
	c.stopDisplayRefresh()
	c.stopHeartbeat()
	if c.nativeCreated {
		return nil
	}
	c.logPacket("TX", ble.Right, protocol.EvenHubPrelude)
	if err := c.transport.Write(ctx, ble.Right, protocol.EvenHubPrelude); err != nil {
		c.startHeartbeat(ctx)
		return err
	}
	if err := sleepContext(ctx, c.nativePreludeDelay); err != nil {
		c.startHeartbeat(ctx)
		return err
	}
	payload, err := protocol.BuildEvenHubCreateList("hud", []string{"…"}, 201)
	if err != nil {
		return err
	}
	response, err := c.sendEvenHubAck(ctx, payload, 201, c.evenHubAckTimeout)
	if err != nil {
		c.startHeartbeat(ctx)
		return err
	}
	if response.Result != nil && *response.Result%2 != 0 {
		c.startHeartbeat(ctx)
		return fmt.Errorf("%w: create result=%d", ErrEvenHubRejected, *response.Result)
	}
	c.nativeCreated = true
	c.evenHubMu.Lock()
	c.evenHubActive = true
	c.evenHubMu.Unlock()
	return sleepContext(ctx, c.nativeCreateDelay)
}

func (c *Client) sendNativeCommand(ctx context.Context, payload []byte, magic int) error {
	response, err := c.sendEvenHubAck(ctx, payload, magic, c.evenHubAckTimeout)
	if err != nil {
		return err
	}
	if response.Result == nil || *response.Result%2 != 0 {
		return fmt.Errorf("%w: result=%v", ErrEvenHubRejected, response.Result)
	}
	return nil
}
