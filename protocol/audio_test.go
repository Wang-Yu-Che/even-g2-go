package protocol

import (
	"bytes"
	"testing"
)

func TestBuildEvenHubAudioControl(t *testing.T) {
	want := []byte{0x08, 0x0F, 0x10, 0x07, 0x92, 0x01, 0x02, 0x08, 0x01}
	if got := BuildEvenHubAudioControl(true, 7); !bytes.Equal(got, want) {
		t.Fatalf("audio control = % X, want % X", got, want)
	}
	if got := BuildEvenHubAudioControl(false, 7); !bytes.Equal(got, []byte{0x08, 0x0F, 0x10, 0x07, 0x92, 0x01, 0x00}) {
		t.Fatalf("audio stop = % X", got)
	}
}

func TestParseEvenHubAudioResponse(t *testing.T) {
	response, err := ParseEvenHubAudioResponse([]byte{0x08, 0x10, 0x10, 0x07, 0x9A, 0x01, 0x02, 0x08, 0x01})
	if err != nil {
		t.Fatal(err)
	}
	if response.Command != 16 || response.Magic != 7 || response.Status != 1 {
		t.Fatalf("response = %#v", response)
	}
}

func TestParseG2AudioPacket(t *testing.T) {
	data := make([]byte, G2AudioPacketBytes)
	for i := range data {
		data[i] = byte(i)
	}
	packet, err := ParseG2AudioPacket(data)
	if err != nil {
		t.Fatal(err)
	}
	if packet.Frames[0][0] != 0 || packet.Frames[4][39] != 199 {
		t.Fatalf("unexpected LC3 frame boundaries: first=%d last=%d", packet.Frames[0][0], packet.Frames[4][39])
	}
	if packet.Trailer != [5]byte{200, 201, 202, 203, 204} || packet.Counter != 204 {
		t.Fatalf("unexpected trailer=%v counter=%d", packet.Trailer, packet.Counter)
	}
	data[0] = 99
	if packet.Frames[0][0] != 0 {
		t.Fatal("parsed packet aliases input")
	}
}

func TestParseG2AudioPacketRejectsUnexpectedSize(t *testing.T) {
	if _, err := ParseG2AudioPacket(make([]byte, G2AudioPacketBytes-1)); err == nil {
		t.Fatal("expected invalid packet size error")
	}
}
