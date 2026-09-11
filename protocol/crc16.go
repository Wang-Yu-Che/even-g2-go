package protocol

const (
	crc16Initial    uint16 = 0xFFFF
	crc16Polynomial uint16 = 0x1021
)

// CRC16CCITT calculates CRC-16/CCITT-FALSE.
//
// Source: OpenEvenSdk/PROTOCOL.md section 3 and
// OpenEvenSdk/ios/Sources/G2Bridge/CRC16.swift.
func CRC16CCITT(data []byte) uint16 {
	crc := crc16Initial

	for _, value := range data {
		crc ^= uint16(value) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ crc16Polynomial
				continue
			}
			crc <<= 1
		}
	}

	return crc
}
