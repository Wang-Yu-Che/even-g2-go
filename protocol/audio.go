package protocol

import "fmt"

const (
	G2AudioPacketBytes     = 205
	G2AudioFrameBytes      = 40
	G2AudioFramesPerPacket = 5
	G2AudioSampleRate      = 16000
	G2AudioFrameSamples    = 160
)

// G2AudioPacket is one microphone notification split into LC3 frames and trailer.
type G2AudioPacket struct {
	Frames  [G2AudioFramesPerPacket][G2AudioFrameBytes]byte
	Trailer [5]byte
	Counter byte
}

// ParseG2AudioPacket splits a 205-byte notification into five LC3 frames.
func ParseG2AudioPacket(data []byte) (G2AudioPacket, error) {
	var packet G2AudioPacket
	if len(data) != G2AudioPacketBytes {
		return packet, fmt.Errorf("G2 audio packet must be %d bytes, got %d", G2AudioPacketBytes, len(data))
	}
	for i := range packet.Frames {
		start := i * G2AudioFrameBytes
		copy(packet.Frames[i][:], data[start:start+G2AudioFrameBytes])
	}
	copy(packet.Trailer[:], data[G2AudioFrameBytes*G2AudioFramesPerPacket:])
	packet.Counter = packet.Trailer[len(packet.Trailer)-1]
	return packet, nil
}

// EvenHubAudioResponse is the Cmd=16/19 microphone control response.
type EvenHubAudioResponse struct {
	Command int
	Magic   int
	Status  int
}

// BuildEvenHubAudioControl builds Cmd=15 AudioCtrCmd.
func BuildEvenHubAudioControl(enabled bool, magic int) []byte {
	action := 0
	if enabled {
		action = 1
	}
	payload := protoUint(1, EvenHubCommandAudioControl)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(18, protoUint(1, action))...)
}

// ParseEvenHubAudioResponse decodes Cmd=16/19 AudioCtrRes.
func ParseEvenHubAudioResponse(payload []byte) (EvenHubAudioResponse, error) {
	response := EvenHubAudioResponse{Command: -1, Magic: -1}
	err := walkProto(payload, func(field, wire int, value uint64, data []byte) error {
		if wire == 0 && field == 1 {
			response.Command = int(value)
		} else if wire == 0 && field == 2 {
			response.Magic = int(value)
		} else if wire == 2 && field == 19 {
			return walkProto(data, func(nestedField, nestedWire int, nestedValue uint64, _ []byte) error {
				if nestedWire == 0 && nestedField == 1 {
					response.Status = int(nestedValue)
				}
				return nil
			})
		}
		return nil
	})
	return response, err
}
