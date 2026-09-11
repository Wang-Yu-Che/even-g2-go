package audio

import (
	"encoding/binary"
	"io"
	"testing"
)

type memoryWriteSeeker struct {
	data   []byte
	offset int64
}

func (m *memoryWriteSeeker) Write(data []byte) (int, error) {
	end := int(m.offset) + len(data)
	if end > len(m.data) {
		m.data = append(m.data, make([]byte, end-len(m.data))...)
	}
	copy(m.data[m.offset:], data)
	m.offset = int64(end)
	return len(data), nil
}

func (m *memoryWriteSeeker) Seek(offset int64, whence int) (int64, error) {
	if whence != io.SeekStart {
		panic("test only supports SeekStart")
	}
	m.offset = offset
	return offset, nil
}

func TestWAVWriter(t *testing.T) {
	output := &memoryWriteSeeker{}
	writer, err := NewWAVWriter(output, 16000)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.WritePCM([]int16{-1, 0, 1}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if string(output.data[:4]) != "RIFF" || string(output.data[8:12]) != "WAVE" {
		t.Fatalf("invalid WAV header: %q", output.data[:12])
	}
	if got := binary.LittleEndian.Uint32(output.data[40:44]); got != 6 {
		t.Fatalf("data size = %d, want 6", got)
	}
	if got := binary.LittleEndian.Uint16(output.data[44:46]); got != 0xffff {
		t.Fatalf("first sample = %#x", got)
	}
}
