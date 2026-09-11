package protocol

import (
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"testing"
)

func TestEncodeBMP4HeaderPaletteAndBottomUpPixels(t *testing.T) {
	bmp, err := EncodeBMP4(3, 2, func(x, y int) uint8 { return uint8(y*3 + x) })
	if err != nil {
		t.Fatal(err)
	}
	if string(bmp[:2]) != "BM" || binary.LittleEndian.Uint32(bmp[10:14]) != bmpPixelOffset {
		t.Fatalf("invalid BMP header: % X", bmp[:14])
	}
	if binary.LittleEndian.Uint32(bmp[18:22]) != 3 || binary.LittleEndian.Uint32(bmp[22:26]) != 2 || binary.LittleEndian.Uint16(bmp[28:30]) != 4 {
		t.Fatalf("invalid BMP geometry")
	}
	if got := bmp[54+15*4 : 54+15*4+4]; got[0] != 255 || got[1] != 255 || got[2] != 255 || got[3] != 0 {
		t.Fatalf("palette white = % X", got)
	}
	// Bottom source row y=1 is stored first: pixels 3,4,5 and one zero nibble.
	if bmp[bmpPixelOffset] != 0x34 || bmp[bmpPixelOffset+1] != 0x50 {
		t.Fatalf("bottom BMP row = % X", bmp[bmpPixelOffset:bmpPixelOffset+4])
	}
}

func TestRenderImageTilesFullLensAndLetterbox(t *testing.T) {
	source := image.NewGray(image.Rect(0, 0, 2, 1))
	source.SetGray(0, 0, color.Gray{Y: 255})
	source.SetGray(1, 0, color.Gray{Y: 255})
	tiles, err := RenderImageTiles(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(tiles) != 4 {
		t.Fatalf("tile count = %d, want 4", len(tiles))
	}
	for index, tile := range tiles {
		if tile.ID != 10+index || len(tile.BMP) != bmpPixelOffset+ImageTileWidth/2*ImageTileHeight {
			t.Fatalf("tile %d = %#v, BMP bytes=%d", index, tile, len(tile.BMP))
		}
	}
	// A 2:1 source exactly fills the 2:1 lens, so the top-left pixel is white.
	rowStride := ImageTileWidth / 2
	topRowOffset := bmpPixelOffset + (ImageTileHeight-1)*rowStride
	if tiles[0].BMP[topRowOffset]>>4 != 15 {
		t.Fatalf("top-left pixel was not white")
	}

	portrait := image.NewGray(image.Rect(0, 0, 1, 2))
	portrait.SetGray(0, 0, color.Gray{Y: 255})
	portrait.SetGray(0, 1, color.Gray{Y: 255})
	portraitTiles, err := RenderImageTiles(portrait)
	if err != nil {
		t.Fatal(err)
	}
	if portraitTiles[0].BMP[topRowOffset]>>4 != 0 {
		t.Fatalf("portrait image should have a black left letterbox")
	}
}

func TestImageRejectsInvalidSize(t *testing.T) {
	if _, err := EncodeBMP4(0, 1, func(int, int) uint8 { return 0 }); !errors.Is(err, ErrInvalidImageSize) {
		t.Fatalf("error = %v", err)
	}
	if _, err := RenderImageTiles(nil); !errors.Is(err, ErrInvalidImageSize) {
		t.Fatalf("error = %v", err)
	}
}
