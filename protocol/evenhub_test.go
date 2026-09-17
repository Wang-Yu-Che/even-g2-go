package protocol

import (
	"bytes"
	"testing"
)

func TestFrameEvenHubFragmentsAndCRC(t *testing.T) {
	payload := []byte{0x08, 0x0C, 0x10, 0x4D, 0x72, 0x00}
	frames, err := FrameEvenHub(0x31, EvenHubServiceID, EvenHubRequest, payload, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(frames) != 2 {
		t.Fatalf("frame count = %d, want 2", len(frames))
	}
	if !bytes.Equal(frames[0][:8], []byte{0xAA, 0x21, 0x31, 0x04, 0x02, 0x01, 0xE0, 0x20}) || frames[1][5] != 2 {
		t.Fatalf("unexpected fragments: % X", frames)
	}
	joined := append(append([]byte(nil), frames[0][8:]...), frames[1][8:]...)
	wantCRC := CRC16CCITT(payload)
	if !bytes.Equal(joined[:len(payload)], payload) || !bytes.Equal(joined[len(payload):], []byte{byte(wantCRC), byte(wantCRC >> 8)}) {
		t.Fatalf("reassembled payload = % X", joined)
	}
}

func TestBuildEvenHubHeartbeat(t *testing.T) {
	want := []byte{0x08, 0x0C, 0x10, 0x4D, 0x72, 0x00}
	if got := BuildEvenHubHeartbeat(77); !bytes.Equal(got, want) {
		t.Fatalf("heartbeat = % X, want % X", got, want)
	}
}

func TestParseEvenHubResponse(t *testing.T) {
	result := byte(4)
	response, err := ParseEvenHubResponse([]byte{0x08, 0x04, 0x10, 0x2A, 0x1A, 0x02, 0x08, result})
	if err != nil {
		t.Fatal(err)
	}
	if response.Command != 4 || response.Magic != 42 || response.Result == nil || *response.Result != 4 {
		t.Fatalf("response = %#v", response)
	}
}

func TestParseEvenHubEvents(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    EvenHubEvent
	}{
		{"list", []byte{0x08, 0x02, 0x6A, 0x0D, 0x0A, 0x0B, 0x12, 0x03, 'h', 'u', 'd', 0x1A, 0x02, 'o', 'k', 0x20, 0x02}, EvenHubEvent{Kind: EvenHubEventList, Name: "hud", ItemName: "ok", ItemIndex: 2}},
		{"text", []byte{0x08, 0x02, 0x6A, 0x09, 0x12, 0x07, 0x12, 0x03, 'h', 'u', 'd', 0x18, 0x03}, EvenHubEvent{Kind: EvenHubEventText, Name: "hud", Type: 3}},
		{"system", []byte{0x08, 0x02, 0x6A, 0x08, 0x1A, 0x06, 0x08, 0x07, 0x10, 0x03, 0x20, 0x02}, EvenHubEvent{Kind: EvenHubEventSystem, Type: 7, Source: EvenHubEventSourceGlassesLeft, ExitReason: 2}},
		{"private", []byte{0x08, 0x0B, 0x82, 0x01, 0x09, 0x12, 0x03, 'h', 'u', 'd', 0x18, 0x09, 0x20, 0x02}, EvenHubEvent{Kind: EvenHubEventPrivate, Name: "hud", EventID: 9, EventData: 2}},
		{"menu", []byte{0x08, 0x11, 0xA2, 0x01, 0x03, 0x08, 0xAD, 0x4E}, EvenHubEvent{Kind: EvenHubEventMenu, AppID: 10029}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseEvenHubEvent(test.payload)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("event = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestParseEvenHubRejectsTruncatedProtobuf(t *testing.T) {
	if _, err := ParseEvenHubEvent([]byte{0x6A, 0x05, 0x08}); err == nil {
		t.Fatal("expected truncated protobuf error")
	}
}

func TestEvenHubEventNames(t *testing.T) {
	if got := EvenHubEventList.String(); got != "list" {
		t.Fatalf("kind name = %q", got)
	}
	if got := EvenHubEventTypeName(EvenHubEventDoubleClick); got != "double-click" {
		t.Fatalf("event type name = %q", got)
	}
	if got := EvenHubEventTypeName(99); got != "unknown(99)" {
		t.Fatalf("unknown event type name = %q", got)
	}
	if EvenHubControlServiceID != 0x81 || EvenHubServiceID != 0xE0 {
		t.Fatalf("EvenHub service IDs = %02X/%02X", EvenHubControlServiceID, EvenHubServiceID)
	}
}
