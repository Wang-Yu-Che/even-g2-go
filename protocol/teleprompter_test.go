package protocol

import (
	"bytes"
	"strings"
	"testing"
)

func TestFormatTeleprompterHello(t *testing.T) {
	pages := FormatTeleprompter("Hello G2")
	if len(pages) != TeleprompterMinimumPages {
		t.Fatalf("page count = %d, want %d", len(pages), TeleprompterMinimumPages)
	}
	if pages[0].Number != 0 || !strings.HasPrefix(pages[0].Text, "Hello G2\n") {
		t.Fatalf("first page = %#v", pages[0])
	}
	for index, page := range pages {
		if page.Number != index {
			t.Fatalf("page number = %d, want %d", page.Number, index)
		}
		if strings.Count(page.Text, "\n") != TeleprompterLinesPerPage {
			t.Fatalf("page %d newline count = %d", index, strings.Count(page.Text, "\n"))
		}
		if !strings.HasSuffix(page.Text, " \n") {
			t.Fatalf("page %d lacks required suffix", index)
		}
	}
}

func TestFormatTeleprompterWrapsAndExpandsLiteralNewlines(t *testing.T) {
	pages := FormatTeleprompter("1234567890 1234567890 12345\\nnext")
	lines := strings.Split(pages[0].Text, "\n")
	if lines[0] != "1234567890 1234567890" || lines[1] != "12345" || lines[2] != "next" {
		t.Fatalf("wrapped lines = %#v", lines[:3])
	}
	if got := TeleprompterInputLineCount("first\\nsecond"); got != 2 {
		t.Fatalf("TeleprompterInputLineCount() = %d, want 2", got)
	}
}

func TestFormatTeleprompterWrapsChineseWithoutSpaces(t *testing.T) {
	pages := FormatTeleprompter("这是一段提词器测试文本，用于验证分页、同步和自动刷新。")
	lines := strings.Split(pages[0].Text, "\n")
	if lines[0] != "这是一段提词器测试文本" || lines[1] != "，用于验证分页、同步和" || lines[2] != "自动刷新。" {
		t.Fatalf("wrapped Chinese lines = %#v", lines[:3])
	}
}

func TestTeleprompterPacketGoldens(t *testing.T) {
	tests := []struct {
		name string
		got  []byte
		want []byte
	}{
		{
			name: "init",
			got:  BuildTeleprompterInit(0x09, 0x15, 1, true),
			want: []byte{0xAA, 0x21, 0x09, 0x21, 0x01, 0x01, 0x06, 0x20, 0x08, 0x01, 0x10, 0x15, 0x1A, 0x19, 0x08, 0x01, 0x12, 0x15, 0x08, 0x01, 0x10, 0x00, 0x18, 0x00, 0x20, 0x8B, 0x02, 0x28, 0x13, 0x30, 0xE6, 0x01, 0x38, 0x8E, 0x0A, 0x40, 0x05, 0x48, 0x00, 0xB6, 0x53},
		},
		{
			name: "content",
			got:  BuildContentPage(0x0A, 0x16, Page{Number: 0, Text: "Hello G2"}),
			want: []byte{0xAA, 0x21, 0x0A, 0x17, 0x01, 0x01, 0x06, 0x20, 0x08, 0x03, 0x10, 0x16, 0x2A, 0x0F, 0x08, 0x00, 0x10, 0x0A, 0x1A, 0x09, 0x0A, 0x48, 0x65, 0x6C, 0x6C, 0x6F, 0x20, 0x47, 0x32, 0xDE, 0x5B},
		},
		{
			name: "marker",
			got:  BuildMarker(0x14, 0x20),
			want: []byte{0xAA, 0x21, 0x14, 0x0D, 0x01, 0x01, 0x06, 0x20, 0x08, 0xFF, 0x01, 0x10, 0x20, 0x6A, 0x04, 0x08, 0x00, 0x10, 0x06, 0x8E, 0x15},
		},
		{
			name: "sync",
			got:  BuildSync(0x17, 0x23),
			want: []byte{0xAA, 0x21, 0x17, 0x08, 0x01, 0x01, 0x80, 0x00, 0x08, 0x0E, 0x10, 0x23, 0x6A, 0x00, 0x2A, 0xEC},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !bytes.Equal(tt.got, tt.want) {
				t.Fatalf("packet = % X, want % X", tt.got, tt.want)
			}
		})
	}
}

func TestDisplayConfigContainsVerifiedBlob(t *testing.T) {
	packet := BuildDisplayConfig(0x08, 0x14)
	payloadEnd := len(packet) - packetCRCLength
	if !bytes.Equal(packet[payloadEnd-len(displayConfigBlob):payloadEnd], displayConfigBlob) {
		t.Fatal("display config blob changed")
	}
}
