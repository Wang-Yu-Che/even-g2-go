package g2

import (
	"context"
	"testing"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

type fileRecordingTransport struct {
	recordingTransport
	fileNotifications chan []byte
	fileServices      []byte
}

func newFileRecordingTransport() *fileRecordingTransport {
	return &fileRecordingTransport{fileNotifications: make(chan []byte, 8)}
}

func (t *fileRecordingTransport) SubscribeFile(context.Context, ble.Arm) (<-chan []byte, error) {
	return t.fileNotifications, nil
}

func (t *fileRecordingTransport) WriteFile(_ context.Context, arm ble.Arm, frame []byte) error {
	t.record(arm, frame)
	service := frame[6]
	t.fileServices = append(t.fileServices, service)
	command := byte(protocol.FileCommandData)
	if service == protocol.FileCommandServiceID {
		command = frame[8]
	}
	payload := []byte{command, 0}
	crc := protocol.CRC16CCITT(payload)
	t.fileNotifications <- []byte{0xAA, 0x12, 1, 4, 1, 1, service, 0, command, 0, byte(crc), byte(crc >> 8)}
	return nil
}

func (t *fileRecordingTransport) record(arm ble.Arm, data []byte) {
	t.mu.Lock()
	t.writes = append(t.writes, recordedWrite{arm: arm, data: append([]byte(nil), data...)})
	t.mu.Unlock()
}

func TestTransferFileRunsAllThreePhases(t *testing.T) {
	transport := newFileRecordingTransport()
	client := testClient(transport)
	client.fileTimeout = client.evenHubAckTimeout
	client.AttachFileNotifications(t.Context(), transport.fileNotifications)
	status, err := client.TransferFile(t.Context(), 1, "n", []byte{7, 8})
	if err != nil || status != 0 {
		t.Fatalf("TransferFile() status=%d error=%v", status, err)
	}
	want := []byte{protocol.FileCommandServiceID, protocol.FileCommandServiceID, protocol.FileDataServiceID, protocol.FileCommandServiceID}
	if len(transport.fileServices) != len(want) {
		t.Fatalf("file services = % X", transport.fileServices)
	}
	for index := range want {
		if transport.fileServices[index] != want[index] {
			t.Fatalf("file services = % X, want % X", transport.fileServices, want)
		}
	}
}
