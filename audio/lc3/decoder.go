// Package lc3 decodes the LC3 stream produced by Even Realities G2 glasses.
package lc3

import (
	"errors"
	"fmt"
)

const (
	SampleRate   = 16000
	FrameMicros  = 10000
	FrameBytes   = 40
	FrameSamples = 160
)

var ErrUnsupported = errors.New("embedded LC3 decoder is unsupported on this platform")

// Decoder preserves LC3 state across consecutive mono frames.
type Decoder struct {
	native nativeDecoder
}

// NewDecoder loads the embedded decoder for the current platform.
func NewDecoder() (*Decoder, error) {
	native, err := newNativeDecoder()
	if err != nil {
		return nil, err
	}
	return &Decoder{native: native}, nil
}

// DecodeFrame decodes one 10 ms G2 LC3 frame into 16-bit mono PCM.
func (d *Decoder) DecodeFrame(frame []byte) ([]int16, error) {
	if len(frame) != FrameBytes {
		return nil, fmt.Errorf("LC3 frame must be %d bytes, got %d", FrameBytes, len(frame))
	}
	pcm := make([]int16, FrameSamples)
	if err := d.native.decode(frame, pcm); err != nil {
		return nil, err
	}
	return pcm, nil
}

// DecodeLostFrame applies packet-loss concealment for one missing frame.
func (d *Decoder) DecodeLostFrame() ([]int16, error) {
	pcm := make([]int16, FrameSamples)
	if err := d.native.decode(nil, pcm); err != nil {
		return nil, err
	}
	return pcm, nil
}

// Close releases the native decoder.
func (d *Decoder) Close() error {
	if d.native == nil {
		return nil
	}
	err := d.native.close()
	d.native = nil
	return err
}

type nativeDecoder interface {
	decode(frame []byte, pcm []int16) error
	close() error
}
