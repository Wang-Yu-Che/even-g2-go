package protocol

import (
	"encoding/json"
	"time"
)

const (
	NotificationCommandControl        = 1
	NotificationCommandIOS            = 2
	NotificationCommandWhitelist      = 3
	NotificationCommandWhitelistCheck = 4
	NotificationCommandResponse       = 161
)

type NotificationConfig struct {
	Enabled         bool
	AutoDisplay     bool
	DurationSeconds int
	DoNotDisturb    bool
}

type NotificationResponse struct {
	Command       int
	Magic         int
	FailedCommand int
	ErrorCode     int
}

func ParseNotificationResponse(payload []byte) (NotificationResponse, error) {
	response := NotificationResponse{Command: -1, Magic: -1, FailedCommand: -1, ErrorCode: -1}
	err := walkProto(payload, func(field, wire int, value uint64, data []byte) error {
		switch {
		case wire == 0 && field == 1:
			response.Command = int(value)
		case wire == 0 && field == 2:
			response.Magic = int(value)
		case wire == 2 && field == 5:
			return walkProto(data, func(nestedField, nestedWire int, nestedValue uint64, _ []byte) error {
				if nestedWire == 0 && nestedField == 1 {
					response.FailedCommand = int(nestedValue)
				} else if nestedWire == 0 && nestedField == 2 {
					response.ErrorCode = int(nestedValue)
				}
				return nil
			})
		}
		return nil
	})
	return response, err
}

type PhoneNotification struct {
	ID          int
	Action      int
	PackageName string
	Title       string
	Subtitle    string
	Message     string
	Timestamp   time.Time
	DisplayName string
}

func BuildNotificationControl(magic int, config NotificationConfig) []byte {
	control := protoUintPresent(1, boolInt(config.Enabled))
	control = append(control, protoUintPresent(2, boolInt(config.AutoDisplay))...)
	control = append(control, protoUintPresent(3, config.DurationSeconds)...)
	control = append(control, protoUintPresent(5, boolInt(config.DoNotDisturb))...)
	payload := protoUint(1, NotificationCommandControl)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(3, control)...)
}

func BuildNotificationWhitelistControl(magic int, disabled bool) []byte {
	payload := protoUint(1, NotificationCommandWhitelist)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(6, protoUint(1, boolInt(disabled)))...)
}

func BuildNotificationJSON(notification PhoneNotification) ([]byte, error) {
	timestamp := notification.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	body := map[string]any{
		"msg_id": notification.ID, "action": notification.Action,
		"app_identifier": notification.PackageName, "title": notification.Title,
		"subtitle": notification.Subtitle, "message": notification.Message,
		"time_s": timestamp.Unix(), "date": timestamp.Format("20060102T150405"),
		"display_name": notification.DisplayName,
	}
	return json.Marshal(map[string]any{"android_notification": body})
}
