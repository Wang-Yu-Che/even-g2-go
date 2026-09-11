package protocol

import "strings"

const (
	TeleprompterColumns      = 25
	TeleprompterLinesPerPage = 10
	TeleprompterMinimumPages = 14
	teleprompterColumnUnits  = 10
)

var displayConfigBlob = []byte{
	0x08, 0x01, 0x12, 0x13, 0x08, 0x02, 0x10, 0x90, 0x4E, 0x1D, 0x00, 0xE0,
	0x94, 0x44, 0x25, 0x00, 0x00, 0x00, 0x00, 0x28, 0x00, 0x30, 0x00, 0x12,
	0x13, 0x08, 0x03, 0x10, 0x0D, 0x0F, 0x1D, 0x00, 0x40, 0x8D, 0x44, 0x25,
	0x00, 0x00, 0x00, 0x00, 0x28, 0x00, 0x30, 0x00, 0x12, 0x12, 0x08, 0x04,
	0x10, 0x00, 0x1D, 0x00, 0x00, 0x88, 0x42, 0x25, 0x00, 0x00, 0x00, 0x00,
	0x28, 0x00, 0x30, 0x00, 0x12, 0x12, 0x08, 0x05, 0x10, 0x00, 0x1D, 0x00,
	0x00, 0x92, 0x42, 0x25, 0x00, 0x00, 0xA2, 0x42, 0x28, 0x00, 0x30, 0x00,
	0x12, 0x12, 0x08, 0x06, 0x10, 0x00, 0x1D, 0x00, 0x00, 0xC6, 0x42, 0x25,
	0x00, 0x00, 0xC4, 0x42, 0x28, 0x00, 0x30, 0x00, 0x18, 0x00,
}

// Page is one ten-line teleprompter page.
type Page struct {
	Number int
	Text   string
}

// BuildDisplayConfig builds the fixed display configuration packet.
func BuildDisplayConfig(sequence byte, messageID int) []byte {
	payload := append([]byte{0x08, 0x02, 0x10}, EncodeVarint(uint64(messageID))...)
	payload = append(payload, 0x22, 0x6A)
	payload = append(payload, displayConfigBlob...)
	return BuildPacket(sequence, 0x0E, 0x20, payload)
}

// BuildTeleprompterInit builds the teleprompter initialization packet.
func BuildTeleprompterInit(sequence byte, messageID, totalLines int, manual bool) []byte {
	mode := byte(0x01)
	if manual {
		mode = 0x00
	}
	contentHeight := totalLines * 2665 / 140
	if contentHeight < 1 {
		contentHeight = 1
	}
	display := []byte{0x08, 0x01, 0x10, 0x00, 0x18, 0x00, 0x20, 0x8B, 0x02, 0x28}
	display = append(display, EncodeVarint(uint64(contentHeight))...)
	display = append(display, 0x30, 0xE6, 0x01, 0x38, 0x8E, 0x0A, 0x40, 0x05, 0x48, mode)
	settings := append([]byte{0x08, 0x01, 0x12, byte(len(display))}, display...)
	payload := append([]byte{0x08, 0x01, 0x10}, EncodeVarint(uint64(messageID))...)
	payload = append(payload, 0x1A, byte(len(settings)))
	payload = append(payload, settings...)
	return BuildPacket(sequence, 0x06, 0x20, payload)
}

// BuildContentPage builds one teleprompter content packet.
func BuildContentPage(sequence byte, messageID int, page Page) []byte {
	text := append([]byte{0x0A}, []byte(page.Text)...)
	inner := append([]byte{0x08}, EncodeVarint(uint64(page.Number))...)
	inner = append(inner, 0x10, 0x0A, 0x1A)
	inner = append(inner, EncodeVarint(uint64(len(text)))...)
	inner = append(inner, text...)
	content := append([]byte{0x2A}, EncodeVarint(uint64(len(inner)))...)
	content = append(content, inner...)
	payload := append([]byte{0x08, 0x03, 0x10}, EncodeVarint(uint64(messageID))...)
	payload = append(payload, content...)
	return BuildPacket(sequence, 0x06, 0x20, payload)
}

// BuildMarker builds the mid-stream marker packet.
func BuildMarker(sequence byte, messageID int) []byte {
	payload := append([]byte{0x08, 0xFF, 0x01, 0x10}, EncodeVarint(uint64(messageID))...)
	payload = append(payload, 0x6A, 0x04, 0x08, 0x00, 0x10, 0x06)
	return BuildPacket(sequence, 0x06, 0x20, payload)
}

// BuildSync builds the packet that activates teleprompter rendering.
func BuildSync(sequence byte, messageID int) []byte {
	payload := append([]byte{0x08, 0x0E, 0x10}, EncodeVarint(uint64(messageID))...)
	payload = append(payload, 0x6A, 0x00)
	return BuildPacket(sequence, 0x80, 0x00, payload)
}

// FormatTeleprompter wraps text to 25 characters by 10 lines and pads it to at
// least 14 pages, matching OpenEvenSdk G2Protocol.formatText.
func FormatTeleprompter(input string) []Page {
	text := strings.ReplaceAll(input, "\\n", "\n")
	lines := make([]string, 0, TeleprompterLinesPerPage)
	for _, sourceLine := range strings.Split(text, "\n") {
		if strings.TrimSpace(sourceLine) == "" {
			lines = append(lines, "")
			continue
		}
		current := ""
		currentWidth := 0
		for _, word := range strings.Split(sourceLine, " ") {
			if word == "" {
				continue
			}
			wordWidth := teleprompterTextWidth(word)
			if currentWidth > 0 && currentWidth+teleprompterColumnUnits+wordWidth > TeleprompterColumns*teleprompterColumnUnits {
				if trimmed := strings.TrimSpace(current); trimmed != "" {
					lines = append(lines, trimmed)
				}
				current = ""
				currentWidth = 0
			}
			for _, character := range word {
				width := teleprompterCharacterWidth(character)
				if currentWidth+width > TeleprompterColumns*teleprompterColumnUnits {
					lines = append(lines, strings.TrimSpace(current))
					current = ""
					currentWidth = 0
				}
				current += string(character)
				currentWidth += width
			}
			current += " "
			currentWidth += teleprompterColumnUnits
		}
		if trimmed := strings.TrimSpace(current); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	for len(lines) < TeleprompterLinesPerPage {
		lines = append(lines, " ")
	}

	pages := make([]Page, 0, TeleprompterMinimumPages)
	for offset := 0; offset < len(lines); offset += TeleprompterLinesPerPage {
		end := min(offset+TeleprompterLinesPerPage, len(lines))
		pageLines := append([]string(nil), lines[offset:end]...)
		for len(pageLines) < TeleprompterLinesPerPage {
			pageLines = append(pageLines, " ")
		}
		pages = append(pages, Page{Number: len(pages), Text: strings.Join(pageLines, "\n") + " \n"})
	}
	blank := strings.Join(makeBlankLines(), "\n") + " \n"
	for len(pages) < TeleprompterMinimumPages {
		pages = append(pages, Page{Number: len(pages), Text: blank})
	}
	return pages
}

func teleprompterTextWidth(text string) int {
	width := 0
	for _, character := range text {
		width += teleprompterCharacterWidth(character)
	}
	return width
}

func teleprompterCharacterWidth(character rune) int {
	if character <= 0x7F {
		return teleprompterColumnUnits
	}
	// G2 renders CJK glyphs slightly wider than two Latin columns. Using 2.1
	// keeps the twelfth glyph from being clipped at the right edge.
	return 21
}

// TeleprompterInputLineCount returns the unwrapped source line count.
func TeleprompterInputLineCount(input string) int {
	return len(strings.Split(strings.ReplaceAll(input, "\\n", "\n"), "\n"))
}

func makeBlankLines() []string {
	lines := make([]string, TeleprompterLinesPerPage)
	for index := range lines {
		lines[index] = " "
	}
	return lines
}
