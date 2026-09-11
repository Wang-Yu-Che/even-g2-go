package protocol

import (
	"encoding/binary"
	"errors"
	"image"
)

const (
	LensWidth       = 576
	LensHeight      = 288
	ImageTileWidth  = 288
	ImageTileHeight = 144
	bmpPixelOffset  = 14 + 40 + 64
)

var ErrInvalidImageSize = errors.New("image dimensions must be positive")

// ImageTile is one quadrant of a full-lens image in reading order.
type ImageTile struct {
	ID   int
	Name string
	X    int
	Y    int
	BMP  []byte
}

// EncodeBMP4 builds a bottom-up Windows BMP with a 16-level grayscale palette.
func EncodeBMP4(width, height int, pixel func(x, y int) uint8) ([]byte, error) {
	if width <= 0 || height <= 0 {
		return nil, ErrInvalidImageSize
	}
	rowStride := ((width+1)/2 + 3) &^ 3
	pixelSize := rowStride * height
	data := make([]byte, bmpPixelOffset+pixelSize)
	data[0], data[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(data[2:6], uint32(len(data)))
	binary.LittleEndian.PutUint32(data[10:14], bmpPixelOffset)
	binary.LittleEndian.PutUint32(data[14:18], 40)
	binary.LittleEndian.PutUint32(data[18:22], uint32(width))
	binary.LittleEndian.PutUint32(data[22:26], uint32(height))
	binary.LittleEndian.PutUint16(data[26:28], 1)
	binary.LittleEndian.PutUint16(data[28:30], 4)
	binary.LittleEndian.PutUint32(data[34:38], uint32(pixelSize))
	binary.LittleEndian.PutUint32(data[46:50], 16)
	for index := 0; index < 16; index++ {
		value := byte(index * 17)
		offset := 54 + index*4
		data[offset], data[offset+1], data[offset+2] = value, value, value
	}

	for bmpRow := 0; bmpRow < height; bmpRow++ {
		y := height - 1 - bmpRow
		rowOffset := bmpPixelOffset + bmpRow*rowStride
		for x := 0; x < width; x += 2 {
			high := pixel(x, y) & 0x0F
			low := uint8(0)
			if x+1 < width {
				low = pixel(x+1, y) & 0x0F
			}
			data[rowOffset+x/2] = high<<4 | low
		}
	}
	return data, nil
}

// RenderImageTiles scales an image to fit the lens, letterboxes it in black,
// and returns the four BMP tiles expected by EvenHub.
func RenderImageTiles(source image.Image) ([]ImageTile, error) {
	if source == nil || source.Bounds().Dx() <= 0 || source.Bounds().Dy() <= 0 {
		return nil, ErrInvalidImageSize
	}
	bounds := source.Bounds()
	sourceWidth, sourceHeight := bounds.Dx(), bounds.Dy()
	destinationWidth, destinationHeight := LensWidth, LensHeight
	if sourceWidth*LensHeight > sourceHeight*LensWidth {
		destinationHeight = sourceHeight * LensWidth / sourceWidth
	} else {
		destinationWidth = sourceWidth * LensHeight / sourceHeight
	}
	offsetX := (LensWidth - destinationWidth) / 2
	offsetY := (LensHeight - destinationHeight) / 2

	pixel := func(x, y int) uint8 {
		if x < offsetX || x >= offsetX+destinationWidth || y < offsetY || y >= offsetY+destinationHeight {
			return 0
		}
		sourceX := bounds.Min.X + (x-offsetX)*sourceWidth/destinationWidth
		sourceY := bounds.Min.Y + (y-offsetY)*sourceHeight/destinationHeight
		red, green, blue, _ := source.At(sourceX, sourceY).RGBA()
		gray := (299*uint64(red) + 587*uint64(green) + 114*uint64(blue)) / 1000
		return uint8((gray*15 + 32767) / 65535)
	}

	definitions := []struct {
		id   int
		name string
		x, y int
	}{
		{10, "t0", 0, 0},
		{11, "t1", ImageTileWidth, 0},
		{12, "t2", 0, ImageTileHeight},
		{13, "t3", ImageTileWidth, ImageTileHeight},
	}
	tiles := make([]ImageTile, 0, len(definitions))
	for _, definition := range definitions {
		bmp, err := EncodeBMP4(ImageTileWidth, ImageTileHeight, func(x, y int) uint8 {
			return pixel(definition.x+x, definition.y+y)
		})
		if err != nil {
			return nil, err
		}
		tiles = append(tiles, ImageTile{ID: definition.id, Name: definition.name, X: definition.x, Y: definition.y, BMP: bmp})
	}
	return tiles, nil
}
