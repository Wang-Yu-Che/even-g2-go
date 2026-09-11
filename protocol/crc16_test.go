package protocol

import "testing"

func TestCRC16CCITT(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want uint16
	}{
		{name: "empty", data: nil, want: 0xFFFF},
		{name: "standard check value", data: []byte("123456789"), want: 0x29B1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CRC16CCITT(tt.data); got != tt.want {
				t.Fatalf("CRC16CCITT() = %04X, want %04X", got, tt.want)
			}
		})
	}
}
