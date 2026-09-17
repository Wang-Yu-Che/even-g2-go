package protocol

import (
	"bytes"
	"strings"
	"testing"
)

func TestBuildEvenHubNativeGoldens(t *testing.T) {
	tests := []struct {
		name string
		got  func() ([]byte, error)
		want []byte
	}{
		{"create list", func() ([]byte, error) { return BuildEvenHubCreateList("hud", []string{"one", ""}, 201) }, []byte{0x10, 0xC9, 0x01, 0x1A, 0x28, 0x08, 0x01, 0x12, 0x21, 0x18, 0xC0, 0x04, 0x20, 0xA0, 0x02, 0x48, 0x01, 0x52, 0x03, 'h', 'u', 'd', 0x5A, 0x10, 0x08, 0x02, 0x10, 0xC0, 0x04, 0x18, 0x01, 0x22, 0x03, 'o', 'n', 'e', 0x22, 0x02, 0xC2, 0xB7, 0x60, 0x01, 0x28, 0x90, 0x4E}},
		{"rebuild text", func() ([]byte, error) { return BuildEvenHubRebuildText("hud", "hello", 9) }, []byte{0x08, 0x07, 0x10, 0x09, 0x3A, 0x1A, 0x08, 0x01, 0x1A, 0x16, 0x18, 0xC0, 0x04, 0x20, 0xA0, 0x02, 0x48, 0x01, 0x52, 0x03, 'h', 'u', 'd', 0x58, 0x01, 0x62, 0x05, 'h', 'e', 'l', 'l', 'o'}},
		{"text upgrade", func() ([]byte, error) { return BuildEvenHubTextUpgrade("hud", "ok", 10) }, []byte{0x08, 0x05, 0x10, 0x0A, 0x4A, 0x0D, 0x08, 0x01, 0x12, 0x03, 'h', 'u', 'd', 0x20, 0x02, 0x2A, 0x02, 'o', 'k'}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.got()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, test.want) {
				t.Fatalf("payload = % X, want % X", got, test.want)
			}
		})
	}
}

func TestEvenHubContainerNameLimitUsesUTF8Bytes(t *testing.T) {
	_, err := BuildEvenHubRebuildText(strings.Repeat("界", 5), "text", 1)
	if err == nil {
		t.Fatal("expected a name longer than 14 UTF-8 bytes to fail")
	}
}

func TestBuildEvenHubRebuildStyledText(t *testing.T) {
	payload, err := BuildEvenHubRebuildStyledText("notice", "hello", 9,
		EvenHubGeometry{X: 20, Y: 20, Width: 536, Height: 248},
		EvenHubTextStyle{BorderWidth: 2, BorderColor: 15, BorderRadius: 8, PaddingLength: 12})
	if err != nil {
		t.Fatal(err)
	}
	// Text object fields 5-8: border width, colour, radius and padding.
	for _, field := range [][]byte{{0x28, 0x02}, {0x30, 0x0F}, {0x38, 0x08}, {0x40, 0x0C}} {
		if !bytes.Contains(payload, field) {
			t.Fatalf("payload % X does not contain style field % X", payload, field)
		}
	}
}

func TestBuildEvenHubRebuildStyledTextRejectsInvalidStyle(t *testing.T) {
	_, err := BuildEvenHubRebuildStyledText("notice", "hello", 9, EvenHubFullLens, EvenHubTextStyle{BorderWidth: 6})
	if err == nil {
		t.Fatal("expected invalid border width to fail")
	}
}

func TestBuildEvenHubImageMessages(t *testing.T) {
	tiles := []ImageTile{{ID: 10, Name: "t0", X: 0, Y: 0}, {ID: 11, Name: "t1", X: 288, Y: 0}}
	create, err := BuildEvenHubCreateImages(tiles, 201)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(create[:5], []byte{0x10, 0xC9, 0x01, 0x1A, 0x24}) {
		t.Fatalf("create prefix = % X", create[:5])
	}
	fragment, err := BuildEvenHubImageFragment(10, "t0", 2, 20000, 1, []byte{1, 2, 3}, 7)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x08, 0x03, 0x10, 0x07, 0x2A, 0x15, 0x08, 0x0A, 0x12, 0x02, 't', '0', 0x18, 0x02, 0x20, 0xA0, 0x9C, 0x01, 0x30, 0x01, 0x38, 0x03, 0x42, 0x03, 0x01, 0x02, 0x03}
	if !bytes.Equal(fragment, want) {
		t.Fatalf("fragment = % X, want % X", fragment, want)
	}
}

func TestBuildEvenHubCreateTextImage(t *testing.T) {
	payload, err := BuildEvenHubCreateTextImage("status", "running", EvenHubGeometry{X: 64, Y: 20, Width: 492, Height: 248}, EvenHubTextStyle{}, EvenHubImage{ID: 2, Name: "state", X: 20, Y: 24, Width: 32, Height: 32}, 201)
	if err != nil {
		t.Fatalf("BuildEvenHubCreateTextImage() error = %v", err)
	}
	if len(payload) == 0 || payload[0] != 0x10 {
		t.Fatalf("payload = %x", payload)
	}
}

func TestBuildEvenHubCreateTextImages(t *testing.T) {
	payload, err := BuildEvenHubCreateTextImages("status", "running", EvenHubFullLens, EvenHubTextStyle{}, []EvenHubImage{
		{ID: 2, Name: "terminal", X: 20, Y: 20, Width: 20, Height: 20},
		{ID: 3, Name: "state", X: 516, Y: 20, Width: 20, Height: 20},
	}, 201)
	if err != nil {
		t.Fatal(err)
	}
	// Create field 1 reports three total containers: one text plus two images.
	if len(payload) < 7 || !bytes.Equal(payload[5:7], []byte{0x08, 0x03}) {
		t.Fatalf("payload does not contain a three-container create: % X", payload)
	}
}

func TestBuildEvenHubRebuildTextImagesUsesRebuildCommand(t *testing.T) {
	payload, err := BuildEvenHubRebuildTextImages("status", "running", EvenHubFullLens, EvenHubTextStyle{}, []EvenHubImage{
		{ID: 2, Name: "terminal", X: 20, Y: 20, Width: 20, Height: 20},
		{ID: 3, Name: "state", X: 516, Y: 20, Width: 20, Height: 20},
	}, 9)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) < 2 || !bytes.Equal(payload[:2], []byte{0x08, 0x07}) {
		t.Fatalf("payload is not Cmd=7 rebuild: % X", payload)
	}
	if bytes.Contains(payload, BuildEvenHubShutdown(9)) {
		t.Fatalf("rebuild payload contains shutdown: % X", payload)
	}
}
