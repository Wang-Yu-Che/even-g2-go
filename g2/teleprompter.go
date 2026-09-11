package g2

import (
	"context"
	"errors"
	"fmt"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

var ErrDisplayFailed = errors.New("G2 teleprompter display failed")

// DisplayText renders text using the verified teleprompter flow and refreshes
// the view before the firmware's display timeout.
func (c *Client) DisplayText(ctx context.Context, text string) error {
	if c.Left.State() != Ready || c.Right.State() != Ready {
		return ble.ErrDisconnected
	}
	c.stopDisplayRefresh()

	c.displayMu.Lock()
	err := c.sendTeleprompter(ctx, ctx, text)
	c.displayMu.Unlock()
	if err != nil {
		return c.displayFailure(err)
	}
	c.startDisplayRefresh(ctx, text)
	return nil
}

func (c *Client) sendTeleprompter(operationCtx, heartbeatCtx context.Context, text string) error {
	c.stopHeartbeat()
	pages := protocol.FormatTeleprompter(text)
	totalLines := protocol.TeleprompterInputLineCount(text)
	sequence := byte(0x08)
	messageID := 0x14

	send := func(packet []byte) error {
		if err := c.sendBoth(operationCtx, packet); err != nil {
			return err
		}
		sequence++
		messageID++
		return nil
	}
	if err := send(protocol.BuildDisplayConfig(sequence, messageID)); err != nil {
		return err
	}
	if err := sleepContext(operationCtx, c.displayConfigDelay); err != nil {
		return err
	}
	if err := send(protocol.BuildTeleprompterInit(sequence, messageID, totalLines, true)); err != nil {
		return err
	}
	if err := sleepContext(operationCtx, c.displayInitDelay); err != nil {
		return err
	}

	for index := 0; index < min(10, len(pages)); index++ {
		c.log("[TEXT] sending page %d/%d\n", index+1, len(pages))
		if err := send(protocol.BuildContentPage(sequence, messageID, pages[index])); err != nil {
			return err
		}
		if err := sleepContext(operationCtx, c.displayFirstPageDelay); err != nil {
			return err
		}
	}
	if err := send(protocol.BuildMarker(sequence, messageID)); err != nil {
		return err
	}
	if err := sleepContext(operationCtx, c.displayMarkerDelay); err != nil {
		return err
	}
	for index := 10; index < min(12, len(pages)); index++ {
		c.log("[TEXT] sending page %d/%d\n", index+1, len(pages))
		if err := send(protocol.BuildContentPage(sequence, messageID, pages[index])); err != nil {
			return err
		}
		if err := sleepContext(operationCtx, c.displayLatePageDelay); err != nil {
			return err
		}
	}
	if err := send(protocol.BuildSync(sequence, messageID)); err != nil {
		return err
	}
	if err := sleepContext(operationCtx, c.displaySyncDelay); err != nil {
		return err
	}
	for index := 12; index < len(pages); index++ {
		c.log("[TEXT] sending page %d/%d\n", index+1, len(pages))
		if err := send(protocol.BuildContentPage(sequence, messageID, pages[index])); err != nil {
			return err
		}
		if err := sleepContext(operationCtx, c.displayLatePageDelay); err != nil {
			return err
		}
	}

	c.log("[TEXT] complete\n")
	c.startHeartbeat(heartbeatCtx)
	return nil
}

func (c *Client) startDisplayRefresh(parent context.Context, text string) {
	if c.displayRefreshInterval <= 0 {
		return
	}
	c.stopDisplayRefresh()
	ctx, cancel := context.WithCancel(parent)
	c.displayRefreshMu.Lock()
	c.displayRefreshCancel = cancel
	c.displayRefreshWG.Add(1)
	c.displayRefreshMu.Unlock()

	go func() {
		defer c.displayRefreshWG.Done()
		for {
			if err := sleepContext(ctx, c.displayRefreshInterval); err != nil {
				return
			}
			c.displayMu.Lock()
			err := c.sendTeleprompter(ctx, parent, text)
			c.displayMu.Unlock()
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					c.displayBackgroundFailure(err)
				}
				return
			}
		}
	}()
}

func (c *Client) stopDisplayRefresh() {
	c.displayRefreshMu.Lock()
	cancel := c.displayRefreshCancel
	c.displayRefreshCancel = nil
	c.displayRefreshMu.Unlock()
	if cancel != nil {
		cancel()
		c.displayRefreshWG.Wait()
	}
}

func (c *Client) displayFailure(cause error) error {
	c.stopHeartbeat()
	c.setState(Disconnected)
	closeErr := c.transport.Close()
	return errors.Join(fmt.Errorf("%w: %w", ErrDisplayFailed, cause), closeErr)
}

func (c *Client) displayBackgroundFailure(cause error) {
	c.stopHeartbeat()
	c.setState(Disconnected)
	_ = c.transport.Close()
	c.emitError(fmt.Errorf("%w: %w", ErrDisplayFailed, cause))
}
