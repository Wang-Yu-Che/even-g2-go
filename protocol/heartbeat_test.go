package protocol

import (
	"bytes"
	"testing"
)

func TestBuildHeartbeatGolden(t *testing.T) {
	want := []byte{
		0xAA, 0x21, 0xC0, 0x04, 0x01, 0x01, 0x80, 0x00,
		0x08, 0x25, 0x61, 0xE0,
	}
	got := BuildHeartbeat(0xC0)
	if !bytes.Equal(got, want) {
		t.Fatalf("BuildHeartbeat() = % X, want % X", got, want)
	}
}
