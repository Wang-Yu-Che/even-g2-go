package protocol

import (
	"bytes"
	"testing"
)

func TestBuildMenuInfo(t *testing.T) {
	items := []MenuItem{{PackageName: "com.example.weather", Name: "Weather", Running: true}}
	payload, appIDs, err := BuildMenuInfo(7, items)
	if err != nil {
		t.Fatal(err)
	}
	appID := MenuAppID(items[0].PackageName)
	if appIDs[appID] != items[0].PackageName {
		t.Fatalf("app ID mapping = %#v", appIDs)
	}
	if !bytes.Equal(payload[:6], []byte{0x08, 0x00, 0x10, 0x07, 0x1A, byte(len(payload) - 6)}) {
		t.Fatalf("menu header = % X", payload[:6])
	}
	if !bytes.Contains(payload, []byte("● Weather")) {
		t.Fatalf("running menu label missing from % X", payload)
	}
	if got := bytes.Count(payload, []byte("  ---")); got != 3 {
		t.Fatalf("placeholder count = %d, want 3", got)
	}
}

func TestBuildMenuInfoCapsThirdPartyItems(t *testing.T) {
	items := make([]MenuItem, 12)
	for index := range items {
		items[index] = MenuItem{PackageName: "pkg" + string(rune('a'+index)), Name: "App"}
	}
	_, appIDs, err := BuildMenuInfo(1, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(appIDs) != MenuMaxItems-1 {
		t.Fatalf("third-party item count = %d, want %d", len(appIDs), MenuMaxItems-1)
	}
}
