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
- G2 settings query: battery, charging, per-arm firmware, brightness, head-up,
  wearing detection, and screen position
- brightness, automatic brightness, head-up angle, and screen-position controls
- stock dashboard release plus experimental widget order and Schedule injection

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

`Connect` succeeds only when discovery finds exactly one left and one right G2
arm. Applications that support multiple nearby glasses should scan the arm
candidates, let the user select a verified pair, and connect explicitly:

```go
arms, err := g2.ScanArms(ctx, g2.ScanOptions{Timeout: 10 * time.Second})
if err != nil {
	return err
}
device, err := g2.NewDevice(arms[ble.Left][0], arms[ble.Right][0])
if err != nil {
	return err
}
client, err := g2.ConnectDevice(ctx, device, g2.ConnectOptions{})
```

Runtime state and input events use independent subscriptions:

```go
statuses := client.SubscribeStatus(ctx)
events := client.SubscribeEvents(ctx)
```

`Disconnect` releases the current BLE connections while retaining the selected
device for `Reconnect`. `Close` permanently stops the client session.

## Run

```bash
go run ./cmd/g2 scan
go run ./cmd/g2 settings --debug
go run ./cmd/g2 brightness --level 60 --auto
go run ./cmd/g2 head-up --enabled=true --angle 30
go run ./cmd/g2 screen-position --height 4 --depth 1
go run ./cmd/g2 dashboard
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

`list` 支持单击选中项查看完整文字、双击返回列表。选中框按列表容器宽度
铺满整行（576 像素）。列表参数本身就是详情文字，不会额外读取文件或请求服务。
