package g2

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

var ErrEvenHubRejected = errors.New("EvenHub command rejected")

type nativeShape int

// TextStyle controls the native text container used by ShowTextWithStyle.
type TextStyle struct {
	X, Y, Width, Height                                   int
	BorderWidth, BorderColor, BorderRadius, PaddingLength int
}

// StatusIcon is a small 4-bit BMP rendered beside native text.
type StatusIcon struct {
	ID, X, Y, Width, Height int
	Name                    string
	BMP                     []byte
}

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

// ShowTextWithStyle switches to a positioned native text container with a
// border, rounded corners and padding.
func (c *Client) ShowTextWithStyle(ctx context.Context, name, content string, style TextStyle) error {
	c.nativeMu.Lock()
	defer c.nativeMu.Unlock()
	if err := c.prepareNative(ctx); err != nil {
		return err
	}
	magic := c.nextEvenHubMagic()
	payload, err := protocol.BuildEvenHubRebuildStyledText(name, content, magic,
		protocol.EvenHubGeometry{X: style.X, Y: style.Y, Width: style.Width, Height: style.Height},
		protocol.EvenHubTextStyle{BorderWidth: style.BorderWidth, BorderColor: style.BorderColor, BorderRadius: style.BorderRadius, PaddingLength: style.PaddingLength})
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

// ShowTextWithIcon creates a native page with independently updateable text
// and bitmap containers. The bitmap is transmitted only when this method is
// called; later text updates keep using the cheap Cmd=5 path.
func (c *Client) ShowTextWithIcon(ctx context.Context, name, content string, style TextStyle, icon StatusIcon) error {
	return c.ShowTextWithIcons(ctx, name, content, style, []StatusIcon{icon})
}

// ShowTextWithIcons creates a native page with multiple static image
// containers. The last icon remains available to UpdateStatusIcon.
func (c *Client) ShowTextWithIcons(ctx context.Context, name, content string, style TextStyle, icons []StatusIcon) error {
	if len(icons) == 0 {
		return errors.New("at least one native icon is required")
	}
	c.imageMu.Lock()
	defer c.imageMu.Unlock()
	c.nativeMu.Lock()
	defer c.nativeMu.Unlock()
	if c.Left.State() != Ready || c.Right.State() != Ready {
		return ble.ErrDisconnected
	}
	c.stopDisplayRefresh()
	c.stopHeartbeat()
	defer c.startHeartbeat(ctx)
	protocolIcons := make([]protocol.EvenHubImage, 0, len(icons))
	for _, icon := range icons {
		protocolIcons = append(protocolIcons, protocol.EvenHubImage{ID: icon.ID, Name: icon.Name, X: icon.X, Y: icon.Y, Width: icon.Width, Height: icon.Height, BMP: icon.BMP})
	}
	geometry := protocol.EvenHubGeometry{X: style.X, Y: style.Y, Width: style.Width, Height: style.Height}
	textStyle := protocol.EvenHubTextStyle{BorderWidth: style.BorderWidth, BorderColor: style.BorderColor, BorderRadius: style.BorderRadius, PaddingLength: style.PaddingLength}
	warmup := false
	if c.nativeCreated {
		magic := c.nextEvenHubMagic()
		payload, err := protocol.BuildEvenHubRebuildTextImages(name, content, geometry, textStyle, protocolIcons, magic)
		if err != nil {
			return err
		}
		if err := c.sendNativeCommand(ctx, payload, magic); err != nil {
			return err
		}
	} else {
		c.logPacket("TX", ble.Right, protocol.EvenHubPrelude)
		if err := c.transport.Write(ctx, ble.Right, protocol.EvenHubPrelude); err != nil {
			return err
		}
		if err := sleepContext(ctx, c.nativePreludeDelay); err != nil {
			return err
		}
		payload, err := protocol.BuildEvenHubCreateTextImages(name, content, geometry, textStyle, protocolIcons, 201)
		if err != nil {
			return err
		}
		response, err := c.sendEvenHubAck(ctx, payload, 201, c.evenHubAckTimeout)
		if err != nil {
			return err
		}
		if response.Result != nil && *response.Result%2 != 0 {
			return fmt.Errorf("%w: create result=%d", ErrEvenHubRejected, *response.Result)
		}
		if err := sleepContext(ctx, c.nativeCreateDelay); err != nil {
			return err
		}
		warmup = true
	}
	c.nativeCreated = true
	c.nativeShape = nativeShapeText
	updateIcon := icons[len(icons)-1]
	c.nativeIconID, c.nativeIconName = updateIcon.ID, updateIcon.Name
	c.evenHubMu.Lock()
	c.evenHubActive = true
	c.evenHubMu.Unlock()
	// Firmware drops the first image burst only after CREATE. Rebuilds can send
	// every image once, in order, without a sacrificial transfer.
	for index, icon := range icons {
		if err := c.sendNativeIcon(ctx, icon, warmup && index == 0); err != nil {
			return err
		}
	}
	return nil
}

// UpdateStatusIcon replaces the bitmap in a mixed native page.
func (c *Client) UpdateStatusIcon(ctx context.Context, icon StatusIcon) error {
	c.imageMu.Lock()
	defer c.imageMu.Unlock()
	if !c.nativeCreated || c.nativeIconID != icon.ID || c.nativeIconName != icon.Name {
		return errors.New("native icon container is not active")
	}
	return c.sendNativeIcon(ctx, icon, false)
}

func (c *Client) sendNativeIcon(ctx context.Context, icon StatusIcon, warmup bool) error {
	sends := 1
	if warmup {
		sends = 2
	}
	for range sends {
		magic := c.nextEvenHubMagic()
		payload, err := protocol.BuildEvenHubImageFragment(icon.ID, icon.Name, c.nextImageSession(), len(icon.BMP), 0, icon.BMP, magic)
		if err != nil {
			return err
		}
		response, err := c.sendEvenHubAck(ctx, payload, magic, 10*time.Second)
		if err != nil {
			return err
		}
		if response.Result == nil || *response.Result != 4 {
			return ErrEvenHubRejected
		}
	}
	return nil
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

// ShutdownNative closes the active EvenHub page and frees its containers.
func (c *Client) ShutdownNative(ctx context.Context) error {
	c.nativeMu.Lock()
	defer c.nativeMu.Unlock()

	if c.Left.State() != Ready || c.Right.State() != Ready {
		return ble.ErrDisconnected
	}
	c.stopDisplayRefresh()
	c.stopHeartbeat()
	if !c.nativeCreated {
		c.startHeartbeat(ctx)
		return nil
	}

	magic := c.nextEvenHubMagic()
	if err := c.sendNativeCommand(ctx, protocol.BuildEvenHubShutdown(magic), magic); err != nil {
		c.startHeartbeat(ctx)
		return err
	}
	c.nativeCreated = false
	c.nativeShape = nativeShapeNone
	c.nativeIconID = 0
	c.nativeIconName = ""
	c.evenHubMu.Lock()
	c.evenHubActive = false
	c.evenHubMu.Unlock()
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
