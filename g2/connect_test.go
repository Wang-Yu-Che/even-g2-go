package g2

import (
	"testing"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
)

func TestConnectWithTransportCompletesFullFlow(t *testing.T) {
	transport := &recordingTransport{scanResults: []ble.ScanResult{{Arm: ble.Left}, {Arm: ble.Right}}}
	client, err := connectWithTransport(t.Context(), transport, ConnectOptions{})
	if err != nil {
		t.Fatalf("connectWithTransport() error = %v", err)
	}
	defer client.Close()
	if client.Left.State() != Ready || client.Right.State() != Ready {
		t.Fatalf("states = %s/%s", client.Left.State(), client.Right.State())
	}
	if got := len(transport.writesSnapshot()); got < 14 {
		t.Fatalf("writes = %d, want at least 14 authentication writes", got)
	}
}

func TestClosedNotificationSignalsDisconnect(t *testing.T) {
	client := testClient(&recordingTransport{})
	notifications := make(chan []byte)
	close(notifications)
	client.AttachNotifications(t.Context(), ble.Left, notifications)
	select {
	case <-client.disconnects:
	case <-time.After(time.Second):
		t.Fatal("closed notification stream did not signal disconnect")
	}
}

func TestDiscoverBothReportsMissingArm(t *testing.T) {
	transport := &recordingTransport{scanResults: []ble.ScanResult{{Arm: ble.Left}}}
	if err := discoverBoth(t.Context(), transport, 1); err != ble.ErrRightArmNotFound {
		t.Fatalf("error = %v, want ErrRightArmNotFound", err)
	}
}
