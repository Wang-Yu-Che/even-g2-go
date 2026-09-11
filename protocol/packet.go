package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	PacketMagic0 byte = 0xAA
	PacketMagic1 byte = 0x21

	SingleFragmentCount byte = 0x01
	SingleFragmentIndex byte = 0x01

	packetHeaderLength = 8
	packetCRCLength    = 2
)

var (
	ErrPacketTooShort       = errors.New("packet too short")
	ErrInvalidPacketMagic   = errors.New("invalid packet magic")
	ErrInvalidPacketLength  = errors.New("invalid packet length")
	ErrUnsupportedFragments = errors.New("unsupported packet fragments")
	ErrInvalidPacketCRC     = errors.New("invalid packet CRC")
)

// Packet is a decoded single-fragment G2 control packet.
type Packet struct {
	Sequence  byte
	ServiceHi byte
	ServiceLo byte
	Payload   []byte
}

// BuildPacket creates a single-fragment G2 control packet.
//
// The CRC covers the payload only and is appended little-endian. Source:
// OpenEvenSdk/PROTOCOL.md section 4a and G2Protocol.swift buildPacket.
func BuildPacket(sequence, serviceHi, serviceLo byte, payload []byte) []byte {
	packet := make([]byte, packetHeaderLength+len(payload)+packetCRCLength)
	packet[0] = PacketMagic0
	packet[1] = PacketMagic1
	packet[2] = sequence
	packet[3] = byte(len(payload) + packetCRCLength)
	packet[4] = SingleFragmentCount
	packet[5] = SingleFragmentIndex
	packet[6] = serviceHi
	packet[7] = serviceLo
	copy(packet[packetHeaderLength:], payload)

	crc := CRC16CCITT(payload)
	binary.LittleEndian.PutUint16(packet[len(packet)-packetCRCLength:], crc)

	return packet
}

// ParsePacket validates and decodes a single-fragment G2 control packet.
func ParsePacket(data []byte) (Packet, error) {
	if len(data) < packetHeaderLength+packetCRCLength {
		return Packet{}, ErrPacketTooShort
	}
	if data[0] != PacketMagic0 || data[1] != PacketMagic1 {
		return Packet{}, ErrInvalidPacketMagic
	}
	if data[4] != SingleFragmentCount || data[5] != SingleFragmentIndex {
		return Packet{}, ErrUnsupportedFragments
	}

	if data[3] != byte(len(data)-packetHeaderLength) {
		return Packet{}, ErrInvalidPacketLength
	}

	payloadEnd := len(data) - packetCRCLength
	payload := data[packetHeaderLength:payloadEnd]
	wantCRC := binary.LittleEndian.Uint16(data[payloadEnd:])
	if gotCRC := CRC16CCITT(payload); gotCRC != wantCRC {
		return Packet{}, fmt.Errorf("%w: got %04X, want %04X", ErrInvalidPacketCRC, wantCRC, gotCRC)
	}

	return Packet{
		Sequence:  data[2],
		ServiceHi: data[6],
		ServiceLo: data[7],
		Payload:   append([]byte(nil), payload...),
	}, nil
}
