package protocol

import "time"

func BuildOnboardingFinish(magic int) []byte {
	payload := protoUint(1, 1)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(3, protoUint(1, 4))...)
}

func BuildGestureControlInit(magic int) []byte {
	payload := protoUintPresent(1, 0)
	return append(payload, protoUint(2, magic)...)
}

func BuildDeviceTimeSync(magic int, instant time.Time) []byte {
	_, offset := instant.Zone()
	localSeconds := instant.Unix() + int64(offset)
	payload := protoUint(1, 128)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(128, protoUint64(1, uint64(localSeconds)))...)
}

func BuildRingConnection(magic int, connected bool, mac []byte, name string) []byte {
	ring := protoUintPresent(1, boolInt(connected))
	ring = append(ring, protoBytes(2, mac)...)
	ring = append(ring, protoString(3, name)...)
	payload := protoUint(1, 6)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(5, ring)...)
}

func protoUint64(field int, value uint64) []byte {
	if value == 0 {
		return nil
	}
	encoded := EncodeVarint(uint64(field << 3))
	return append(encoded, EncodeVarint(value)...)
}
