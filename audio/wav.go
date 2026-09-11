// Package audio contains PCM output helpers for G2 audio.
package audio

import (
	"encoding/binary"
	"errors"
	"io"
)

// WAVWriter writes 16-bit mono PCM and patches its header when closed.
type WAVWriter struct {
	output     io.WriteSeeker
	sampleRate uint32
	dataBytes  uint32
	closed     bool
}

// NewWAVWriter starts a 16-bit mono WAV stream at the requested sample rate.
func NewWAVWriter(output io.WriteSeeker, sampleRate int) (*WAVWriter, error) {
	writer := &WAVWriter{output: output, sampleRate: uint32(sampleRate)}
	if err := writer.writeHeader(); err != nil {
		return nil, err
	}
	return writer, nil
}

// WritePCM appends signed 16-bit mono samples.
func (w *WAVWriter) WritePCM(samples []int16) error {
	if w.closed {
		return errors.New("WAV writer is closed")
	}
	if err := binary.Write(w.output, binary.LittleEndian, samples); err != nil {
		return err
	}
	w.dataBytes += uint32(len(samples) * 2)
	return nil
}

// Close patches the RIFF and data sizes without closing the underlying output.
func (w *WAVWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	if _, err := w.output.Seek(4, io.SeekStart); err != nil {
		return err
	}
	if err := binary.Write(w.output, binary.LittleEndian, uint32(36)+w.dataBytes); err != nil {
		return err
	}
	if _, err := w.output.Seek(40, io.SeekStart); err != nil {
		return err
	}
	return binary.Write(w.output, binary.LittleEndian, w.dataBytes)
}

func (w *WAVWriter) writeHeader() error {
	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1)
	binary.LittleEndian.PutUint16(header[22:24], 1)
	binary.LittleEndian.PutUint32(header[24:28], w.sampleRate)
	binary.LittleEndian.PutUint32(header[28:32], w.sampleRate*2)
	binary.LittleEndian.PutUint16(header[32:34], 2)
	binary.LittleEndian.PutUint16(header[34:36], 16)
	copy(header[36:40], "data")
	_, err := w.output.Write(header)
	return err
}
