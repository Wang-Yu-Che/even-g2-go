package protocol

// BuildHeartbeat builds the text-mode keep-alive packet sent to both arms.
// Source: OpenEvenSdk/PROTOCOL.md section 7 and G2Protocol.swift heartbeat.
func BuildHeartbeat(sequence byte) []byte {
	return BuildPacket(sequence, 0x80, 0x00, []byte{0x08, 0x25})
}
