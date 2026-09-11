package protocol

import (
	"bytes"
	"errors"
	"testing"
)

func TestBuildPacketGolden(t *testing.T) {
	// OpenEvenSdk authentication step 1, including its payload CRC.
	payload := []byte{0x08, 0x04, 0x10, 0x0C, 0x1A, 0x04, 0x08, 0x01, 0x10, 0x04}
	want := []byte{
		0xAA, 0x21, 0x01, 0x0C, 0x01, 0x01, 0x80, 0x00,
		0x08, 0x04, 0x10, 0x0C, 0x1A, 0x04, 0x08, 0x01, 0x10, 0x04,
		0xC6, 0xBC,
	}

	got := BuildPacket(0x01, 0x80, 0x00, payload)
	if !bytes.Equal(got, want) {
		t.Fatalf("BuildPacket() = % X, want % X", got, want)
	}
}

func TestParsePacket(t *testing.T) {
	wire := BuildPacket(0xC0, 0x80, 0x00, []byte{0x08, 0x25})

	got, err := ParsePacket(wire)
	if err != nil {
		t.Fatalf("ParsePacket() error = %v", err)
	}
	if got.Sequence != 0xC0 || got.ServiceHi != 0x80 || got.ServiceLo != 0x00 {
		t.Fatalf("ParsePacket() header = %+v", got)
	}
	if !bytes.Equal(got.Payload, []byte{0x08, 0x25}) {
		t.Fatalf("ParsePacket() payload = % X", got.Payload)
	}
}

func TestParsePacketRejectsInvalidCRC(t *testing.T) {
	wire := BuildPacket(0x01, 0x80, 0x00, []byte{0x08, 0x04})
	wire[len(wire)-1] ^= 0xFF

	_, err := ParsePacket(wire)
	if !errors.Is(err, ErrInvalidPacketCRC) {
		t.Fatalf("ParsePacket() error = %v, want ErrInvalidPacketCRC", err)
	}
}

func TestBuildPacketPreservesLargePayload(t *testing.T) {
	payload := make([]byte, 260)
	wire := BuildPacket(0x01, 0x06, 0x20, payload)
	if wire[3] != byte(len(payload)+packetCRCLength) {
		t.Fatalf("length byte = %02X, want %02X", wire[3], byte(len(payload)+packetCRCLength))
	}
	if _, err := ParsePacket(wire); err != nil {
		t.Fatalf("ParsePacket() error = %v", err)
	}
}
