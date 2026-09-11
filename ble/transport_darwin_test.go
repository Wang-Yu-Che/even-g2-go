//go:build darwin

package ble

import (
	"errors"
	"testing"
)

func TestSameUUID(t *testing.T) {
	if !sameUUID("00002760-08c2-11e1-9073-0e8ac72e5401", writeUUID) {
		t.Fatal("sameUUID() rejected equivalent UUID casing")
	}
	if sameUUID(writeUUID, notifyUUID) {
		t.Fatal("sameUUID() accepted different UUIDs")
	}
}

func TestConnectionRequiresDiscoveredArm(t *testing.T) {
	transport := &DarwinTransport{
		discovered:  make(map[Arm]ScanResult),
		connections: make(map[Arm]*armConnection),
	}

	if err := transport.Connect(t.Context(), Left); !errors.Is(err, ErrLeftArmNotFound) {
		t.Fatalf("Connect(Left) error = %v, want ErrLeftArmNotFound", err)
	}
	if err := transport.Connect(t.Context(), Right); !errors.Is(err, ErrRightArmNotFound) {
		t.Fatalf("Connect(Right) error = %v, want ErrRightArmNotFound", err)
	}
}

func TestNotificationStreamClosesOnce(t *testing.T) {
	connection := &armConnection{notifications: make(chan []byte, 1)}
	connection.deliver([]byte{0xAA, 0x21})
	if err := connection.close(false); err != nil {
		t.Fatalf("close() error = %v", err)
	}
	if err := connection.close(false); err != nil {
		t.Fatalf("second close() error = %v", err)
	}
	connection.deliver([]byte{0x01})

	if got := <-connection.notifications; len(got) != 2 {
		t.Fatalf("notification = % X, want AA 21", got)
	}
	if _, ok := <-connection.notifications; ok {
		t.Fatal("notification channel remained open")
	}
}
