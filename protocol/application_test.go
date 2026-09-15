package protocol

import (
	"errors"
	"slices"
	"testing"
)

func TestApplicationPacketRoundTrip(t *testing.T) {
	payload := []byte{0x08, 0x07, 0x10, 0x01}
	data, err := BuildApplicationPacket(0x23, ServiceDashboard, payload)
	if err != nil {
		t.Fatal(err)
	}
	if data[0] != 0xAA || data[1] != 0x12 || data[6] != 0x08 || data[7] != 0x00 {
		t.Fatalf("packet header = % X", data[:8])
	}
	packet, err := ParseApplicationPacket(data)
	if err != nil {
		t.Fatal(err)
	}
	if packet.Sequence != 0x23 || packet.ServiceID != ServiceDashboard || !slices.Equal(packet.Payload, payload) {
		t.Fatalf("packet = %+v", packet)
	}
}

func TestApplicationPacketRejectsInvalidData(t *testing.T) {
	data, err := BuildApplicationPacket(1, ServiceDeviceInfo, []byte{0x08, 0x02})
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-1] ^= 0xFF
	if _, err := ParseApplicationPacket(data); !errors.Is(err, ErrInvalidPacketCRC) {
		t.Fatalf("error = %v, want ErrInvalidPacketCRC", err)
	}
	data[1] = PacketMagic1
	if _, err := ParseApplicationPacket(data); !errors.Is(err, ErrNotApplicationPacket) {
		t.Fatalf("error = %v, want ErrNotApplicationPacket", err)
	}
}

func TestApplicationServiceName(t *testing.T) {
	if got := ApplicationServiceName(ServiceNavigation); got != "navigation" {
		t.Fatalf("name = %q", got)
	}
	if got := ApplicationServiceName(0xFFFF); got != "unknown" {
		t.Fatalf("unknown name = %q", got)
	}
}
