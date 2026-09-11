package g2

import (
	"testing"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

func TestStreamTilesAcceptsImageOK(t *testing.T) {
	transport := &recordingTransport{}
	client := testClient(transport)
	client.evenHubChunkSize = 180
	transport.writeHook = func(arm ble.Arm, frame []byte) {
		if arm != ble.Right || len(frame) < 14 || frame[6] != protocol.EvenHubServiceID {
			return
		}
		payload := frame[8 : len(frame)-2]
		if len(payload) < 4 || payload[0] != 0x08 || payload[1] != 0x03 || payload[2] != 0x10 {
			return
		}
		magic := payload[3]
		ack := []byte{0x08, 0x04, 0x10, magic, 0x1A, 0x02, 0x08, 0x04}
		client.HandleNotification(ble.Right, protocol.BuildPacket(1, protocol.EvenHubServiceID, 0, ack))
	}

	tiles := []protocol.ImageTile{{ID: 10, Name: "t0", BMP: []byte{1, 2, 3}}}
	if err := client.streamTiles(t.Context(), tiles); err != nil {
		t.Fatalf("streamTiles() error = %v", err)
	}
}

func TestImageSessionWraps(t *testing.T) {
	client := testClient(&recordingTransport{})
	client.imageSession = 249
	if got := client.nextImageSession(); got != 2 {
		t.Fatalf("session = %d, want 2", got)
	}
}

func TestImageSessionJumpsByTwoAfterWedge(t *testing.T) {
	client := testClient(&recordingTransport{})
	client.imageSession = 7
	client.imageSessionJump = true
	if got := client.nextImageSession(); got != 9 {
		t.Fatalf("session = %d, want 9", got)
	}
}
