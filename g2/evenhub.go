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

// EventHandler receives one decoded native-container event.
type EventHandler func(protocol.EvenHubEvent)

// OnEvent registers an event handler and returns its unsubscribe function.
func (c *Client) OnEvent(handler EventHandler) func() {
	if handler == nil {
		return func() {}
	}
	c.eventHandlerMu.Lock()
	id := c.nextHandlerID
	c.nextHandlerID++
	c.eventHandlers[id] = handler
	c.eventHandlerMu.Unlock()
	return func() {
		c.eventHandlerMu.Lock()
		delete(c.eventHandlers, id)
		c.eventHandlerMu.Unlock()
	}
}

// SubscribeEvents returns an independent event stream tied to ctx.
func (c *Client) SubscribeEvents(ctx context.Context) <-chan protocol.EvenHubEvent {
	events := make(chan protocol.EvenHubEvent, 32)
	c.subscriberMu.Lock()
	id := c.nextSubscriberID
	c.nextSubscriberID++
	c.eventSubscribers[id] = events
	c.subscriberMu.Unlock()
	go func() {
		<-ctx.Done()
		c.subscriberMu.Lock()
		delete(c.eventSubscribers, id)
		close(events)
		c.subscriberMu.Unlock()
	}()
	return events
}

// SubscribeNavigation returns compass and calibration events tied to ctx.
func (c *Client) SubscribeNavigation(ctx context.Context) <-chan protocol.NavigationEvent {
	events := make(chan protocol.NavigationEvent, 32)
	c.subscriberMu.Lock()
	id := c.nextSubscriberID
	c.nextSubscriberID++
	c.navigationSubscribers[id] = events
	c.subscriberMu.Unlock()
	go func() {
		<-ctx.Done()
		c.subscriberMu.Lock()
		delete(c.navigationSubscribers, id)
		close(events)
		c.subscriberMu.Unlock()
	}()
	return events
}

// SubscribeNotificationResponses returns notification control responses tied to ctx.
func (c *Client) SubscribeNotificationResponses(ctx context.Context) <-chan protocol.NotificationResponse {
	events := make(chan protocol.NotificationResponse, 16)
	c.subscriberMu.Lock()
	id := c.nextSubscriberID
	c.nextSubscriberID++
	c.notificationSubscribers[id] = events
	c.subscriberMu.Unlock()
	go func() {
		<-ctx.Done()
		c.subscriberMu.Lock()
		delete(c.notificationSubscribers, id)
		close(events)
		c.subscriberMu.Unlock()
	}()
	return events
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
	if len(data) < 9 || data[0] != protocol.PacketMagic0 {
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
	if data[6] == protocol.NavigationServiceID {
		event, parseErr := protocol.ParseNavigationEvent(payload)
		if parseErr != nil {
			c.log("[NAVIGATION] ignored event: %v\n", parseErr)
			return
		}
		c.publishNavigation(event)
		return
	}
	if data[6] == protocol.NotificationServiceID {
		response, parseErr := protocol.ParseNotificationResponse(payload)
		if parseErr != nil {
			c.log("[NOTIFICATION] ignored response: %v\n", parseErr)
			return
		}
		c.publishNotificationResponse(response)
		return
	}
	if data[6] == protocol.G2SettingsServiceID {
		magic, parseErr := protocol.ParseSettingsMagic(payload)
		if parseErr != nil || magic < 0 {
			return
		}
		c.evenHubMu.Lock()
		waiter := c.pendingSettings[magic]
		if waiter != nil {
			delete(c.pendingSettings, magic)
		}
		c.evenHubMu.Unlock()
		if waiter != nil {
			waiter <- settingsReply{payload: append([]byte(nil), payload...)}
		}
		return
	}
	if data[6] != protocol.EvenHubServiceID {
		return
	}
	event, eventErr := protocol.ParseEvenHubEvent(payload)
	if eventErr == nil && event.Kind != protocol.EvenHubEventOther {
		if event.Kind == protocol.EvenHubEventMenu {
			c.menuMu.Lock()
			if event.AppID == c.lastMenuAppID && time.Since(c.lastMenuAt) < 500*time.Millisecond {
				c.menuMu.Unlock()
				return
			}
			c.lastMenuAppID = event.AppID
			c.lastMenuAt = time.Now()
			event.PackageName = c.menuAppIDs[event.AppID]
			c.menuMu.Unlock()
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
		c.publishEvent(event)
		c.dispatchEvenHubEvent(event)
		return
	}
	if eventErr != nil {
		c.log("[EVENHUB] event parse failed: %v\n", eventErr)
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
	handlers := make([]EventHandler, 0, len(c.eventHandlers))
	for _, handler := range c.eventHandlers {
		handlers = append(handlers, handler)
	}
	if event.Type == protocol.EvenHubEventDoubleClick && (event.Kind == protocol.EvenHubEventList || event.Kind == protocol.EvenHubEventText || event.Kind == protocol.EvenHubEventSystem) {
		if now.Sub(c.lastBackAt) < 450*time.Millisecond {
			c.eventHandlerMu.Unlock()
			return
		}
		c.lastBackAt = now
		c.eventHandlerMu.Unlock()
		for _, handler := range handlers {
			handler(event)
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

	for _, handler := range handlers {
		handler(event)
	}
}

func (c *Client) publishEvent(event protocol.EvenHubEvent) {
	c.subscriberMu.Lock()
	defer c.subscriberMu.Unlock()
	for _, events := range c.eventSubscribers {
		select {
		case events <- event:
		default:
		}
	}
}

func (c *Client) publishNavigation(event protocol.NavigationEvent) {
	c.subscriberMu.Lock()
	defer c.subscriberMu.Unlock()
	for _, events := range c.navigationSubscribers {
		select {
		case events <- event:
		default:
		}
	}
}

func (c *Client) publishNotificationResponse(response protocol.NotificationResponse) {
	c.subscriberMu.Lock()
	defer c.subscriberMu.Unlock()
	for _, events := range c.notificationSubscribers {
		select {
		case events <- response:
		default:
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
	settingsWaiters := c.pendingSettings
	c.pendingSettings = make(map[int]chan settingsReply)
	c.evenHubMu.Unlock()
	for _, waiter := range waiters {
		waiter <- evenHubAck{err: err}
	}
	for _, waiter := range settingsWaiters {
		waiter <- settingsReply{err: err}
	}
}
