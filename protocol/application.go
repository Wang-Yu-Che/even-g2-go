package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const ApplicationPacketVersion byte = 0x12

// These service IDs were observed in G2 BLE captures documented by Even-G2-RE.
const (
	ServiceDefault        uint16 = 0x0001
	ServiceDashboard      uint16 = 0x0008
	ServiceConfiguration  uint16 = 0x000C
	ServiceTranslation    uint16 = 0x000E
	ServiceSystem         uint16 = 0x0080
	ServiceSyncInfo       uint16 = 0x0108
	ServiceDeviceInfo     uint16 = 0x0109
	ServiceDeviceSettings uint16 = 0x010D
	ServiceNavigation     uint16 = 0x0180
)

var ErrNotApplicationPacket = errors.New("not an AA 12 application packet")

// ApplicationPacket is a decoded single-fragment AA 12 application packet.
// ServiceID is encoded little-endian on the wire.
type ApplicationPacket struct {
	Sequence  byte
	ServiceID uint16
	Payload   []byte
}

// BuildApplicationPacket builds a single-fragment AA 12 application packet.
func BuildApplicationPacket(sequence byte, serviceID uint16, payload []byte) ([]byte, error) {
	if len(payload)+packetCRCLength > 255 {
		return nil, fmt.Errorf("application payload too large: %d", len(payload))
	}
	packet := make([]byte, packetHeaderLength+len(payload)+packetCRCLength)
	packet[0] = PacketMagic0
	packet[1] = ApplicationPacketVersion
	packet[2] = sequence
	packet[3] = byte(len(payload) + packetCRCLength)
	packet[4] = SingleFragmentCount
	packet[5] = SingleFragmentIndex
	binary.LittleEndian.PutUint16(packet[6:8], serviceID)
	copy(packet[packetHeaderLength:], payload)
	binary.LittleEndian.PutUint16(packet[len(packet)-packetCRCLength:], CRC16CCITT(payload))
	return packet, nil
}

// ParseApplicationPacket validates and decodes a single-fragment AA 12 packet.
func ParseApplicationPacket(data []byte) (ApplicationPacket, error) {
	if len(data) < packetHeaderLength+packetCRCLength {
		return ApplicationPacket{}, ErrPacketTooShort
	}
	if data[0] != PacketMagic0 || data[1] != ApplicationPacketVersion {
		return ApplicationPacket{}, ErrNotApplicationPacket
	}
	if data[4] != SingleFragmentCount || data[5] != SingleFragmentIndex {
		return ApplicationPacket{}, ErrUnsupportedFragments
	}
	if int(data[3])+packetHeaderLength != len(data) {
		return ApplicationPacket{}, ErrInvalidPacketLength
	}
	payloadEnd := len(data) - packetCRCLength
	payload := data[packetHeaderLength:payloadEnd]
	wantCRC := binary.LittleEndian.Uint16(data[payloadEnd:])
	if gotCRC := CRC16CCITT(payload); gotCRC != wantCRC {
		return ApplicationPacket{}, fmt.Errorf("%w: got %04X, want %04X", ErrInvalidPacketCRC, wantCRC, gotCRC)
	}
	return ApplicationPacket{
		Sequence:  data[2],
		ServiceID: binary.LittleEndian.Uint16(data[6:8]),
		Payload:   append([]byte(nil), payload...),
	}, nil
}

// ApplicationServiceName returns a stable name for known, capture-confirmed services.
func ApplicationServiceName(serviceID uint16) string {
	switch serviceID {
	case ServiceDefault:
		return "default"
	case ServiceDashboard:
		return "dashboard"
	case ServiceConfiguration:
		return "configuration"
	case ServiceTranslation:
		return "translation"
	case ServiceSystem:
		return "system"
	case ServiceSyncInfo:
		return "sync-info"
	case ServiceDeviceInfo:
		return "device-info"
	case ServiceDeviceSettings:
		return "device-settings"
	case ServiceNavigation:
		return "navigation"
	default:
		return "unknown"
	}
}
