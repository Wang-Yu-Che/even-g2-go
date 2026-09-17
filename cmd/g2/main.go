package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/audio"
	"github.com/Wang-Yu-Che/even-g2-go/audio/lc3"
	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/bridge"
	"github.com/Wang-Yu-Che/even-g2-go/g2"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

const defaultScanTimeout = 20 * time.Second

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "g2: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: g2 <scan|connect|auth|settings|brightness|head-up|screen-position|dashboard|hello|text|native-text|list|image|mic|decode-lc3|decode-packet|serve>")
	}

	switch args[0] {
	case "scan":
		return runScan(args[1:])
	case "connect":
		return runConnect(args[1:])
	case "auth":
		return runAuth(args[1:])
	case "settings":
		return runSettings(args[1:])
	case "brightness":
		return runBrightness(args[1:])
	case "head-up":
		return runHeadUp(args[1:])
	case "screen-position":
		return runScreenPosition(args[1:])
	case "dashboard":
		return runDashboard(args[1:])
	case "hello":
		return runDisplay(args[1:], "Hello G2")
	case "text":
		return runDisplay(args[1:], "")
	case "native-text":
		return runNative(args[1:], false)
	case "list":
		return runNative(args[1:], true)
	case "image":
		return runImage(args[1:])
	case "serve":
		return runServe(args[1:])
	case "mic":
		return runMic(args[1:])
	case "decode-lc3":
		return runDecodeLC3(args[1:])
	case "decode-packet":
		return runDecodePacket(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func withClient(scanTimeout time.Duration, debug bool, run func(context.Context, *g2.Client) error) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client, err := g2.Connect(ctx, g2.ConnectOptions{ScanTimeout: scanTimeout, Debug: debug, Output: os.Stdout})
	if err != nil {
		return err
	}
	defer client.Close()
	return run(ctx, client)
}

func runSettings(args []string) error {
	flags := flag.NewFlagSet("settings", flag.ContinueOnError)
	scanTimeout := flags.Duration("scan-timeout", defaultScanTimeout, "BLE scan duration")
	debug := flags.Bool("debug", false, "print TX and RX packet bytes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	return withClient(*scanTimeout, *debug, func(ctx context.Context, client *g2.Client) error {
		settings, err := client.RequestDeviceSettings(ctx)
		if err != nil {
			return err
		}
		encoded, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(encoded))
		return nil
	})
}

func runBrightness(args []string) error {
	flags := flag.NewFlagSet("brightness", flag.ContinueOnError)
	level := flags.Int("level", 50, "brightness level 0..100")
	automatic := flags.Bool("auto", false, "enable ambient auto brightness")
	scanTimeout := flags.Duration("scan-timeout", defaultScanTimeout, "BLE scan duration")
	debug := flags.Bool("debug", false, "print TX and RX packet bytes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	return withClient(*scanTimeout, *debug, func(ctx context.Context, client *g2.Client) error {
		return client.SetBrightness(ctx, g2.BrightnessOptions{Level: *level, Auto: *automatic})
	})
}

func runHeadUp(args []string) error {
	flags := flag.NewFlagSet("head-up", flag.ContinueOnError)
	enabled := flags.Bool("enabled", true, "enable head-up dashboard")
	angle := flags.Int("angle", 30, "trigger angle 0..60")
	scanTimeout := flags.Duration("scan-timeout", defaultScanTimeout, "BLE scan duration")
	debug := flags.Bool("debug", false, "print TX and RX packet bytes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	return withClient(*scanTimeout, *debug, func(ctx context.Context, client *g2.Client) error {
		return client.SetHeadUp(ctx, *enabled, *angle)
	})
}

func runScreenPosition(args []string) error {
	flags := flag.NewFlagSet("screen-position", flag.ContinueOnError)
	height := flags.Int("height", 0, "vertical position 0..12")
	depth := flags.Int("depth", 0, "depth position 0..2")
	scanTimeout := flags.Duration("scan-timeout", defaultScanTimeout, "BLE scan duration")
	debug := flags.Bool("debug", false, "print TX and RX packet bytes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	return withClient(*scanTimeout, *debug, func(ctx context.Context, client *g2.Client) error {
		return client.SetScreenPosition(ctx, g2.ScreenPosition{Height: *height, Depth: *depth})
	})
}

func runDashboard(args []string) error {
	flags := flag.NewFlagSet("dashboard", flag.ContinueOnError)
	scanTimeout := flags.Duration("scan-timeout", defaultScanTimeout, "BLE scan duration")
	debug := flags.Bool("debug", false, "print TX and RX packet bytes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	return withClient(*scanTimeout, *debug, func(ctx context.Context, client *g2.Client) error {
		return client.ShowDashboard(ctx)
	})
}

func runDecodePacket(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: g2 decode-packet <hex>")
	}
	data, err := hex.DecodeString(strings.NewReplacer(" ", "", ":", "", "-", "").Replace(args[0]))
	if err != nil {
		return fmt.Errorf("decode packet hex: %w", err)
	}
	if len(data) < 2 {
		return protocol.ErrPacketTooShort
	}
	switch data[1] {
	case protocol.ApplicationPacketVersion:
		packet, err := protocol.ParseApplicationPacket(data)
		if err != nil {
			return err
		}
		fmt.Printf("version=AA12 sequence=%d service=0x%04X (%s) payload=%X\n",
			packet.Sequence, packet.ServiceID, protocol.ApplicationServiceName(packet.ServiceID), packet.Payload)
		return nil
	case protocol.PacketMagic1:
		packet, err := protocol.ParsePacket(data)
		if err != nil {
			return err
		}
		fmt.Printf("version=AA21 sequence=%d service=%02X%02X payload=%X\n",
			packet.Sequence, packet.ServiceHi, packet.ServiceLo, packet.Payload)
		return nil
	default:
		return fmt.Errorf("unsupported packet version 0x%02X", data[1])
	}
}

func runDecodeLC3(args []string) error {
	flags := flag.NewFlagSet("decode-lc3", flag.ContinueOnError)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(flags.Args()) != 2 {
		return errors.New("usage: g2 decode-lc3 <frames.lc3> <output.wav>")
	}
	encoded, err := os.ReadFile(flags.Args()[0])
	if err != nil {
		return err
	}
	if len(encoded)%lc3.FrameBytes != 0 {
		return fmt.Errorf("LC3 input size %d is not divisible by %d", len(encoded), lc3.FrameBytes)
	}
	decoder, err := lc3.NewDecoder()
	if err != nil {
		return err
	}
	defer decoder.Close()
	output, err := os.Create(flags.Args()[1])
	if err != nil {
		return err
	}
	defer output.Close()
	wavWriter, err := audio.NewWAVWriter(output, lc3.SampleRate)
	if err != nil {
		return err
	}
	for offset := 0; offset < len(encoded); offset += lc3.FrameBytes {
		pcm, err := decoder.DecodeFrame(encoded[offset : offset+lc3.FrameBytes])
		if err != nil {
			return fmt.Errorf("decode frame %d: %w", offset/lc3.FrameBytes, err)
		}
		if err := wavWriter.WritePCM(pcm); err != nil {
			return err
		}
	}
	if err := wavWriter.Close(); err != nil {
		return err
	}
	duration := time.Duration(len(encoded)/lc3.FrameBytes) * 10 * time.Millisecond
	fmt.Printf("[LC3] decoded %d frame(s), duration=%s, output=%s\n", len(encoded)/lc3.FrameBytes, duration, flags.Args()[1])
	return nil
}

func runMic(args []string) error {
	flags := flag.NewFlagSet("mic", flag.ContinueOnError)
	outputPath := flags.String("output", "", "optional raw LC3 output file")
	lc3OutputPath := flags.String("lc3-output", "", "optional header-free LC3 frame output file")
	wavOutputPath := flags.String("wav-output", "", "optional decoded 16 kHz mono WAV output file")
	debug := flags.Bool("debug", false, "print TX and RX packet bytes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client, err := g2.Connect(ctx, g2.ConnectOptions{Debug: *debug, Output: os.Stdout, AutoReconnect: true})
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.StartMicrophone(ctx); err != nil {
		return err
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = client.StopMicrophone(stopCtx)
	}()

	var output *os.File
	if *outputPath != "" {
		output, err = os.Create(*outputPath)
		if err != nil {
			return err
		}
		defer output.Close()
	}
	var lc3Output *os.File
	if *lc3OutputPath != "" {
		lc3Output, err = os.Create(*lc3OutputPath)
		if err != nil {
			return err
		}
		defer lc3Output.Close()
	}
	var wavOutput *os.File
	var wavWriter *audio.WAVWriter
	var decoder *lc3.Decoder
	if *wavOutputPath != "" {
		wavOutput, err = os.Create(*wavOutputPath)
		if err != nil {
			return err
		}
		defer wavOutput.Close()
		wavWriter, err = audio.NewWAVWriter(wavOutput, lc3.SampleRate)
		if err != nil {
			return err
		}
		defer wavWriter.Close()
		decoder, err = lc3.NewDecoder()
		if err != nil {
			return err
		}
		defer decoder.Close()
	}
	fmt.Println("[MIC] recording raw LC3 frames; press Ctrl-C to stop")
	audioFrames := client.SubscribeAudio(ctx)
	count := 0
	lastCounters := make(map[ble.Arm]byte)
	seenCounters := make(map[ble.Arm]bool)
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-client.Errors():
			return err
		case frame := <-audioFrames:
			count++
			packet, parseErr := protocol.ParseG2AudioPacket(frame.Data)
			if parseErr != nil {
				return parseErr
			}
			gap := 0
			duplicate := false
			if seenCounters[frame.Arm] {
				delta := int(uint8(packet.Counter - lastCounters[frame.Arm]))
				if delta == 0 {
					duplicate = true
					fmt.Printf("[MIC] duplicate arm=%s counter=%d\n", frame.Arm, packet.Counter)
				} else {
					gap = delta - 1
				}
			}
			lastCounters[frame.Arm] = packet.Counter
			seenCounters[frame.Arm] = true
			fmt.Printf("[MIC] packet=%d arm=%s bytes=%d counter=%d missing=%d\n", count, frame.Arm, len(frame.Data), packet.Counter, gap)
			if output != nil {
				if _, err := output.Write(frame.Data); err != nil {
					return err
				}
			}
			if lc3Output != nil {
				for i := range packet.Frames {
					if _, err := lc3Output.Write(packet.Frames[i][:]); err != nil {
						return err
					}
				}
			}
			if wavWriter != nil && !duplicate {
				if gap > 0 && gap <= 8 {
					for range gap * protocol.G2AudioFramesPerPacket {
						pcm, err := decoder.DecodeLostFrame()
						if err != nil {
							return err
						}
						if err := wavWriter.WritePCM(pcm); err != nil {
							return err
						}
					}
				}
				for i := range packet.Frames {
					pcm, err := decoder.DecodeFrame(packet.Frames[i][:])
					if err != nil {
						return err
					}
					if err := wavWriter.WritePCM(pcm); err != nil {
						return err
					}
				}
			}
		}
	}
}

func runServe(args []string) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	address := flags.String("addr", ":8080", "HTTP listen address")
	scanTimeout := flags.Duration("scan-timeout", defaultScanTimeout, "BLE scan duration")
	debug := flags.Bool("debug", false, "print TX and RX packet bytes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client, err := g2.Connect(ctx, g2.ConnectOptions{
		ScanTimeout:   *scanTimeout,
		Debug:         *debug,
		Output:        os.Stdout,
		AutoReconnect: true,
	})
	if err != nil {
		return err
	}
	defer client.Close()

	server := &http.Server{
		Addr:              *address,
		Handler:           bridge.NewServerWithContext(ctx, client).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	serverErr := make(chan error, 1)
	go func() { serverErr <- server.ListenAndServe() }()
	fmt.Printf("[HTTP] listening on %s without authentication; use only on a trusted network\n", *address)
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func runImage(args []string) error {
	flags := flag.NewFlagSet("image", flag.ContinueOnError)
	scanTimeout := flags.Duration("scan-timeout", defaultScanTimeout, "BLE scan duration")
	debug := flags.Bool("debug", false, "print TX and RX packet bytes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *scanTimeout <= 0 {
		return errors.New("scan timeout must be positive")
	}
	if len(flags.Args()) != 1 {
		return errors.New("usage: g2 image [--debug] <png-or-jpeg-file>")
	}
	file, err := os.Open(flags.Args()[0])
	if err != nil {
		return err
	}
	decoded, _, err := image.Decode(file)
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client, err := g2.Connect(ctx, g2.ConnectOptions{ScanTimeout: *scanTimeout, Debug: *debug, Output: os.Stdout})
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.DisplayImage(ctx, decoded); err != nil {
		return err
	}
	fmt.Println("[IMAGE] displaying; press Ctrl-C to stop")
	select {
	case <-ctx.Done():
		return nil
	case err := <-client.Errors():
		return err
	}
}

func runNative(args []string, list bool) error {
	flags := flag.NewFlagSet("native", flag.ContinueOnError)
	scanTimeout := flags.Duration("scan-timeout", defaultScanTimeout, "BLE scan duration")
	debug := flags.Bool("debug", false, "print TX and RX packet bytes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *scanTimeout <= 0 {
		return errors.New("scan timeout must be positive")
	}
	content := flags.Args()
	if len(content) == 0 {
		return errors.New("native-text requires text; list requires one or more rows")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client, err := g2.Connect(ctx, g2.ConnectOptions{ScanTimeout: *scanTimeout, Debug: *debug, Output: os.Stdout})
	if err != nil {
		return err
	}
	defer client.Close()
	events := client.SubscribeEvents(ctx)
	navigation := listNavigation{rows: content}
	if list {
		if err := client.DisplayList(ctx, "hud", content); err != nil {
			return err
		}
	} else if err := client.ShowText(ctx, "hud", strings.Join(content, " ")); err != nil {
		return err
	}
	fmt.Println("[NATIVE] displaying; interact with the glasses or press Ctrl-C to stop")

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-client.Errors():
			return err
		case event := <-events:
			fmt.Printf("[EVENT] kind=%s name=%q item=%q index=%d type=%s(%d) data=%d\n",
				event.Kind, event.Name, event.ItemName, event.ItemIndex,
				protocol.EvenHubEventTypeName(event.Type), event.Type, event.EventData)
			if list {
				if err := navigation.handle(ctx, client, event, time.Now()); err != nil {
					return err
				}
			}
		}
	}
}

func runConnect(args []string) error {
	flags := flag.NewFlagSet("connect", flag.ContinueOnError)
	scanTimeout := flags.Duration("scan-timeout", defaultScanTimeout, "BLE scan duration")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *scanTimeout <= 0 {
		return errors.New("scan timeout must be positive")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client, err := g2.Connect(ctx, g2.ConnectOptions{ScanTimeout: *scanTimeout, Debug: true, Output: os.Stdout})
	if err != nil {
		return err
	}
	defer client.Close()
	fmt.Println("[BLE] authenticated and ready; press Ctrl-C to disconnect")
	<-ctx.Done()
	return nil
}

func runAuth(args []string) error {
	flags := flag.NewFlagSet("auth", flag.ContinueOnError)
	scanTimeout := flags.Duration("scan-timeout", defaultScanTimeout, "BLE scan duration")
	debug := flags.Bool("debug", false, "print TX and RX packet bytes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *scanTimeout <= 0 {
		return errors.New("scan timeout must be positive")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client, err := g2.Connect(ctx, g2.ConnectOptions{ScanTimeout: *scanTimeout, Debug: *debug, Output: os.Stdout})
	if err != nil {
		return err
	}
	defer client.Close()
	fmt.Println("[AUTH] ready")
	return nil
}

func runDisplay(args []string, fixedText string) error {
	flags := flag.NewFlagSet("display", flag.ContinueOnError)
	scanTimeout := flags.Duration("scan-timeout", defaultScanTimeout, "BLE scan duration")
	debug := flags.Bool("debug", false, "print TX and RX packet bytes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *scanTimeout <= 0 {
		return errors.New("scan timeout must be positive")
	}
	text := fixedText
	if text == "" {
		text = strings.Join(flags.Args(), " ")
		if text == "" {
			return errors.New("usage: g2 text [--debug] <text>")
		}
	} else if len(flags.Args()) != 0 {
		return errors.New("hello command does not accept text arguments")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client, err := g2.Connect(ctx, g2.ConnectOptions{ScanTimeout: *scanTimeout, Debug: *debug, Output: os.Stdout})
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.DisplayText(ctx, text); err != nil {
		return err
	}
	fmt.Printf("[TEXT] displaying %q; press Ctrl-C to stop\n", text)

	select {
	case <-ctx.Done():
		return nil
	case err := <-client.Errors():
		return err
	}
}

func runScan(args []string) error {
	flags := flag.NewFlagSet("scan", flag.ContinueOnError)
	timeout := flags.Duration("timeout", defaultScanTimeout, "BLE scan duration")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *timeout <= 0 {
		return errors.New("scan timeout must be positive")
	}

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(signalCtx, *timeout)
	defer cancel()

	fmt.Printf("[SCAN] searching for Even G2 devices for %s...\n", timeout.String())

	seen := make(map[string]struct{})
	var seenMu sync.Mutex
	err := ble.NewScanner().Scan(ctx, func(result ble.ScanResult) {
		address := result.Address
		seenMu.Lock()
		if _, exists := seen[address]; exists {
			seenMu.Unlock()
			return
		}
		seen[address] = struct{}{}
		seenMu.Unlock()

		fmt.Printf("[SCAN] %-5s name=%q address=%s rssi=%d\n", result.Arm, result.Name, address, result.RSSI)
	})
	if err != nil {
		return err
	}

	seenMu.Lock()
	count := len(seen)
	seenMu.Unlock()
	fmt.Printf("[SCAN] complete: %d Even device(s) found\n", count)
	return nil
}
