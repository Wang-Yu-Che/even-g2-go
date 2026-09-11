package g2

import (
	"testing"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
)

func TestDisplayTextSendsReferenceFlow(t *testing.T) {
	transport := &recordingTransport{}
	client := testClient(transport)
	configureFastDisplay(client)
	if err := client.Authenticate(t.Context()); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if err := client.DisplayText(t.Context(), "Hello G2"); err != nil {
		t.Fatalf("DisplayText() error = %v", err)
	}

	flow := transport.writesSnapshot()[14:]
	if len(flow) != 36 {
		t.Fatalf("display write count = %d, want 36", len(flow))
	}
	expectedSequences := []byte{
		0x08, 0x09,
		0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F, 0x10, 0x11, 0x12, 0x13,
		0x14, 0x15, 0x16, 0x17, 0x18, 0x19,
	}
	for index, sequence := range expectedSequences {
		left := flow[index*2]
		right := flow[index*2+1]
		if left.arm != ble.Left || right.arm != ble.Right {
			t.Fatalf("logical packet %d arm order = %s/%s", index, left.arm, right.arm)
		}
		if left.data[2] != sequence || right.data[2] != sequence {
			t.Fatalf("logical packet %d sequence = %02X/%02X, want %02X", index, left.data[2], right.data[2], sequence)
		}
	}
	if flow[0].data[6] != 0x0E || flow[0].data[7] != 0x20 {
		t.Fatalf("display config service = %02X %02X", flow[0].data[6], flow[0].data[7])
	}
	if flow[30].data[6] != 0x80 || flow[30].data[7] != 0x00 {
		t.Fatalf("sync service = %02X %02X", flow[30].data[6], flow[30].data[7])
	}
}

func TestDisplayRefreshStopsOnClose(t *testing.T) {
	transport := &recordingTransport{}
	client := testClient(transport)
	configureFastDisplay(client)
	client.displayRefreshInterval = 5 * time.Millisecond
	if err := client.Authenticate(t.Context()); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if err := client.DisplayText(t.Context(), "Hello G2"); err != nil {
		t.Fatalf("DisplayText() error = %v", err)
	}
	waitForWrites(t, transport, 14+36+36)
	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	count := len(transport.writesSnapshot())
	time.Sleep(2 * client.displayRefreshInterval)
	if got := len(transport.writesSnapshot()); got != count {
		t.Fatalf("display refresh continued after Close: got %d writes, want %d", got, count)
	}
}

func configureFastDisplay(client *Client) {
	client.displayRefreshInterval = 0
	client.displayConfigDelay = 0
	client.displayInitDelay = 0
	client.displayFirstPageDelay = 0
	client.displayMarkerDelay = 0
	client.displayLatePageDelay = 0
	client.displaySyncDelay = 0
}
