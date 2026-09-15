package g2

import (
	"testing"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
)

func TestScanDevicesPairsMatchingArms(t *testing.T) {
	transport := &recordingTransport{scanResults: []ble.ScanResult{
		{Arm: ble.Right, Name: "Even G2_R_ABCD", Address: "right"},
		{Arm: ble.Left, Name: "Even G2_L_ABCD", Address: "left"},
	}}
	devices, err := scanDevices(t.Context(), transport, ScanOptions{Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 1 || devices[0].Left.Address != "left" || devices[0].Right.Address != "right" {
		t.Fatalf("devices = %#v", devices)
	}
}

func TestScanDevicesRejectsAmbiguousMultipleArms(t *testing.T) {
	transport := &recordingTransport{scanResults: []ble.ScanResult{
		{Arm: ble.Left, Name: "Even G2_L_A", Address: "left-a"},
		{Arm: ble.Left, Name: "Even G2_L_B", Address: "left-b"},
		{Arm: ble.Right, Name: "Even G2_R_B", Address: "right-b"},
	}}
	_, err := scanDevices(t.Context(), transport, ScanOptions{Timeout: time.Second})
	if err != ErrMultipleDevices {
		t.Fatalf("error = %v, want ErrMultipleDevices", err)
	}
}
