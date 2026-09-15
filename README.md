# even-g2-go

Unofficial Go SDK for Even Realities G2.

## Status

Experimental. Protocol foundation is under active development.

## Current target

macOS → BLE → G2

## Implemented

- CRC-16/CCITT-FALSE
- G2 single-packet builder and parser
- macOS BLE scanning and left/right arm classification
- dual-arm connection, GATT discovery, and notifications
- seven-packet authentication
- heartbeat lifecycle
- teleprompter formatting, sending, and refresh
- EvenHub acknowledgement and input-event routing
- full-lens native text/list containers
- 4-bpp grayscale BMP encoding and 2x2 full-lens image tiling
- full-lens image streaming
- high-level connect, state reporting, and automatic reconnect
- local HTTP `/status`, `/text`, and `/image` bridge
- G2 microphone control and raw LC3 packet capture
- capture-confirmed AA 12 application packet codec and service identification

## Reference

[OpenEvenSdk](https://github.com/Thepizzapie/OpenEvenSdk)

This project is an independent Go implementation based on publicly available
reverse-engineered protocol documentation. It is not an official Even Realities SDK.

## Go API

```go
client, err := g2.Connect(ctx, g2.ConnectOptions{AutoReconnect: true})
if err != nil {
	return err
}
defer client.Close()

if err := client.ShowText(ctx, "hud", "Hello G2"); err != nil {
	return err
}
```

## Run

```bash
go run ./cmd/g2 scan
go run ./cmd/g2 hello --debug
go run ./cmd/g2 text --debug "hello world"
go run ./cmd/g2 native-text --debug "full lens text"
go run ./cmd/g2 list --debug "first row" "second row"
go run ./cmd/g2 image --debug ./picture.png
go run ./cmd/g2 mic --output ./capture.lc3
go run ./cmd/g2 mic --lc3-output ./frames.lc3
go run ./cmd/g2 mic --wav-output ./capture.wav
go run ./cmd/g2 decode-lc3 ./frames.lc3 ./capture.wav
go run ./cmd/g2 decode-packet 'AA 12 ...'
go run ./cmd/g2 serve --addr :8080
```

The HTTP bridge has no authentication and should only be exposed on a trusted network.

G2 microphone notifications contain five 40-byte LC3 frames followed by a
five-byte trailer. `--output` preserves the complete 205-byte notifications;
`--lc3-output` writes only the consecutive LC3 frames. The stream is 16 kHz,
mono, with 10 ms (160 sample) frames.

On macOS arm64, `--wav-output` and `decode-lc3` use the embedded Apache-2.0
`google/liblc3` v1.1.3 binary through a no-cgo FFI binding. No Homebrew package
is required at runtime.

Native events use the exported `protocol.EvenHubEvent*` constants. The CLI prints
readable names such as `click`, `scroll-top`, `scroll-bottom`, and `double-click`.
