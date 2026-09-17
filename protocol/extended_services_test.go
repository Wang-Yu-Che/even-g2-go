package protocol

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestNavigationBuildersAndParser(t *testing.T) {
	if got := BuildNavigationStart(7); !bytes.Equal(got, []byte{0x08, 0x05, 0x10, 0x07}) {
		t.Fatalf("navigation start = % X", got)
	}
	event, err := ParseNavigationEvent([]byte{0x08, 0x0F, 0x52, 0x03, 0x08, 0x87, 0x02})
	if err != nil || event.Command != NavigationEventCompassChanged || event.Heading != 263 {
		t.Fatalf("navigation event = %#v, error = %v", event, err)
	}
}

func TestEvenAIAndDeviceBuilders(t *testing.T) {
	if got := BuildEvenAIConfig(9, false); !bytes.Equal(got, []byte{0x08, 0x0A, 0x10, 0x09, 0x6A, 0x02, 0x10, 0x20}) {
		t.Fatalf("Even AI config = % X", got)
	}
	if got := BuildOnboardingFinish(3); !bytes.Equal(got, []byte{0x08, 0x01, 0x10, 0x03, 0x1A, 0x02, 0x08, 0x04}) {
		t.Fatalf("onboarding finish = % X", got)
	}
	if got := BuildRingConnection(4, true, []byte{1, 2, 3, 4, 5, 6}, "R1"); !bytes.Contains(got, []byte{0x12, 0x06, 1, 2, 3, 4, 5, 6}) {
		t.Fatalf("ring connection = % X", got)
	}
}

func TestIMUControlAndReport(t *testing.T) {
	wantControl := []byte{0x08, 0x13, 0x10, 0x07, 0xA2, 0x01, 0x05, 0x08, 0x01, 0x10, 0xF4, 0x03}
	if got := BuildEvenHubIMUControl(true, 500, 7); !bytes.Equal(got, wantControl) {
		t.Fatalf("IMU control = % X, want % X", got, wantControl)
	}
	imu := make([]byte, 0, 15)
	for field, value := range []float32{1.25, -2.5, 0.5} {
		imu = append(imu, byte((field+1)<<3|5))
		bits := math.Float32bits(value)
		imu = append(imu, byte(bits), byte(bits>>8), byte(bits>>16), byte(bits>>24))
	}
	system := append([]byte{0x08, 0x08, 0x1A, byte(len(imu))}, imu...)
	device := append([]byte{0x1A, byte(len(system))}, system...)
	payload := append([]byte{0x08, 0x02, 0x6A, byte(len(device))}, device...)
	event, err := ParseEvenHubEvent(payload)
	if err != nil || event.Type != EvenHubEventIMUDataReport || event.IMUX != 1.25 || event.IMUY != -2.5 || event.IMUZ != 0.5 {
		t.Fatalf("IMU event = %#v, error = %v", event, err)
	}
}

func TestFileAndNotificationProtocol(t *testing.T) {
	start, err := BuildFileStart(1, NotificationFilePath, []byte{7, 8})
	if err != nil {
		t.Fatal(err)
	}
	if len(start) != 93 || !bytes.Equal(start[:13], []byte{0, 1, 0, 0, 0, 2, 0, 0, 0, 0xD6, 0x80, 0x18, 0x09}) {
		t.Fatalf("file start = % X", start[:13])
	}
	if got := FileCRC32([]byte("123456789")); got != 0xC052A8C8 {
		t.Fatalf("file CRC32 = %08X", got)
	}

	payload, err := BuildNotificationJSON(PhoneNotification{ID: 2000, PackageName: "chat", Title: "Hi", Timestamp: time.Date(2026, 9, 17, 12, 30, 0, 0, time.FixedZone("CST", 8*3600))})
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]map[string]any
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["android_notification"]["date"] != "20260917T123000" || envelope["android_notification"]["time_s"] != float64(1789619400) {
		t.Fatalf("notification JSON = %s", payload)
	}
}

func TestNotificationControlsMatchMentraGoldens(t *testing.T) {
	disabled := BuildNotificationControl(11, NotificationConfig{AutoDisplay: true, DurationSeconds: 5})
	if !bytes.Equal(disabled, []byte{8, 1, 16, 11, 26, 8, 8, 0, 16, 1, 24, 5, 40, 0}) {
		t.Fatalf("disabled control = % X", disabled)
	}
	enabled := BuildNotificationControl(11, NotificationConfig{Enabled: true, AutoDisplay: true, DurationSeconds: 5})
	if !bytes.Equal(enabled, []byte{8, 1, 16, 11, 26, 8, 8, 1, 16, 1, 24, 5, 40, 0}) {
		t.Fatalf("enabled control = % X", enabled)
	}
}

func TestParseNotificationRejection(t *testing.T) {
	response, err := ParseNotificationResponse([]byte{0x08, 0xA1, 0x01, 0x10, 0x0B, 0x2A, 0x04, 0x08, 0x01, 0x10, 0x08})
	if err != nil || response.Command != NotificationCommandResponse || response.Magic != 11 || response.FailedCommand != NotificationCommandControl || response.ErrorCode != 8 {
		t.Fatalf("notification response = %#v, error = %v", response, err)
	}
}
