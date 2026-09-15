package protocol

import "fmt"

const G2SettingsServiceID byte = 0x09

// DeviceSettingsSnapshot is the basic-settings response sent by stock G2 firmware.
type DeviceSettingsSnapshot struct {
	BatteryPercent       int    `json:"batteryPercent"`
	Charging             bool   `json:"charging"`
	LeftFirmwareVersion  string `json:"leftFirmwareVersion"`
	RightFirmwareVersion string `json:"rightFirmwareVersion"`
	Brightness           int    `json:"brightness"`
	AutoBrightness       bool   `json:"autoBrightness"`
	HeadUpEnabled        bool   `json:"headUpEnabled"`
	HeadUpAngle          int    `json:"headUpAngle"`
	WearDetection        bool   `json:"wearDetection"`
	SilentMode           bool   `json:"silentMode"`
	ScreenDepth          int    `json:"screenDepth"`
	ScreenHeight         int    `json:"screenHeight"`
	DeviceRunningStatus  int    `json:"deviceRunningStatus"`
}

func settingsEnvelope(command, magic, field int, message []byte) []byte {
	payload := protoUint(1, command)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(field, message)...)
}

// BuildSettingsQuery requests battery, firmware and persisted display settings.
func BuildSettingsQuery(magic int) []byte {
	return settingsEnvelope(2, magic, 4, protoUint(1, 1))
}

// BuildSetBrightness sets manual brightness (0..100) and ambient auto adjustment.
func BuildSetBrightness(magic, level int, automatic bool) ([]byte, error) {
	if level < 0 || level > 100 {
		return nil, fmt.Errorf("brightness must be between 0 and 100")
	}
	leaf := protoUint(1, boolInt(automatic))
	leaf = append(leaf, protoUint(2, level)...)
	return settingsEnvelope(1, magic, 3, protoMessage(1, leaf)), nil
}

func BuildSetHeadUpSwitch(magic int, enabled bool) []byte {
	return settingsEnvelope(1, magic, 3, protoMessage(4, protoUint(1, boolInt(enabled))))
}

func BuildSetHeadUpAngle(magic, angle int) ([]byte, error) {
	if angle < 0 || angle > 60 {
		return nil, fmt.Errorf("head-up angle must be between 0 and 60")
	}
	return settingsEnvelope(1, magic, 3, protoMessage(4, protoUint(2, angle))), nil
}

func BuildSetScreenHeight(magic, level int) ([]byte, error) {
	if level < 0 || level > 12 {
		return nil, fmt.Errorf("screen height must be between 0 and 12")
	}
	return settingsEnvelope(1, magic, 3, protoMessage(2, protoUint(1, level))), nil
}

func BuildSetScreenDepth(magic, level int) ([]byte, error) {
	if level < 0 || level > 2 {
		return nil, fmt.Errorf("screen depth must be between 0 and 2")
	}
	return settingsEnvelope(1, magic, 3, protoMessage(3, protoUint(1, level))), nil
}

// ParseSettingsMagic extracts the request correlation value.
func ParseSettingsMagic(payload []byte) (int, error) {
	magic := -1
	err := walkProto(payload, func(field, wire int, value uint64, _ []byte) error {
		if field == 2 && wire == 0 {
			magic = int(value)
		}
		return nil
	})
	return magic, err
}

// ParseDeviceSettings decodes fields confirmed in the basic-settings snapshot.
func ParseDeviceSettings(payload []byte) (DeviceSettingsSnapshot, error) {
	var inner []byte
	err := walkProto(payload, func(field, wire int, _ uint64, data []byte) error {
		if wire == 2 && (field == 4 || field == 5) {
			inner = data
		}
		return nil
	})
	if err != nil || inner == nil {
		return DeviceSettingsSnapshot{}, ErrInvalidProtobuf
	}
	var snapshot DeviceSettingsSnapshot
	err = walkProto(inner, func(field, wire int, value uint64, data []byte) error {
		switch {
		case wire == 2 && field == 5:
			snapshot.LeftFirmwareVersion = string(data)
		case wire == 2 && field == 6:
			snapshot.RightFirmwareVersion = string(data)
		case wire == 0 && field == 2:
			snapshot.Brightness = int(value)
		case wire == 0 && field == 3:
			snapshot.ScreenHeight = int(value)
		case wire == 0 && field == 4:
			snapshot.ScreenDepth = int(value)
		case wire == 0 && field == 7:
			snapshot.HeadUpEnabled = value != 0
		case wire == 0 && field == 8:
			snapshot.HeadUpAngle = int(value)
		case wire == 0 && field == 10:
			snapshot.WearDetection = value != 0
		case wire == 0 && field == 11:
			snapshot.DeviceRunningStatus = int(value)
		case wire == 0 && field == 12:
			snapshot.BatteryPercent = int(value)
		case wire == 0 && field == 13:
			snapshot.Charging = value != 0
		case wire == 0 && field == 14:
			snapshot.SilentMode = value != 0
		case wire == 0 && field == 18:
			snapshot.AutoBrightness = value != 0
		}
		return nil
	})
	return snapshot, err
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
