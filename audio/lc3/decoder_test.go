package lc3

import "testing"

func TestDecoderRejectsInvalidFrameSize(t *testing.T) {
	decoder, err := NewDecoder()
	if err != nil {
		if err == ErrUnsupported {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	defer decoder.Close()
	if _, err := decoder.DecodeFrame(make([]byte, FrameBytes-1)); err == nil {
		t.Fatal("expected invalid frame size error")
	}
}

func TestDecoderDecodesFrame(t *testing.T) {
	decoder, err := NewDecoder()
	if err != nil {
		if err == ErrUnsupported {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	defer decoder.Close()
	pcm, err := decoder.DecodeFrame(make([]byte, FrameBytes))
	if err != nil {
		t.Fatal(err)
	}
	if len(pcm) != FrameSamples {
		t.Fatalf("PCM samples = %d, want %d", len(pcm), FrameSamples)
	}
}
