package g2

import (
	"context"
	"errors"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

var ErrAudioUnsupported = errors.New("G2 microphone notification characteristic unavailable")

// AudioFrame is one raw LC3 notification and its source arm.
type AudioFrame struct {
	Arm  ble.Arm
	Data []byte
}

// SubscribeAudio returns an independent raw LC3 stream tied to ctx.
func (c *Client) SubscribeAudio(ctx context.Context) <-chan AudioFrame {
	frames := make(chan AudioFrame, 64)
	c.subscriberMu.Lock()
	id := c.nextSubscriberID
	c.nextSubscriberID++
	c.audioSubscribers[id] = frames
	c.subscriberMu.Unlock()
	go func() {
		<-ctx.Done()
		c.subscriberMu.Lock()
		delete(c.audioSubscribers, id)
		close(frames)
		c.subscriberMu.Unlock()
	}()
	return frames
}

// AttachAudioNotifications routes a 6402 notification stream to AudioFrames.
func (c *Client) AttachAudioNotifications(ctx context.Context, arm ble.Arm, notifications <-chan []byte) {
	ctx, cancel := context.WithCancel(ctx)
	c.notificationMu.Lock()
	c.notificationStops = append(c.notificationStops, cancel)
	c.notificationMu.Unlock()
	c.notificationWG.Add(1)
	go func() {
		defer c.notificationWG.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case data, ok := <-notifications:
				if !ok {
					return
				}
				frame := AudioFrame{Arm: arm, Data: append([]byte(nil), data...)}
				c.subscriberMu.Lock()
				for _, frames := range c.audioSubscribers {
					select {
					case frames <- frame:
					default:
					}
				}
				c.subscriberMu.Unlock()
			}
		}
	}()
}

// StartMicrophone creates a startup page when needed and enables LC3 capture.
func (c *Client) StartMicrophone(ctx context.Context) error {
	if !c.audioAvailable.Load() {
		return ErrAudioUnsupported
	}
	c.nativeMu.Lock()
	defer c.nativeMu.Unlock()
	if err := c.prepareNative(ctx); err != nil {
		return err
	}
	if err := c.sendAudioControl(ctx, true); err != nil {
		return err
	}
	c.startHeartbeat(ctx)
	return nil
}

// StopMicrophone disables LC3 capture.
func (c *Client) StopMicrophone(ctx context.Context) error {
	if !c.audioAvailable.Load() {
		return ErrAudioUnsupported
	}
	return c.sendAudioControl(ctx, false)
}

func (c *Client) sendAudioControl(ctx context.Context, enabled bool) error {
	magic := c.nextEvenHubMagic()
	c.evenHubMu.Lock()
	sequence := c.evenHubSequence
	c.evenHubSequence++
	c.evenHubMu.Unlock()
	frames, err := protocol.FrameEvenHub(sequence, protocol.EvenHubServiceID, protocol.EvenHubRequest, protocol.BuildEvenHubAudioControl(enabled, magic), c.evenHubChunkSize)
	if err != nil {
		return err
	}
	c.evenHubWriteMu.Lock()
	defer c.evenHubWriteMu.Unlock()
	for _, arm := range []ble.Arm{ble.Right, ble.Left} {
		for _, frame := range frames {
			c.logPacket("TX", arm, frame)
			if err := c.transport.Write(ctx, arm, frame); err != nil {
				return err
			}
		}
	}
	return nil
}
