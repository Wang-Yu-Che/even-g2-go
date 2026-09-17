package g2

import (
	"testing"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

func TestSetMenuWritesRightArmAndRoutesSelection(t *testing.T) {
	transport := &recordingTransport{}
	client := testClient(transport)
	client.setState(Ready)
	item := MenuItem{PackageName: "com.example.weather", Name: "Weather"}
	if err := client.SetMenu(t.Context(), []MenuItem{item}); err != nil {
		t.Fatal(err)
	}

	writes := transport.writesSnapshot()
	if len(writes) != 1 || writes[0].arm != ble.Right {
		t.Fatalf("menu writes = %#v", writes)
	}
	for _, write := range writes {
		if len(write.data) < 8 || write.data[6] != protocol.MenuServiceID {
			t.Fatalf("menu frame = % X", write.data)
		}
	}

	events := client.SubscribeEvents(t.Context())
	appID := protocol.MenuAppID(item.PackageName)
	menuPayload := []byte{0x08, 0x11, 0xA2, 0x01, 0x03, 0x08, byte(appID&0x7F) | 0x80, byte(appID >> 7)}
	packet := protocol.BuildPacket(1, protocol.EvenHubServiceID, 0x00, menuPayload)
	client.HandleNotification(ble.Left, packet)
	client.HandleNotification(ble.Right, packet)

	select {
	case event := <-events:
		if event.Kind != protocol.EvenHubEventMenu || event.AppID != appID || event.PackageName != item.PackageName {
			t.Fatalf("menu event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for menu selection")
	}
	select {
	case duplicate := <-events:
		t.Fatalf("duplicate menu event = %#v", duplicate)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestMenuIsRestoredAfterAuthentication(t *testing.T) {
	transport := &recordingTransport{}
	client := testClient(transport)
	client.menuItems = []protocol.MenuItem{{PackageName: "com.example.weather", Name: "Weather"}}
	if err := client.Authenticate(t.Context()); err != nil {
		t.Fatal(err)
	}
	writes := transport.writesSnapshot()
	if len(writes) != 15 {
		t.Fatalf("write count = %d, want 14 auth + 1 menu", len(writes))
	}
	if writes[14].data[6] != protocol.MenuServiceID {
		t.Fatalf("restored menu service = %02X", writes[14].data[6])
	}
}
