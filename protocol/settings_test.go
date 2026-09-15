package protocol

import (
	"slices"
	"testing"
)

func TestBuildSettingsQuery(t *testing.T) {
	want := []byte{0x08, 0x02, 0x10, 0x2A, 0x22, 0x02, 0x08, 0x01}
	if got := BuildSettingsQuery(42); !slices.Equal(got, want) {
		t.Fatalf("query = % X, want % X", got, want)
	}
}

func TestBuildSetBrightness(t *testing.T) {
	got, err := BuildSetBrightness(42, 60, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x08, 0x01, 0x10, 0x2A, 0x1A, 0x06, 0x0A, 0x04, 0x08, 0x01, 0x10, 0x3C}
	if !slices.Equal(got, want) {
		t.Fatalf("brightness = % X, want % X", got, want)
	}
}

func TestBuildDisplaySettings(t *testing.T) {
	tests := []struct {
		name string
		got  []byte
		err  error
		want []byte
	}{
		{name: "head-up switch", got: BuildSetHeadUpSwitch(42, true), want: []byte{0x08, 0x01, 0x10, 0x2A, 0x1A, 0x04, 0x22, 0x02, 0x08, 0x01}},
	}
	angle, angleErr := BuildSetHeadUpAngle(42, 30)
	height, heightErr := BuildSetScreenHeight(42, 4)
	depth, depthErr := BuildSetScreenDepth(42, 1)
	tests = append(tests,
		struct {
			name string
			got  []byte
			err  error
			want []byte
		}{"head-up angle", angle, angleErr, []byte{0x08, 0x01, 0x10, 0x2A, 0x1A, 0x04, 0x22, 0x02, 0x10, 0x1E}},
		struct {
			name string
			got  []byte
			err  error
			want []byte
		}{"screen height", height, heightErr, []byte{0x08, 0x01, 0x10, 0x2A, 0x1A, 0x04, 0x12, 0x02, 0x08, 0x04}},
		struct {
			name string
			got  []byte
			err  error
			want []byte
		}{"screen depth", depth, depthErr, []byte{0x08, 0x01, 0x10, 0x2A, 0x1A, 0x04, 0x1A, 0x02, 0x08, 0x01}},
	)
	for _, test := range tests {
		if test.err != nil {
			t.Fatalf("%s: %v", test.name, test.err)
		}
		if !slices.Equal(test.got, test.want) {
			t.Fatalf("%s = % X, want % X", test.name, test.got, test.want)
		}
	}
}

func TestParseDeviceSettings(t *testing.T) {
	inner := append(protoUint(2, 55), protoString(5, "2.2.9.22")...)
	inner = append(inner, protoString(6, "2.2.9.22")...)
	inner = append(inner, protoUint(12, 87)...)
	inner = append(inner, protoUint(13, 1)...)
	got, err := ParseDeviceSettings(settingsEnvelope(2, 42, 4, inner))
	if err != nil {
		t.Fatal(err)
	}
	if got.BatteryPercent != 87 || !got.Charging || got.Brightness != 55 || got.LeftFirmwareVersion != "2.2.9.22" {
		t.Fatalf("snapshot = %#v", got)
	}
}
