package g2

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

var ErrEvenHubAckTimeout = errors.New("EvenHub acknowledgement timed out")

type evenHubAck struct {
	response protocol.EvenHubResponse
	err      error
}

// EventHandlers receives semantic native-container input callbacks.
type EventHandlers struct {
	OnListTap   func(name string, index int, itemName string)
	OnTextTap   func(name string)
	OnPageEvent func(name string, eventType int)
	OnDoubleTap func()
}

// SetEventHandlers replaces the native-container callback set.
func (c *Client) SetEventHandlers(handlers EventHandlers) {
	c.eventHandlerMu.Lock()
	c.eventHandlers = handlers
	c.eventHandlerMu.Unlock()
}

// Events returns decoded native-container input events.
func (c *Client) Events() <-chan protocol.EvenHubEvent {
	return c.events
}

// AttachNotifications routes one arm's notification stream through the client.
func (c *Client) AttachNotifications(ctx context.Context, arm ble.Arm, notifications <-chan []byte) {
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
			case packet, ok := <-notifications:
				if !ok {
					select {
					case c.disconnects <- struct{}{}:
					default:
					}
					return
				}
				c.HandleNotification(arm, packet)
			}
		}
	}()
}

func (c *Client) stopNotifications() {
	c.notificationMu.Lock()
	stops := c.notificationStops
	c.notificationStops = nil
	c.notificationMu.Unlock()
	for _, stop := range stops {
		stop()
	}
}

// HandleNotification parses a single G2 notification and routes EvenHub data.
func (c *Client) HandleNotification(arm ble.Arm, data []byte) {
	c.logPacket("RX", arm, data)
	// G2 replies use AA 12 while requests use AA 21. OpenEvenSdk therefore
	// checks only the first magic byte and the EvenHub service ID on RX.
	if len(data) < 9 || data[0] != protocol.PacketMagic0 || data[6] != protocol.EvenHubServiceID {
		return
	}
	payload := data[8:]
	if len(payload) >= 2 {
		body := payload[:len(payload)-2]
		crc := uint16(payload[len(payload)-2]) | uint16(payload[len(payload)-1])<<8
		if protocol.CRC16CCITT(body) == crc {
			payload = body
		}
	}
	if data[7] == 0x01 || data[7] == 0x06 {
		event, parseErr := protocol.ParseEvenHubEvent(payload)
		if parseErr != nil || event.Kind == protocol.EvenHubEventOther {
			c.log("[EVENHUB] ignored event: %v\n", parseErr)
			return
		}
		if event.Kind == protocol.EvenHubEventSystem && (event.Type == protocol.EvenHubEventForegroundExit || event.Type == protocol.EvenHubEventAbnormalExit || event.Type == protocol.EvenHubEventSystemExit) {
			c.nativeMu.Lock()
			c.nativeCreated = false
			c.nativeShape = nativeShapeNone
			c.nativeMu.Unlock()
			c.evenHubMu.Lock()
			c.evenHubActive = false
			c.evenHubMu.Unlock()
		}
		select {
		case c.events <- event:
		default:
		}
		c.dispatchEvenHubEvent(event)
		return
	}

	response, parseErr := protocol.ParseEvenHubResponse(payload)
	if parseErr != nil {
		c.log("[EVENHUB] ignored acknowledgement: %v\n", parseErr)
		return
	}
	c.log("[EVENHUB] acknowledgement command=%d magic=%d result=%v\n", response.Command, response.Magic, response.Result)
	c.evenHubMu.Lock()
	waiter := c.pendingAcks[response.Magic]
	if waiter != nil {
		delete(c.pendingAcks, response.Magic)
	}
	c.evenHubMu.Unlock()
	if waiter != nil {
		waiter <- evenHubAck{response: response}
	}
}

func (c *Client) dispatchEvenHubEvent(event protocol.EvenHubEvent) {
	now := time.Now()
	c.eventHandlerMu.Lock()
	handlers := c.eventHandlers
	if event.Type == protocol.EvenHubEventDoubleClick && (event.Kind == protocol.EvenHubEventList || event.Kind == protocol.EvenHubEventText || event.Kind == protocol.EvenHubEventSystem) {
		if now.Sub(c.lastBackAt) < 450*time.Millisecond {
			c.eventHandlerMu.Unlock()
			return
		}
		c.lastBackAt = now
		c.eventHandlerMu.Unlock()
		if handlers.OnDoubleTap != nil {
			handlers.OnDoubleTap()
		}
		return
	}

	if (event.Kind == protocol.EvenHubEventList || event.Kind == protocol.EvenHubEventText) && event.Type == protocol.EvenHubEventClick {
		if now.Sub(c.lastTapAt) < 350*time.Millisecond {
			c.eventHandlerMu.Unlock()
			return
		}
		c.lastTapAt = now
	}
	c.eventHandlerMu.Unlock()

	switch event.Kind {
	case protocol.EvenHubEventList:
		if event.Type == protocol.EvenHubEventClick && handlers.OnListTap != nil {
			handlers.OnListTap(event.Name, event.ItemIndex, event.ItemName)
		} else if event.Type != 0 && handlers.OnPageEvent != nil {
			handlers.OnPageEvent(event.Name, event.Type)
		}
	case protocol.EvenHubEventText:
		if event.Type == protocol.EvenHubEventClick && handlers.OnTextTap != nil {
			handlers.OnTextTap(event.Name)
		} else if event.Type != 0 && handlers.OnPageEvent != nil {
			handlers.OnPageEvent(event.Name, event.Type)
		}
	case protocol.EvenHubEventSystem:
		if handlers.OnPageEvent != nil {
			handlers.OnPageEvent("", event.Type)
		}
	case protocol.EvenHubEventPrivate:
		if handlers.OnPageEvent != nil {
			handlers.OnPageEvent(event.Name, event.EventData)
		}
	}
}

func (c *Client) sendEvenHubAck(ctx context.Context, payload []byte, magic int, timeout time.Duration) (protocol.EvenHubResponse, error) {
	waiter, err := c.fireEvenHub(ctx, payload, magic, timeout)
	if err != nil {
		return protocol.EvenHubResponse{}, err
	}
	select {
	case ack := <-waiter:
		return ack.response, ack.err
	case <-ctx.Done():
		c.removePendingAck(magic)
		return protocol.EvenHubResponse{}, ctx.Err()
	}
}

func (c *Client) fireEvenHub(ctx context.Context, payload []byte, magic int, timeout time.Duration) (<-chan evenHubAck, error) {
	waiter := make(chan evenHubAck, 1)
	c.evenHubMu.Lock()
	if _, exists := c.pendingAcks[magic]; exists {
		c.evenHubMu.Unlock()
		return nil, fmt.Errorf("EvenHub magic %d is already pending", magic)
	}
	c.pendingAcks[magic] = waiter
	sequence := c.evenHubSequence
	c.evenHubSequence++
	c.evenHubMu.Unlock()

	frames, err := protocol.FrameEvenHub(sequence, protocol.EvenHubServiceID, protocol.EvenHubRequest, payload, c.evenHubChunkSize)
	if err == nil {
		c.evenHubWriteMu.Lock()
		for _, frame := range frames {
			c.logPacket("TX", ble.Right, frame)
			if err = c.transport.Write(ctx, ble.Right, frame); err != nil {
				break
			}
		}
		c.evenHubWriteMu.Unlock()
	}
	if err != nil {
		c.removePendingAck(magic)
		return nil, err
	}
	go func() {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		var waitErr error
		select {
		case <-ctx.Done():
			waitErr = ctx.Err()
		case <-timer.C:
			waitErr = fmt.Errorf("%w: magic=%d", ErrEvenHubAckTimeout, magic)
		}
		c.evenHubMu.Lock()
		if c.pendingAcks[magic] == waiter {
			delete(c.pendingAcks, magic)
			waiter <- evenHubAck{err: waitErr}
		}
		c.evenHubMu.Unlock()
	}()
	return waiter, nil
}

func (c *Client) nextEvenHubMagic() int {
	c.evenHubMu.Lock()
	defer c.evenHubMu.Unlock()
	magic := c.evenHubMagic
	c.evenHubMagic++
	if c.evenHubMagic > 120 {
		c.evenHubMagic = 1
	}
	return magic
}

func (c *Client) removePendingAck(magic int) {
	c.evenHubMu.Lock()
	delete(c.pendingAcks, magic)
	c.evenHubMu.Unlock()
}

func (c *Client) failPendingAcks(err error) {
	c.evenHubMu.Lock()
	waiters := c.pendingAcks
	c.pendingAcks = make(map[int]chan evenHubAck)
	c.evenHubMu.Unlock()
	for _, waiter := range waiters {
		waiter <- evenHubAck{err: err}
	}
}
