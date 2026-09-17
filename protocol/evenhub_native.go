package protocol

import "fmt"

const EvenHubMaxContainerNameBytes = 14

// EvenHubGeometry describes one native container on the 576x288 lens.
type EvenHubGeometry struct {
	X, Y, Width, Height int
}

// EvenHubTextStyle contains the verified text-container presentation fields.
type EvenHubTextStyle struct {
	BorderWidth, BorderColor, BorderRadius, PaddingLength int
}

// EvenHubImage describes one bitmap container on a native page.
type EvenHubImage struct {
	ID, X, Y, Width, Height int
	Name                    string
	BMP                     []byte
}

var EvenHubFullLens = EvenHubGeometry{Width: 576, Height: 288}

func BuildEvenHubCreateList(name string, rows []string, magic int) ([]byte, error) {
	object, err := evenHubListObject(1, name, rows, true, true, EvenHubFullLens)
	if err != nil {
		return nil, err
	}
	create := protoUint(1, 1)
	create = append(create, protoMessage(2, object)...)
	create = append(create, protoUint(5, 10000)...)
	payload := protoUint(2, magic) // Cmd=create is zero and omitted by proto3.
	return append(payload, protoMessage(3, create)...), nil
}

func BuildEvenHubRebuildList(name string, rows []string, magic int) ([]byte, error) {
	object, err := evenHubListObject(1, name, rows, true, true, EvenHubFullLens)
	if err != nil {
		return nil, err
	}
	rebuild := protoUint(1, 1)
	rebuild = append(rebuild, protoMessage(2, object)...)
	payload := protoUint(1, 7)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(7, rebuild)...), nil
}

func BuildEvenHubRebuildText(name, content string, magic int) ([]byte, error) {
	return BuildEvenHubRebuildStyledText(name, content, magic, EvenHubFullLens, EvenHubTextStyle{})
}

// BuildEvenHubRebuildStyledText rebuilds a text container with geometry and
// border fields matching the native G2 container schema.
func BuildEvenHubRebuildStyledText(name, content string, magic int, geometry EvenHubGeometry, style EvenHubTextStyle) ([]byte, error) {
	object, err := evenHubTextObject(1, name, content, true, geometry, style)
	if err != nil {
		return nil, err
	}
	rebuild := protoUint(1, 1)
	rebuild = append(rebuild, protoMessage(3, object)...)
	payload := protoUint(1, 7)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(7, rebuild)...), nil
}

func BuildEvenHubTextUpgrade(name, content string, magic int) ([]byte, error) {
	if err := validateEvenHubName(name); err != nil {
		return nil, err
	}
	if content == "" {
		content = "·"
	}
	data := []byte(content)
	upgrade := protoUint(1, 1)
	upgrade = append(upgrade, protoString(2, name)...)
	upgrade = append(upgrade, protoUint(4, len(data))...)
	upgrade = append(upgrade, protoBytes(5, data)...)
	payload := protoUint(1, 5)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(9, upgrade)...), nil
}

// BuildEvenHubCreateImages creates the four full-lens image containers.
func BuildEvenHubCreateImages(tiles []ImageTile, magic int) ([]byte, error) {
	create := protoUint(1, len(tiles))
	for _, tile := range tiles {
		if err := validateEvenHubName(tile.Name); err != nil {
			return nil, err
		}
		object := protoUint(1, tile.X)
		object = append(object, protoUint(2, tile.Y)...)
		object = append(object, protoUint(3, ImageTileWidth)...)
		object = append(object, protoUint(4, ImageTileHeight)...)
		object = append(object, protoUint(5, tile.ID)...)
		object = append(object, protoString(6, tile.Name)...)
		create = append(create, protoMessage(4, object)...)
	}
	create = append(create, protoUint(5, 10000)...)
	payload := protoUint(2, magic)
	return append(payload, protoMessage(3, create)...), nil
}

// BuildEvenHubCreateTextImage creates a mixed page containing one text and one
// image container. The image pixels are sent separately with Cmd=3.
func BuildEvenHubCreateTextImage(textName, content string, textGeometry EvenHubGeometry, style EvenHubTextStyle, image EvenHubImage, magic int) ([]byte, error) {
	return BuildEvenHubCreateTextImages(textName, content, textGeometry, style, []EvenHubImage{image}, magic)
}

// BuildEvenHubCreateTextImages creates one text container and multiple image
// containers. Callers must send each image's pixels serially after creation.
func BuildEvenHubCreateTextImages(textName, content string, textGeometry EvenHubGeometry, style EvenHubTextStyle, images []EvenHubImage, magic int) ([]byte, error) {
	text, imageObjects, err := evenHubTextImageObjects(textName, content, textGeometry, style, images)
	if err != nil {
		return nil, err
	}
	create := protoUint(1, 1+len(images))
	create = append(create, protoMessage(3, text)...)
	for _, imageObject := range imageObjects {
		create = append(create, protoMessage(4, imageObject)...)
	}
	create = append(create, protoUint(5, 10000)...)
	payload := protoUint(2, magic)
	return append(payload, protoMessage(3, create)...), nil
}

// BuildEvenHubRebuildTextImages replaces the active page without emitting a
// system-exit event. Image pixels are sent separately after the rebuild.
func BuildEvenHubRebuildTextImages(textName, content string, textGeometry EvenHubGeometry, style EvenHubTextStyle, images []EvenHubImage, magic int) ([]byte, error) {
	text, imageObjects, err := evenHubTextImageObjects(textName, content, textGeometry, style, images)
	if err != nil {
		return nil, err
	}
	rebuild := protoUint(1, 1+len(images))
	rebuild = append(rebuild, protoMessage(3, text)...)
	for _, imageObject := range imageObjects {
		rebuild = append(rebuild, protoMessage(4, imageObject)...)
	}
	payload := protoUint(1, 7)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(7, rebuild)...), nil
}

func evenHubTextImageObjects(textName, content string, textGeometry EvenHubGeometry, style EvenHubTextStyle, images []EvenHubImage) ([]byte, [][]byte, error) {
	text, err := evenHubTextObject(1, textName, content, true, textGeometry, style)
	if err != nil {
		return nil, nil, err
	}
	if len(images) == 0 || len(images) > 4 {
		return nil, nil, fmt.Errorf("invalid EvenHub image container count: %d", len(images))
	}
	objects := make([][]byte, 0, len(images))
	for _, image := range images {
		if err := validateEvenHubName(image.Name); err != nil {
			return nil, nil, err
		}
		if image.ID < 1 || image.Width < 1 || image.Height < 1 {
			return nil, nil, fmt.Errorf("invalid EvenHub image container: %+v", image)
		}
		object := protoUint(1, image.X)
		object = append(object, protoUint(2, image.Y)...)
		object = append(object, protoUint(3, image.Width)...)
		object = append(object, protoUint(4, image.Height)...)
		object = append(object, protoUint(5, image.ID)...)
		object = append(object, protoString(6, image.Name)...)
		objects = append(objects, object)
	}
	return text, objects, nil
}

// BuildEvenHubImageFragment builds one Cmd=3 ImageRawData message.
func BuildEvenHubImageFragment(containerID int, name string, sessionID, totalSize, fragmentIndex int, data []byte, magic int) ([]byte, error) {
	if err := validateEvenHubName(name); err != nil {
		return nil, err
	}
	imageData := protoUint(1, containerID)
	imageData = append(imageData, protoString(2, name)...)
	imageData = append(imageData, protoUint(3, sessionID)...)
	imageData = append(imageData, protoUint(4, totalSize)...)
	imageData = append(imageData, protoUint(6, fragmentIndex)...)
	imageData = append(imageData, protoUint(7, len(data))...)
	imageData = append(imageData, protoBytes(8, data)...)
	payload := protoUint(1, 3)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(5, imageData)...), nil
}

// BuildEvenHubShutdown builds Cmd=9 for the active page.
func BuildEvenHubShutdown(magic int) []byte {
	payload := protoUint(1, 9)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(11, nil)...)
}

func evenHubListObject(id int, name string, rows []string, capture, selectBorder bool, geometry EvenHubGeometry) ([]byte, error) {
	if err := validateEvenHubName(name); err != nil {
		return nil, err
	}
	items := protoUint(1, len(rows))
	items = append(items, protoUint(2, geometry.Width)...)
	if selectBorder {
		items = append(items, protoUint(3, 1)...)
	}
	for _, row := range rows {
		if row == "" {
			row = "·"
		}
		items = append(items, protoString(4, row)...)
	}
	object := evenHubGeometryFields(geometry)
	object = append(object, protoUint(9, id)...)
	object = append(object, protoString(10, name)...)
	object = append(object, protoMessage(11, items)...)
	if capture {
		object = append(object, protoUint(12, 1)...)
	}
	return object, nil
}

func evenHubTextObject(id int, name, content string, capture bool, geometry EvenHubGeometry, style EvenHubTextStyle) ([]byte, error) {
	if err := validateEvenHubName(name); err != nil {
		return nil, err
	}
	if style.BorderWidth < 0 || style.BorderWidth > 5 || style.BorderColor < 0 || style.BorderColor > 15 ||
		style.BorderRadius < 0 || style.BorderRadius > 10 || style.PaddingLength < 0 || style.PaddingLength > 32 {
		return nil, fmt.Errorf("invalid EvenHub text style: %+v", style)
	}
	object := evenHubGeometryFields(geometry)
	object = append(object, protoUint(5, style.BorderWidth)...)
	object = append(object, protoUint(6, style.BorderColor)...)
	object = append(object, protoUint(7, style.BorderRadius)...)
	object = append(object, protoUint(8, style.PaddingLength)...)
	object = append(object, protoUint(9, id)...)
	object = append(object, protoString(10, name)...)
	if capture {
		object = append(object, protoUint(11, 1)...)
	}
	object = append(object, protoString(12, content)...)
	return object, nil
}

func evenHubGeometryFields(geometry EvenHubGeometry) []byte {
	payload := protoUint(1, geometry.X)
	payload = append(payload, protoUint(2, geometry.Y)...)
	payload = append(payload, protoUint(3, geometry.Width)...)
	return append(payload, protoUint(4, geometry.Height)...)
}

func validateEvenHubName(name string) error {
	if len([]byte(name)) > EvenHubMaxContainerNameBytes {
		return fmt.Errorf("EvenHub container name %q exceeds %d bytes", name, EvenHubMaxContainerNameBytes)
	}
	return nil
}

func protoString(field int, value string) []byte {
	if value == "" {
		return nil
	}
	return protoBytes(field, []byte(value))
}

func protoBytes(field int, value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	return protoMessage(field, value)
}
