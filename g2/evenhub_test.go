package g2

import (
	"errors"
	"testing"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

func TestEvenHubAckIsMatchedByMagic(t *testing.T) {
	transport := &recordingTransport{}
	client := testClient(transport)
	client.evenHubChunkSize = 180

	type result struct {
		response protocol.EvenHubResponse
		err      error
	}
	done := make(chan result, 1)
	go func() {
		response, err := client.sendEvenHubAck(t.Context(), protocol.BuildEvenHubHeartbeat(42), 42, time.Second)
		done <- result{response: response, err: err}
	}()
	waitForWrites(t, transport, 1)

	ackPayload := []byte{0x08, 0x0D, 0x10, 0x2A, 0x1A, 0x02, 0x08, 0x0C}
	client.HandleNotification(ble.Right, protocol.BuildPacket(0x01, protocol.EvenHubServiceID, protocol.EvenHubRequest, ackPayload))
	got := <-done
	if got.err != nil || got.response.Magic != 42 || got.response.Result == nil || *got.response.Result != 12 {
		t.Fatalf("ack = %#v, error = %v", got.response, got.err)
	}
}

func TestEvenHubAcceptsRealG2CreateAckHeader(t *testing.T) {
	client := testClient(&recordingTransport{})
	waiter := make(chan evenHubAck, 1)
	client.pendingAcks[201] = waiter

	client.HandleNotification(ble.Right, []byte{
		0xAA, 0x12, 0x86, 0x09, 0x01, 0x01, 0xE0, 0x00,
		0x08, 0x01, 0x10, 0xC9, 0x01, 0x22, 0x00, 0xFE, 0x34,
	})

	select {
	case ack := <-waiter:
		if ack.response.Command != 1 || ack.response.Magic != 201 || ack.response.Result != nil {
			t.Fatalf("ack = %#v", ack.response)
		}
	case <-time.After(time.Second):
		t.Fatal("real G2 create acknowledgement was not routed")
	}
}

func TestEvenHubAckTimeout(t *testing.T) {
	client := testClient(&recordingTransport{})
	_, err := client.sendEvenHubAck(t.Context(), protocol.BuildEvenHubHeartbeat(1), 1, time.Millisecond)
	if !errors.Is(err, ErrEvenHubAckTimeout) {
		t.Fatalf("error = %v, want ErrEvenHubAckTimeout", err)
	}
}

func TestEvenHubEventIsPublished(t *testing.T) {
	client := testClient(&recordingTransport{})
	events := client.SubscribeEvents(t.Context())
	payload := []byte{0x08, 0x02, 0x6A, 0x09, 0x12, 0x07, 0x12, 0x03, 'h', 'u', 'd', 0x18, 0x03}
	client.HandleNotification(ble.Right, protocol.BuildPacket(0x02, protocol.EvenHubServiceID, 0x01, payload))

	select {
	case event := <-events:
		if event.Kind != protocol.EvenHubEventText || event.Name != "hud" || event.Type != 3 {
			t.Fatalf("event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestEvenHubEventIsBroadcastToSubscribers(t *testing.T) {
	client := testClient(&recordingTransport{})
	first := client.SubscribeEvents(t.Context())
	second := client.SubscribeEvents(t.Context())
	event := protocol.EvenHubEvent{Kind: protocol.EvenHubEventText, Name: "hud", Type: 3}
	client.publishEvent(event)
	for index, events := range []<-chan protocol.EvenHubEvent{first, second} {
		select {
		case got := <-events:
			if got != event {
				t.Fatalf("subscriber %d event = %#v", index, got)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d timed out", index)
		}
	}
}

func TestEvenHubAcceptsEitherArmNotification(t *testing.T) {
	client := testClient(&recordingTransport{})
	events := client.SubscribeEvents(t.Context())
	payload := []byte{0x08, 0x02, 0x6A, 0x09, 0x12, 0x07, 0x12, 0x03, 'h', 'u', 'd', 0x18, 0x03}
	packet := protocol.BuildPacket(0x02, protocol.EvenHubServiceID, 0x01, payload)
	packet[3] = 0 // Firmware RX headers are not required to mirror TX length semantics.
	client.HandleNotification(ble.Left, packet)
	select {
	case event := <-events:
		if event.Kind != protocol.EvenHubEventText {
			t.Fatalf("event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestEvenHubCallbacksDebounceDuplicateTap(t *testing.T) {
	client := testClient(&recordingTransport{})
	count := 0
	client.OnEvent(func(protocol.EvenHubEvent) { count++ })
	event := protocol.EvenHubEvent{Kind: protocol.EvenHubEventList, Name: "hud", ItemName: "one"}
	client.dispatchEvenHubEvent(event)
	client.dispatchEvenHubEvent(event)
	if count != 1 {
		t.Fatalf("callback count = %d, want 1", count)
	}
}
