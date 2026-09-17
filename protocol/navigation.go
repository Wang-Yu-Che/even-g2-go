package protocol

const (
	NavigationCommandHeartbeat          = 0
	NavigationCommandStart              = 5
	NavigationCommandBasicInfo          = 7
	NavigationCommandExit               = 12
	NavigationEventExit                 = 13
	NavigationEventReviewChanged        = 14
	NavigationEventCompassChanged       = 15
	NavigationEventCalibrationStarted   = 16
	NavigationEventCalibrationCompleted = 17
)

type NavigationEvent struct {
	Command int
	Heading int
}

func BuildNavigationStart(magic int) []byte {
	return navigationCommand(NavigationCommandStart, magic)
}

func BuildNavigationExit(magic int) []byte {
	return navigationCommand(NavigationCommandExit, magic)
}

func BuildNavigationHeartbeat(magic int) []byte {
	return navigationCommand(NavigationCommandHeartbeat, magic)
}

func navigationCommand(command, magic int) []byte {
	payload := protoUintPresent(1, command)
	return append(payload, protoUint(2, magic)...)
}

func ParseNavigationEvent(payload []byte) (NavigationEvent, error) {
	event := NavigationEvent{Command: -1}
	var compass []byte
	err := walkProto(payload, func(field, wire int, value uint64, data []byte) error {
		if wire == 0 && field == 1 {
			event.Command = int(value)
		} else if wire == 2 && field == 10 {
			compass = data
		}
		return nil
	})
	if err != nil || compass == nil {
		return event, err
	}
	err = walkProto(compass, func(field, wire int, value uint64, _ []byte) error {
		if wire == 0 && field == 1 {
			event.Heading = int(value)
		}
		return nil
	})
	return event, err
}
