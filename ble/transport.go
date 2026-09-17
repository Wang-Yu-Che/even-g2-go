package ble

import (
	"context"
	"errors"
)

var (
	ErrLeftArmNotFound        = errors.New("left arm not found")
	ErrRightArmNotFound       = errors.New("right arm not found")
	ErrServiceNotFound        = errors.New("G2 service not found")
	ErrCharacteristicNotFound = errors.New("G2 characteristic not found")
	ErrDisconnected           = errors.New("G2 arm disconnected")
)

// Transport separates the G2 protocol from a platform BLE implementation.
type Transport interface {
	Scan(ctx context.Context, report func(ScanResult)) error
	Connect(ctx context.Context, arm Arm, device ScanResult) error
	Write(ctx context.Context, arm Arm, data []byte) error
	Subscribe(ctx context.Context, arm Arm) (<-chan []byte, error)
	Close() error
}

// AudioTransport optionally exposes the G2 microphone notification channel.
type AudioTransport interface {
	SubscribeAudio(ctx context.Context, arm Arm) (<-chan []byte, error)
}

// FileTransport exposes the G2 file-service characteristic used by native notifications.
type FileTransport interface {
	WriteFile(ctx context.Context, arm Arm, data []byte) error
	SubscribeFile(ctx context.Context, arm Arm) (<-chan []byte, error)
}
