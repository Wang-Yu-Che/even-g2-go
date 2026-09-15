package g2

import (
	"testing"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

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
