package protocol

import (
	"bytes"
	"testing"
)

func TestEncodeVarint(t *testing.T) {
	tests := []struct {
		value uint64
		want  []byte
	}{
		{value: 0, want: []byte{0x00}},
		{value: 127, want: []byte{0x7F}},
		{value: 128, want: []byte{0x80, 0x01}},
		{value: 300, want: []byte{0xAC, 0x02}},
		{value: 1700000000, want: []byte{0x80, 0xE2, 0xCF, 0xAA, 0x06}},
	}

	for _, tt := range tests {
		if got := EncodeVarint(tt.value); !bytes.Equal(got, tt.want) {
			t.Errorf("EncodeVarint(%d) = % X, want % X", tt.value, got, tt.want)
		}
	}
}
