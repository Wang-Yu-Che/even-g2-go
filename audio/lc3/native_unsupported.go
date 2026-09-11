//go:build !darwin || !arm64

package lc3

func newNativeDecoder() (nativeDecoder, error) {
	return nil, ErrUnsupported
}
