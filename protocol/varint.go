package protocol

// EncodeVarint encodes an unsigned integer as a protobuf base-128 varint.
func EncodeVarint(value uint64) []byte {
	encoded := make([]byte, 0, 10)
	for value > 0x7F {
		encoded = append(encoded, byte(value)|0x80)
		value >>= 7
	}
	return append(encoded, byte(value))
}
