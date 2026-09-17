package g2

import (
	"errors"
	"testing"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

func TestShowTextWithIconsRebuildsWithoutShutdown(t *testing.T) {
	transport := &recordingTransport{}
	client := testClient(transport)
	client.setState(Ready)
	client.nativeCreated = true
	client.nativeShape = nativeShapeText
	client.evenHubActive = true
	transport.writeHook = func(arm ble.Arm, frame []byte) {
		if arm != ble.Right || len(frame) < 14 || frame[6] != protocol.EvenHubServiceID {
			return
		}
		payload := frame[8 : len(frame)-2]
		if len(payload) > 1 && payload[0] == 0x08 && payload[1] == 0x09 {
			t.Errorf("ShowTextWithIcons sent shutdown payload: % X", payload)
		}
		if len(payload) < 4 || payload[0] != 0x08 || payload[2] != 0x10 {
			return
		}
		magic := payload[3]
		switch payload[1] {
		case 0x07:
			ack := []byte{0x08, 0x08, 0x10, magic, 0x1A, 0x02, 0x08, 0x06}
			client.HandleNotification(ble.Right, protocol.BuildPacket(1, protocol.EvenHubServiceID, 0, ack))
		case 0x03:
			ack := []byte{0x08, 0x04, 0x10, magic, 0x1A, 0x02, 0x08, 0x04}
			client.HandleNotification(ble.Right, protocol.BuildPacket(1, protocol.EvenHubServiceID, 0, ack))
		}
	}

	icons := []StatusIcon{
		{ID: 2, Name: "terminal", X: 20, Y: 20, Width: 20, Height: 20, BMP: []byte{1}},
		{ID: 3, Name: "state", X: 516, Y: 20, Width: 20, Height: 20, BMP: []byte{2}},
	}
	if err := client.ShowTextWithIcons(t.Context(), "status", "running", TextStyle{Width: 576, Height: 288}, icons); err != nil {
		t.Fatalf("ShowTextWithIcons() error = %v", err)
	}
	if client.nativeIconID != 3 || client.nativeIconName != "state" {
		t.Fatalf("update icon = %d/%q", client.nativeIconID, client.nativeIconName)
	}
}

func TestShowTextWithIconsRestartsHeartbeatAfterFailure(t *testing.T) {
	transport := &recordingTransport{writeErr: errors.New("write failed")}
	client := testClient(transport)
	client.setState(Ready)
	client.nativeCreated = true
	client.heartbeatInterval = time.Millisecond

	err := client.ShowTextWithIcons(t.Context(), "status", "running", TextStyle{Width: 576, Height: 288}, []StatusIcon{
		{ID: 2, Name: "state", Width: 20, Height: 20, BMP: []byte{1}},
	})
	if err == nil {
		t.Fatal("ShowTextWithIcons() succeeded despite transport failure")
	}
	waitForClosed(t, transport)
	client.stopHeartbeat()
}

func TestShowTextPrimesAndRebuildsNativePage(t *testing.T) {
	transport := &recordingTransport{}
	client := testClient(transport)
	client.setState(Ready)

	go func() {
		waitForWrites(t, transport, 2)
		client.HandleNotification(ble.Right, protocol.BuildPacket(1, protocol.EvenHubServiceID, protocol.EvenHubRequest,
			[]byte{0x08, 0x01, 0x10, 0xC9, 0x01, 0x1A, 0x00}))
		waitForWrites(t, transport, 3)
		client.HandleNotification(ble.Right, protocol.BuildPacket(2, protocol.EvenHubServiceID, protocol.EvenHubRequest,
			[]byte{0x08, 0x08, 0x10, 0x01, 0x1A, 0x02, 0x08, 0x06}))
	}()

	if err := client.ShowText(t.Context(), "hud", "full lens"); err != nil {
		t.Fatalf("ShowText() error = %v", err)
	}
	writes := transport.writesSnapshot()
	if len(writes) != 3 || writes[0].arm != ble.Right || writes[1].arm != ble.Right || writes[2].arm != ble.Right {
		t.Fatalf("writes = %#v", writes)
	}
	if client.nativeShape != nativeShapeText || !client.nativeCreated {
		t.Fatalf("native state = shape %d, created %v", client.nativeShape, client.nativeCreated)
	}
}

func TestShowTextWithStylePrimesAndRebuildsNativePage(t *testing.T) {
	transport := &recordingTransport{}
	client := testClient(transport)
	client.setState(Ready)

	go func() {
		waitForWrites(t, transport, 2)
		client.HandleNotification(ble.Right, protocol.BuildPacket(1, protocol.EvenHubServiceID, protocol.EvenHubRequest,
			[]byte{0x08, 0x01, 0x10, 0xC9, 0x01, 0x1A, 0x00}))
		waitForWrites(t, transport, 3)
		client.HandleNotification(ble.Right, protocol.BuildPacket(2, protocol.EvenHubServiceID, protocol.EvenHubRequest,
			[]byte{0x08, 0x08, 0x10, 0x01, 0x1A, 0x02, 0x08, 0x06}))
	}()

	style := TextStyle{X: 20, Y: 20, Width: 536, Height: 248, BorderWidth: 2, BorderColor: 15, BorderRadius: 8, PaddingLength: 12}
	if err := client.ShowTextWithStyle(t.Context(), "notice", "$ Codex\n> testing", style); err != nil {
		t.Fatalf("ShowTextWithStyle() error = %v", err)
	}
	if client.nativeShape != nativeShapeText || !client.nativeCreated {
		t.Fatalf("native state = shape %d, created %v", client.nativeShape, client.nativeCreated)
	}
}

func TestShutdownNativeClosesActivePage(t *testing.T) {
	transport := &recordingTransport{}
	client := testClient(transport)
	client.setState(Ready)
	client.nativeCreated = true
	client.nativeShape = nativeShapeList
	client.evenHubActive = true

	go func() {
		waitForWrites(t, transport, 1)
		client.HandleNotification(ble.Right, protocol.BuildPacket(1, protocol.EvenHubServiceID, protocol.EvenHubRequest,
			[]byte{0x08, 0x0A, 0x10, 0x01, 0x1A, 0x02, 0x08, 0x0A}))
	}()

	if err := client.ShutdownNative(t.Context()); err != nil {
		t.Fatalf("ShutdownNative() error = %v", err)
	}
	if client.nativeCreated || client.nativeShape != nativeShapeNone || client.evenHubActive {
		t.Fatalf("native state = shape %d, created %v, active %v", client.nativeShape, client.nativeCreated, client.evenHubActive)
	}
}

func TestSystemExitClearsNativePage(t *testing.T) {
	client := testClient(&recordingTransport{})
	client.nativeCreated = true
	client.nativeShape = nativeShapeList
	client.evenHubActive = true
	payload := []byte{0x08, 0x02, 0x6A, 0x04, 0x1A, 0x02, 0x08, 0x07}
	client.HandleNotification(ble.Right, protocol.BuildPacket(2, protocol.EvenHubServiceID, 0x01, payload))
	if client.nativeCreated || client.nativeShape != nativeShapeNone || client.evenHubActive {
		t.Fatalf("native page was not cleared")
	}
}
