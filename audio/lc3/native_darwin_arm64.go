//go:build darwin && arm64

package lc3

import (
	_ "embed"
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
)

//go:embed native/darwin_arm64/liblc3.dylib
var embeddedLibrary []byte

type darwinDecoder struct {
	library    uintptr
	memory     []byte
	handle     uintptr
	decodeFunc func(uintptr, unsafe.Pointer, int32, int32, unsafe.Pointer, int32) int32
}

func newNativeDecoder() (nativeDecoder, error) {
	file, err := os.CreateTemp("", "even-g2-go-liblc3-*.dylib")
	if err != nil {
		return nil, err
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err := file.Write(embeddedLibrary); err != nil {
		file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}

	library, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, fmt.Errorf("load embedded liblc3: %w", err)
	}
	var decoderSize func(int32, int32) uint32
	var setupDecoder func(int32, int32, int32, unsafe.Pointer) uintptr
	var decode func(uintptr, unsafe.Pointer, int32, int32, unsafe.Pointer, int32) int32
	purego.RegisterLibFunc(&decoderSize, library, "lc3_decoder_size")
	purego.RegisterLibFunc(&setupDecoder, library, "lc3_setup_decoder")
	purego.RegisterLibFunc(&decode, library, "lc3_decode")

	memory := make([]byte, decoderSize(FrameMicros, SampleRate))
	if len(memory) == 0 {
		purego.Dlclose(library)
		return nil, errorsNewDecoderSetup()
	}
	handle := setupDecoder(FrameMicros, SampleRate, 0, unsafe.Pointer(&memory[0]))
	if handle == 0 {
		purego.Dlclose(library)
		return nil, errorsNewDecoderSetup()
	}
	return &darwinDecoder{library: library, memory: memory, handle: handle, decodeFunc: decode}, nil
}

func errorsNewDecoderSetup() error {
	return fmt.Errorf("liblc3 rejected %d us/%d Hz decoder configuration", FrameMicros, SampleRate)
}

func (d *darwinDecoder) decode(frame []byte, pcm []int16) error {
	if d.handle == 0 {
		return errorsNewDecoderSetup()
	}
	var input unsafe.Pointer
	if len(frame) > 0 {
		input = unsafe.Pointer(&frame[0])
	}
	result := d.decodeFunc(d.handle, input, int32(len(frame)), 0, unsafe.Pointer(&pcm[0]), 1)
	runtime.KeepAlive(frame)
	runtime.KeepAlive(pcm)
	runtime.KeepAlive(d.memory)
	if result < 0 {
		return fmt.Errorf("liblc3 decode failed: result=%d", result)
	}
	return nil
}

func (d *darwinDecoder) close() error {
	if d.library == 0 {
		return nil
	}
	err := purego.Dlclose(d.library)
	d.library = 0
	d.handle = 0
	d.memory = nil
	return err
}
