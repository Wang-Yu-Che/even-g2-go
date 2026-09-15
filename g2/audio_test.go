package g2

import (
	"testing"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
)

func TestAudioNotificationsAreCopiedAndPublished(t *testing.T) {
	client := testClient(&recordingTransport{})
	frames := client.SubscribeAudio(t.Context())
	notifications := make(chan []byte, 1)
	data := []byte{1, 2, 3}
	client.AttachAudioNotifications(t.Context(), ble.Right, notifications)
	notifications <- data
	select {
	case frame := <-frames:
		data[0] = 9
		if frame.Arm != ble.Right || frame.Data[0] != 1 {
			t.Fatalf("frame = %#v", frame)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for audio frame")
	}
}
