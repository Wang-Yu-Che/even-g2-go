package protocol

import (
	"encoding/binary"
	"errors"
)

const (
	FileCommandStart       = 0
	FileCommandData        = 1
	FileCommandResultCheck = 2
	FileTypeNotification   = 1
	NotificationFilePath   = "user/notify_whitelist.json"
)

func FileStatusName(status int) string {
	switch status {
	case 0:
		return "success"
	case 1:
		return "start-error"
	case 2:
		return "data-crc-error"
	case 3:
		return "flash-write-error"
	case 4:
		return "timeout"
	case 5:
		return "no-resources"
	case 6:
		return "result-check-failed"
	case 7:
		return "failed"
	case 8:
		return "cancelled"
	default:
		return "unknown"
	}
}

func BuildFileStart(fileType int, filename string, data []byte) ([]byte, error) {
	name := []byte(filename)
	if len(name) >= 80 {
		return nil, errors.New("file-service filename must be shorter than 80 bytes")
	}
	payload := make([]byte, 93)
	payload[0] = FileCommandStart
	binary.LittleEndian.PutUint32(payload[1:5], uint32(fileType))
	binary.LittleEndian.PutUint32(payload[5:9], uint32(len(data)))
	binary.LittleEndian.PutUint32(payload[9:13], FileCRC32(data))
	copy(payload[13:], name)
	return payload, nil
}

func BuildFileDataCommand() []byte { return []byte{FileCommandData} }

func BuildFileResultCheck() []byte { return []byte{FileCommandResultCheck} }

// FileCRC32 is CRC-32/Castagnoli, MSB-first, zero seed, without final xor.
func FileCRC32(data []byte) uint32 {
	var crc uint32
	for _, current := range data {
		crc ^= uint32(current) << 24
		for range 8 {
			if crc&0x80000000 != 0 {
				crc = crc<<1 ^ 0x1EDC6F41
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func ParseFileAck(frame []byte) (command, status int, err error) {
	if len(frame) != 12 || frame[0] != PacketMagic0 || frame[3] != 4 || frame[4] != 1 || frame[5] != 1 ||
		(frame[6] != FileCommandServiceID && frame[6] != FileDataServiceID) || ((frame[7]>>1)&0x0F) != 0 {
		return 0, 0, ErrInvalidPacketLength
	}
	payload := frame[8:10]
	want := uint16(frame[10]) | uint16(frame[11])<<8
	if CRC16CCITT(payload) != want {
		return 0, 0, ErrInvalidPacketCRC
	}
	return int(payload[0]), int(payload[1]), nil
}
