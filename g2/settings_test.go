package g2

import (
	"testing"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

func TestRequestDeviceSettingsRoutesResponse(t *testing.T) {
	transport := &recordingTransport{}
	client := testClient(transport)
	client.setState(Ready)
	client.evenHubAckTimeout = time.Second
	done := make(chan DeviceSettings, 1)
	errs := make(chan error, 1)
	go func() {
		settings, err := client.RequestDeviceSettings(t.Context())
		if err != nil {
			errs <- err
			return
		}
		done <- settings
	}()
	waitForWrites(t, transport, 2)
	// command=2, magic=1, field4={leftVersion="2.2.9.22", battery=87, charging=1}
	payload := []byte{0x08, 0x02, 0x10, 0x01, 0x22, 0x0E, 0x2A, 0x08, '2', '.', '2', '.', '9', '.', '2', '2', 0x60, 0x57, 0x68, 0x01}
	client.HandleNotification(ble.Right, protocol.BuildPacket(1, protocol.G2SettingsServiceID, protocol.EvenHubRequest, payload))
	select {
	case err := <-errs:
		t.Fatal(err)
	case settings := <-done:
		if settings.BatteryPercent != 87 || !settings.Charging || settings.LeftFirmwareVersion != "2.2.9.22" {
			t.Fatalf("settings = %#v", settings)
		}
	case <-time.After(time.Second):
		t.Fatal("settings query timed out")
	}
}

func TestSetBrightnessWaitsForMatchingResponse(t *testing.T) {
	transport := &recordingTransport{}
	client := testClient(transport)
	client.setState(Ready)
	done := make(chan error, 1)
	go func() { done <- client.SetBrightness(t.Context(), BrightnessOptions{Level: 60, Auto: true}) }()
	waitForWrites(t, transport, 2)
	client.HandleNotification(ble.Left, protocol.BuildPacket(1, protocol.G2SettingsServiceID, protocol.EvenHubRequest, []byte{0x08, 0x01, 0x10, 0x01}))
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
