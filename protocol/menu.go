package protocol

import (
	"fmt"
	"unicode/utf16"
)

const (
	MenuServiceID       byte = 0x03
	MenuCommandSendInfo      = 0
	MenuMinItems             = 5
	MenuMaxItems             = 10
	MenuMaxNameRunes         = 15
)

var menuPlaceholderAppIDs = [...]int{10535, 10536, 10537, 10538, 10539}

// MenuItem describes one third-party entry in the glasses dashboard menu.
type MenuItem struct {
	PackageName string
	Name        string
	Running     bool
}

// MenuAppID deterministically maps a package name to MentraOS's third-party range.
func MenuAppID(packageName string) int {
	var hash int32
	for _, unit := range utf16.Encode([]rune(packageName)) {
		hash = hash*31 + int32(unit)
	}
	value := int64(hash)
	if value < 0 {
		value = -value
	}
	return 10029 + int(value%506)
}

// BuildMenuInfo builds APP_SEND_MENU_INFO and returns its app ID mapping.
func BuildMenuInfo(magic int, items []MenuItem) ([]byte, map[int]string, error) {
	if len(items) > MenuMaxItems-1 {
		items = items[:MenuMaxItems-1]
	}

	type wireItem struct {
		name    string
		appID   int
		builtIn bool
	}
	wireItems := []wireItem{{appID: 4, builtIn: true}}
	appIDs := make(map[int]string, len(items))
	for _, item := range items {
		if item.PackageName == "" || item.Name == "" {
			return nil, nil, fmt.Errorf("menu package name and display name must not be empty")
		}
		appID := MenuAppID(item.PackageName)
		appIDs[appID] = item.PackageName
		name := truncateRunes(item.Name, MenuMaxNameRunes)
		prefix := "  "
		if item.Running {
			prefix = "● "
		}
		wireItems = append(wireItems, wireItem{name: prefix + name, appID: appID})
	}
	for len(wireItems) < MenuMinItems {
		index := len(wireItems) - 1
		wireItems = append(wireItems, wireItem{name: "  ---", appID: menuPlaceholderAppIDs[index]})
	}

	menu := protoUint(1, len(wireItems))
	for _, item := range wireItems {
		wire := protoUint(1, 1)
		if item.builtIn {
			wire = protoUintPresent(1, 0)
		} else {
			wire = append(wire, protoUint(2, 1)...)
			wire = append(wire, protoString(3, item.name)...)
		}
		wire = append(wire, protoUint(4, item.appID)...)
		menu = append(menu, protoMessage(2, wire)...)
	}

	payload := protoUintPresent(1, MenuCommandSendInfo)
	payload = append(payload, protoUint(2, magic)...)
	payload = append(payload, protoMessage(3, menu)...)
	return payload, appIDs, nil
}

// AssociateEvenHubApp associates a page command with a dashboard menu app ID.
func AssociateEvenHubApp(payload []byte, appID int) []byte {
	if appID == 0 {
		return payload
	}
	result := append([]byte(nil), payload...)
	return append(result, protoUint(5, appID)...)
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
