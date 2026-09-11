package protocol

import (
	"errors"
	"fmt"
)

const (
	EvenHubServiceID = 0xE0
	EvenHubRequest   = 0x20
)

var ErrInvalidProtobuf = errors.New("invalid protobuf payload")

// EvenHubPrelude launches the EvenHub task once per BLE session on the right arm.
var EvenHubPrelude = []byte{
	0xAA, 0x21, 0x92, 0x13, 0x01, 0x01, 0x01, 0x20, 0x08, 0x02, 0x10, 0x9C,
	0x01, 0x22, 0x0A, 0x1A, 0x08, 0x12, 0x06, 0x12, 0x04, 0x08, 0x00, 0x10,
	0x00, 0xA1, 0x42,
}

// EvenHubResponse identifies an acknowledgement by command and correlation ID.
// Result is nil for createOK because proto3 omits its zero value.
type EvenHubResponse struct {
	Command int
	Magic   int
	Result  *int
}

type EvenHubEventKind int

const (
	EvenHubEventOther EvenHubEventKind = iota
	EvenHubEventList
	EvenHubEventText
	EvenHubEventSystem
	EvenHubEventPrivate
)

func (kind EvenHubEventKind) String() string {
	switch kind {
	case EvenHubEventList:
		return "list"
	case EvenHubEventText:
		return "text"
	case EvenHubEventSystem:
		return "system"
	case EvenHubEventPrivate:
		return "private"
	default:
		return "other"
	}
}

const (
	EvenHubEventClick           = 0
	EvenHubEventScrollTop       = 1
	EvenHubEventScrollBottom    = 2
	EvenHubEventDoubleClick     = 3
	EvenHubEventForegroundEnter = 4
	EvenHubEventForegroundExit  = 5
	EvenHubEventAbnormalExit    = 6
	EvenHubEventSystemExit      = 7
)

// EvenHubEventTypeName returns the protocol name of an event type.
func EvenHubEventTypeName(eventType int) string {
	switch eventType {
	case EvenHubEventClick:
		return "click"
	case EvenHubEventScrollTop:
		return "scroll-top"
	case EvenHubEventScrollBottom:
		return "scroll-bottom"
	case EvenHubEventDoubleClick:
		return "double-click"
	case EvenHubEventForegroundEnter:
		return "foreground-enter"
	case EvenHubEventForegroundExit:
		return "foreground-exit"
	case EvenHubEventAbnormalExit:
		return "abnormal-exit"
	case EvenHubEventSystemExit:
		return "system-exit"
	default:
		return fmt.Sprintf("unknown(%d)", eventType)
	}
}

// EvenHubEvent is the normalized form of list, text, system, and private events.
type EvenHubEvent struct {
	Kind       EvenHubEventKind
	Name       string
	ItemName   string
	ItemIndex  int
	Type       int
	ExitReason int
	EventID    int
	EventData  int
}

// BuildEvenHubHeartbeat builds Cmd=12 HeartBeatPacket{Cnt:0}.
func BuildEvenHubHeartbeat(magic int) []byte {
	payload := protoUint(1, 12)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(14, nil)...)
}

// FrameEvenHub splits one protobuf payload into G2 frames. All fragments share
// the same sequence; the CRC of the complete payload is appended to the last.
func FrameEvenHub(sequence, serviceID, flag byte, payload []byte, chunkSize int) ([][]byte, error) {
	if chunkSize <= 0 || chunkSize > 255 {
		return nil, errors.New("EvenHub chunk size must be between 1 and 255")
	}
	total := (len(payload) + packetCRCLength + chunkSize - 1) / chunkSize
	if total == 0 {
		total = 1
	}
	if total > 255 {
		return nil, errors.New("EvenHub payload requires more than 255 fragments")
	}

	crc := CRC16CCITT(payload)
	remaining := append(append([]byte(nil), payload...), byte(crc), byte(crc>>8))
	frames := make([][]byte, 0, total)
	for index := 0; index < total; index++ {
		length := min(chunkSize, len(remaining))
		part := remaining[:length]
		remaining = remaining[length:]
		frame := []byte{PacketMagic0, PacketMagic1, sequence, byte(length), byte(total), byte(index + 1), serviceID, flag}
		frames = append(frames, append(frame, part...))
	}
	return frames, nil
}

// ParseEvenHubResponse decodes an EvenHub acknowledgement protobuf body.
func ParseEvenHubResponse(payload []byte) (EvenHubResponse, error) {
	response := EvenHubResponse{Command: -1, Magic: -1}
	err := walkProto(payload, func(field, wire int, value uint64, data []byte) error {
		switch {
		case wire == 0 && field == 1:
			response.Command = int(value)
		case wire == 0 && field == 2:
			response.Magic = int(value)
		case wire == 2:
			return walkProto(data, func(nestedField, nestedWire int, nestedValue uint64, _ []byte) error {
				if nestedWire == 0 && (nestedField == 1 || nestedField == 8) {
					result := int(nestedValue)
					response.Result = &result
				}
				return nil
			})
		}
		return nil
	})
	return response, err
}

// ParseEvenHubEvent decodes Cmd=2 device events and Cmd=11 private events.
func ParseEvenHubEvent(payload []byte) (EvenHubEvent, error) {
	command := -1
	var device, private []byte
	err := walkProto(payload, func(field, wire int, value uint64, data []byte) error {
		if wire == 0 && field == 1 {
			command = int(value)
		}
		if wire == 2 && field == 13 {
			device = data
		}
		if wire == 2 && field == 16 {
			private = data
		}
		return nil
	})
	if err != nil {
		return EvenHubEvent{}, err
	}
	if command == 2 && device != nil {
		return parseDeviceEvent(device)
	}
	if command == 11 && private != nil {
		return parsePrivateEvent(private)
	}
	return EvenHubEvent{Kind: EvenHubEventOther}, nil
}

func parseDeviceEvent(payload []byte) (EvenHubEvent, error) {
	event := EvenHubEvent{Kind: EvenHubEventOther}
	err := walkProto(payload, func(field, wire int, _ uint64, data []byte) error {
		if wire != 2 || event.Kind != EvenHubEventOther {
			return nil
		}
		switch field {
		case 1:
			event.Kind = EvenHubEventList
		case 2:
			event.Kind = EvenHubEventText
		case 3:
			event.Kind = EvenHubEventSystem
		default:
			return nil
		}
		return parseEventFields(&event, data)
	})
	return event, err
}

func parseEventFields(event *EvenHubEvent, payload []byte) error {
	return walkProto(payload, func(field, wire int, value uint64, data []byte) error {
		switch event.Kind {
		case EvenHubEventList:
			switch {
			case wire == 2 && field == 2:
				event.Name = string(data)
			case wire == 2 && field == 3:
				event.ItemName = string(data)
			case wire == 0 && field == 4:
				event.ItemIndex = int(value)
			case wire == 0 && field == 5:
				event.Type = int(value)
			}
		case EvenHubEventText:
			if wire == 2 && field == 2 {
				event.Name = string(data)
			} else if wire == 0 && field == 3 {
				event.Type = int(value)
			}
		case EvenHubEventSystem:
			if wire == 0 && field == 1 {
				event.Type = int(value)
			} else if wire == 0 && field == 4 {
				event.ExitReason = int(value)
			}
		}
		return nil
	})
}

func parsePrivateEvent(payload []byte) (EvenHubEvent, error) {
	event := EvenHubEvent{Kind: EvenHubEventPrivate}
	err := walkProto(payload, func(field, wire int, value uint64, data []byte) error {
		switch {
		case wire == 2 && field == 2:
			event.Name = string(data)
		case wire == 0 && field == 3:
			event.EventID = int(value)
		case wire == 0 && field == 4:
			event.EventData = int(value)
		}
		return nil
	})
	return event, err
}

func walkProto(payload []byte, visit func(field, wire int, value uint64, data []byte) error) error {
	for offset := 0; offset < len(payload); {
		key, next, ok := decodeProtoVarint(payload, offset)
		if !ok {
			return ErrInvalidProtobuf
		}
		offset = next
		field, wire := int(key>>3), int(key&7)
		switch wire {
		case 0:
			value, end, valid := decodeProtoVarint(payload, offset)
			if !valid {
				return ErrInvalidProtobuf
			}
			offset = end
			if err := visit(field, wire, value, nil); err != nil {
				return err
			}
		case 2:
			length, start, valid := decodeProtoVarint(payload, offset)
			if !valid || length > uint64(len(payload)-start) {
				return ErrInvalidProtobuf
			}
			offset = start + int(length)
			if err := visit(field, wire, 0, payload[start:offset]); err != nil {
				return err
			}
		default:
			return ErrInvalidProtobuf
		}
	}
	return nil
}

func decodeProtoVarint(payload []byte, offset int) (uint64, int, bool) {
	var value uint64
	for shift := uint(0); shift < 64 && offset < len(payload); shift += 7 {
		current := payload[offset]
		offset++
		value |= uint64(current&0x7F) << shift
		if current&0x80 == 0 {
			return value, offset, true
		}
	}
	return 0, offset, false
}

func protoUint(field, value int) []byte {
	if value == 0 {
		return nil
	}
	encoded := EncodeVarint(uint64(field << 3))
	return append(encoded, EncodeVarint(uint64(value))...)
}

func protoMessage(field int, payload []byte) []byte {
	encoded := EncodeVarint(uint64(field<<3 | 2))
	encoded = append(encoded, EncodeVarint(uint64(len(payload)))...)
	return append(encoded, payload...)
}
