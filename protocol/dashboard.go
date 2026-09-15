package protocol

import "fmt"

const DashboardServiceID byte = 0x01

type DashboardWidget byte

const (
	DashboardNews      DashboardWidget = 1
	DashboardStock     DashboardWidget = 2
	DashboardSchedule  DashboardWidget = 3
	DashboardQuicklist DashboardWidget = 4
	DashboardHealth    DashboardWidget = 5
)

func BuildDashboardConfig(magic int, order []DashboardWidget, halfDay, celsius bool) ([]byte, error) {
	if len(order) == 0 || len(order) > 5 {
		return nil, fmt.Errorf("dashboard widget order must contain 1..5 entries")
	}
	seen := make(map[DashboardWidget]bool, len(order))
	packed := make([]byte, len(order))
	for index, widget := range order {
		if widget < DashboardNews || widget > DashboardHealth || seen[widget] {
			return nil, fmt.Errorf("invalid dashboard widget order")
		}
		seen[widget] = true
		packed[index] = byte(widget)
	}
	config := protoUint(1, 4)
	config = append(config, protoUint(2, 3)...)
	config = append(config, protoBytes(3, []byte{1, 2, 3})...)
	config = append(config, protoUint(4, len(order))...)
	config = append(config, protoBytes(5, packed)...)
	config = append(config, protoUint(6, boolInt(halfDay))...)
	temperatureUnit := 2
	if celsius {
		temperatureUnit = 1
	}
	config = append(config, protoUint(7, temperatureUnit)...)
	receive := protoMessage(2, config)
	payload := protoUint(1, 2)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(4, receive)...), nil
}

type DashboardScheduleItem struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	Location     string `json:"location"`
	Time         string `json:"time"`
	EndTimestamp int    `json:"endTimestamp"`
}

func BuildDashboardSchedule(magic int, item DashboardScheduleItem, total, index int) ([]byte, error) {
	if total < 1 || index < 0 || index >= total {
		return nil, fmt.Errorf("invalid dashboard schedule position")
	}
	schedule := protoUint(1, item.ID)
	schedule = append(schedule, protoString(2, item.Title)...)
	schedule = append(schedule, protoString(3, item.Location)...)
	schedule = append(schedule, protoString(4, item.Time)...)
	schedule = append(schedule, protoUint(5, item.EndTimestamp)...)
	list := protoUint(1, total)
	list = append(list, protoUint(2, index)...)
	list = append(list, protoMessage(3, schedule)...)
	list = append(list, protoUint(4, 1)...)
	component := protoMessage(3, list)
	content := protoMessage(2, component)
	receive := protoUint(1, 1)
	receive = append(receive, protoMessage(3, content)...)
	payload := protoUint(1, 2)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(4, receive)...), nil
}

func BuildDashboardScheduleClear(magic int) []byte {
	list := protoUint(4, 1)
	component := protoMessage(3, list)
	content := protoMessage(2, component)
	receive := protoUint(1, 1)
	receive = append(receive, protoMessage(3, content)...)
	payload := protoUint(1, 2)
	payload = append(payload, protoUint(2, magic)...)
	return append(payload, protoMessage(4, receive)...)
}
