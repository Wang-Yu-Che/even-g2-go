package g2

import (
	"context"
	"fmt"
	"image"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

const (
	imageDataChunk = 3800
	imageAckWindow = 4
)

// DisplayImage scales and streams an image across the full 576x288 lens.
func (c *Client) DisplayImage(ctx context.Context, source image.Image) error {
	tiles, err := protocol.RenderImageTiles(source)
	if err != nil {
		return err
	}
	c.imageMu.Lock()
	defer c.imageMu.Unlock()
	if c.Left.State() != Ready || c.Right.State() != Ready {
		return ble.ErrDisconnected
	}
	c.stopDisplayRefresh()
	c.stopHeartbeat()
	if err := c.primeImages(ctx, tiles); err != nil {
		c.startHeartbeat(ctx)
		return err
	}
	if err := c.sendImage(ctx, tiles); err != nil {
		c.imagePrimed = false
		c.imageWarmed = false
		c.imageSessionJump = true
		if primeErr := c.primeImages(ctx, tiles); primeErr != nil {
			c.startHeartbeat(ctx)
			return errorsJoin(err, primeErr)
		}
		if retryErr := c.sendImage(ctx, tiles); retryErr != nil {
			c.startHeartbeat(ctx)
			return errorsJoin(err, retryErr)
		}
	}
	c.evenHubMu.Lock()
	c.evenHubActive = true
	c.evenHubMu.Unlock()
	c.startHeartbeat(ctx)
	return nil
}

func (c *Client) sendImage(ctx context.Context, tiles []protocol.ImageTile) error {
	if !c.imageWarmed {
		if err := c.streamWarmup(ctx, tiles[0]); err != nil {
			return fmt.Errorf("image warmup: %w", err)
		}
		c.imageWarmed = true
	}
	return c.streamTiles(ctx, tiles)
}

func (c *Client) primeImages(ctx context.Context, tiles []protocol.ImageTile) error {
	if c.imagePrimed {
		return nil
	}
	c.nativeMu.Lock()
	if c.nativeCreated {
		magic := c.nextEvenHubMagic()
		_, _ = c.sendEvenHubAck(ctx, protocol.BuildEvenHubShutdown(magic), magic, 1500*time.Millisecond)
		c.nativeCreated = false
		c.nativeShape = nativeShapeNone
	}
	c.nativeMu.Unlock()

	c.logPacket("TX", ble.Right, protocol.EvenHubPrelude)
	if err := c.transport.Write(ctx, ble.Right, protocol.EvenHubPrelude); err != nil {
		return err
	}
	if err := sleepContext(ctx, c.nativePreludeDelay); err != nil {
		return err
	}
	payload, err := protocol.BuildEvenHubCreateImages(tiles, 201)
	if err != nil {
		return err
	}
	response, err := c.sendEvenHubAck(ctx, payload, 201, c.evenHubAckTimeout)
	if err != nil {
		return err
	}
	if response.Result != nil && *response.Result%2 != 0 {
		return fmt.Errorf("%w: image create result=%d", ErrEvenHubRejected, *response.Result)
	}
	c.imagePrimed = true
	return sleepContext(ctx, c.nativeCreateDelay)
}

func (c *Client) streamWarmup(ctx context.Context, tile protocol.ImageTile) error {
	session := c.nextImageSession()
	for offset, index := 0, 0; offset < len(tile.BMP); index++ {
		end := min(offset+imageDataChunk, len(tile.BMP))
		magic := c.nextEvenHubMagic()
		payload, err := protocol.BuildEvenHubImageFragment(tile.ID, tile.Name, session, len(tile.BMP), index, tile.BMP[offset:end], magic)
		if err != nil {
			return err
		}
		response, err := c.sendEvenHubAck(ctx, payload, magic, 10*time.Second)
		if err != nil || response.Result == nil {
			if err != nil {
				return err
			}
			return ErrEvenHubRejected
		}
		offset = end
	}
	return nil
}

func (c *Client) streamTiles(ctx context.Context, tiles []protocol.ImageTile) error {
	inFlight := make([]<-chan evenHubAck, 0, imageAckWindow)
	consecutiveMisses := 0
	aborted := false
	drain := func() error {
		ack := <-inFlight[0]
		inFlight = inFlight[1:]
		if ack.err == nil && ack.response.Result != nil && *ack.response.Result == 4 {
			consecutiveMisses = 0
			return nil
		}
		consecutiveMisses++
		if consecutiveMisses > 3 {
			aborted = true
			return ack.err
		}
		return nil
	}

sendLoop:
	for _, tile := range tiles {
		if err := c.sendImageHeartbeat(ctx); err != nil {
			return err
		}
		session := c.nextImageSession()
		for offset, index := 0, 0; offset < len(tile.BMP); index++ {
			for len(inFlight) >= imageAckWindow {
				_ = drain()
				if aborted {
					break sendLoop
				}
			}
			end := min(offset+imageDataChunk, len(tile.BMP))
			magic := c.nextEvenHubMagic()
			payload, err := protocol.BuildEvenHubImageFragment(tile.ID, tile.Name, session, len(tile.BMP), index, tile.BMP[offset:end], magic)
			if err != nil {
				return err
			}
			waiter, err := c.fireEvenHub(ctx, payload, magic, time.Second)
			if err != nil {
				return err
			}
			inFlight = append(inFlight, waiter)
			offset = end
		}
	}
	for len(inFlight) > 0 {
		_ = drain()
	}
	if aborted {
		return fmt.Errorf("image stream wedged after consecutive acknowledgement failures")
	}
	return nil
}

func (c *Client) nextImageSession() int {
	step := 1
	if c.imageSessionJump {
		step = 2
		c.imageSessionJump = false
	}
	c.imageSession += step
	if c.imageSession >= 250 {
		c.imageSession = 2
	}
	return c.imageSession
}

func (c *Client) sendImageHeartbeat(ctx context.Context) error {
	magic := c.nextEvenHubMagic()
	c.evenHubMu.Lock()
	sequence := c.evenHubSequence
	c.evenHubSequence++
	c.evenHubMu.Unlock()
	frames, err := protocol.FrameEvenHub(sequence, protocol.EvenHubServiceID, protocol.EvenHubRequest, protocol.BuildEvenHubHeartbeat(magic), c.evenHubChunkSize)
	if err != nil {
		return err
	}
	c.evenHubWriteMu.Lock()
	defer c.evenHubWriteMu.Unlock()
	for _, frame := range frames {
		if err := c.transport.Write(ctx, ble.Right, frame); err != nil {
			return err
		}
	}
	return nil
}

func errorsJoin(first, second error) error {
	return fmt.Errorf("%v; retry failed: %w", first, second)
}
