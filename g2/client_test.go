package g2

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

func TestAuthenticateWritesSevenPacketsToBothArms(t *testing.T) {
	transport := &recordingTransport{}
	client := testClient(transport)

	if err := client.Authenticate(t.Context()); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if client.Left.State() != Ready || client.Right.State() != Ready {
		t.Fatalf("states = %s/%s, want Ready/Ready", client.Left.State(), client.Right.State())
	}
	writes := transport.writesSnapshot()
	if len(writes) != 14 {
		t.Fatalf("write count = %d, want 14", len(writes))
	}

	wantPackets := protocol.BuildAuthPackets(time.Unix(1700000000, 0))
	for index, want := range wantPackets {
		left := writes[index*2]
		right := writes[index*2+1]
		if left.arm != ble.Left || right.arm != ble.Right {
			t.Fatalf("packet %d arm order = %s/%s", index+1, left.arm, right.arm)
		}
		if !bytes.Equal(left.data, want) || !bytes.Equal(right.data, want) {
			t.Fatalf("packet %d bytes differ from protocol builder", index+1)
		}
	}
}

func TestAuthenticateClosesTransportOnWriteFailure(t *testing.T) {
	writeErr := errors.New("write failed")
	transport := &recordingTransport{writeErr: writeErr}
	client := testClient(transport)

	err := client.Authenticate(t.Context())
	if !errors.Is(err, ErrAuthenticationFailed) {
		t.Fatalf("Authenticate() error = %v, want ErrAuthenticationFailed", err)
	}
	if !transport.isClosed() {
		t.Fatal("transport was not closed")
	}
	if client.Left.State() != Disconnected || client.Right.State() != Disconnected {
		t.Fatalf("states = %s/%s, want Disconnected/Disconnected", client.Left.State(), client.Right.State())
	}
}

func testClient(transport ble.Transport) *Client {
	client := NewClient(transport, false, io.Discard)
	client.now = func() time.Time { return time.Unix(1700000000, 0) }
	client.leftToRightGap = 0
	client.afterRightGap = 0
	client.packetGap = 0
	client.authWait = 0
	client.heartbeatInterval = 0
	client.nativePreludeDelay = 0
	client.nativeCreateDelay = 0
	client.evenHubAckTimeout = time.Second
	return client
}

func TestHeartbeatLifecycle(t *testing.T) {
	transport := &recordingTransport{}
	client := testClient(transport)
	client.heartbeatInterval = 5 * time.Millisecond

	if err := client.Authenticate(t.Context()); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	waitForWrites(t, transport, 18)
	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	writes := transport.writesSnapshot()
	for index, sequence := range []byte{0xC0, 0xC0, 0xC1, 0xC1} {
		write := writes[14+index]
		wantArm := ble.Left
		if index%2 == 1 {
			wantArm = ble.Right
		}
		if write.arm != wantArm || write.data[2] != sequence {
			t.Fatalf("heartbeat write %d = %s/%02X, want %s/%02X", index, write.arm, write.data[2], wantArm, sequence)
		}
	}

	countAfterClose := len(writes)
	time.Sleep(2 * client.heartbeatInterval)
	if got := len(transport.writesSnapshot()); got != countAfterClose {
		t.Fatalf("writes continued after Close: got %d, want %d", got, countAfterClose)
	}
}

func TestHeartbeatSequenceWrapsToC0(t *testing.T) {
	transport := &recordingTransport{}
	client := testClient(transport)
	client.heartbeatSequence = 0xFF

	if err := client.sendHeartbeat(t.Context()); err != nil {
		t.Fatalf("sendHeartbeat() error = %v", err)
	}
	if client.heartbeatSequence != 0xC0 {
		t.Fatalf("next heartbeat sequence = %02X, want C0", client.heartbeatSequence)
	}
}

func TestParentContextCancellationClosesTransport(t *testing.T) {
	transport := &recordingTransport{}
	client := testClient(transport)
	client.heartbeatInterval = time.Second
	ctx, cancel := context.WithCancel(t.Context())

	if err := client.Authenticate(ctx); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	cancel()
	waitForClosed(t, transport)
	client.stopHeartbeat()
	if client.Left.State() != Disconnected || client.Right.State() != Disconnected {
		t.Fatalf("states = %s/%s, want Disconnected/Disconnected", client.Left.State(), client.Right.State())
	}
}

func waitForWrites(t *testing.T, transport *recordingTransport, count int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for len(transport.writesSnapshot()) < count {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d writes", count)
		}
		time.Sleep(time.Millisecond)
	}
}

func waitForClosed(t *testing.T, transport *recordingTransport) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !transport.isClosed() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for transport close")
		}
		time.Sleep(time.Millisecond)
	}
}

type recordedWrite struct {
	arm  ble.Arm
	data []byte
}

type recordingTransport struct {
	mu          sync.Mutex
	writes      []recordedWrite
	writeErr    error
	closed      bool
	writeHook   func(ble.Arm, []byte)
	scanResults []ble.ScanResult
}

func (t *recordingTransport) Scan(_ context.Context, report func(ble.ScanResult)) error {
	for _, result := range t.scanResults {
		report(result)
	}
	return nil
}
func (t *recordingTransport) Connect(context.Context, ble.Arm) error { return nil }
func (t *recordingTransport) Subscribe(context.Context, ble.Arm) (<-chan []byte, error) {
	return nil, nil
}
func (t *recordingTransport) Write(_ context.Context, arm ble.Arm, data []byte) error {
	t.mu.Lock()
	if t.writeErr != nil {
		t.mu.Unlock()
		return t.writeErr
	}
	copyData := append([]byte(nil), data...)
	t.writes = append(t.writes, recordedWrite{arm: arm, data: copyData})
	hook := t.writeHook
	t.mu.Unlock()
	if hook != nil {
		hook(arm, copyData)
	}
	return nil
}
func (t *recordingTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	return nil
}

func (t *recordingTransport) writesSnapshot() []recordedWrite {
	t.mu.Lock()
	defer t.mu.Unlock()
	writes := make([]recordedWrite, len(t.writes))
	copy(writes, t.writes)
	return writes
}

func (t *recordingTransport) isClosed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closed
}
