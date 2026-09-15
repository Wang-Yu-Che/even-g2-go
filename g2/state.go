package g2

import (
	"context"
	"encoding/json"
	"sync"
)

// State is the lifecycle state of a G2 arm.
type State int

const (
	Disconnected State = iota
	Scanning
	Connecting
	Authenticating
	Ready
	Reconnecting
)

func (s State) String() string {
	switch s {
	case Scanning:
		return "Scanning"
	case Connecting:
		return "Connecting"
	case Authenticating:
		return "Authenticating"
	case Ready:
		return "Ready"
	case Reconnecting:
		return "Reconnecting"
	default:
		return "Disconnected"
	}
}

// MarshalJSON encodes lifecycle states as stable readable names.
func (s State) MarshalJSON() ([]byte, error) { return json.Marshal(s.String()) }

// Capabilities reports features verified by this SDK for G2.
type Capabilities struct {
	Text           bool `json:"text"`
	NativeText     bool `json:"nativeText"`
	NativeList     bool `json:"nativeList"`
	Image          bool `json:"image"`
	MicrophoneLC3  bool `json:"microphoneLC3"`
	InputEvents    bool `json:"inputEvents"`
	DeviceSettings bool `json:"deviceSettings"`
	Brightness     bool `json:"brightness"`
	HeadUp         bool `json:"headUp"`
	ScreenPosition bool `json:"screenPosition"`
	Dashboard      bool `json:"dashboard"`
}

// Status is an immutable snapshot of the client runtime.
type Status struct {
	Device         Device         `json:"device"`
	Left           State          `json:"left"`
	Right          State          `json:"right"`
	Ready          bool           `json:"ready"`
	AudioAvailable bool           `json:"audioAvailable"`
	Capabilities   Capabilities   `json:"capabilities"`
	Settings       DeviceSettings `json:"settings"`
	SettingsKnown  bool           `json:"settingsKnown"`
}

// Capabilities returns the features currently available on the connected G2.
func (c *Client) Capabilities() Capabilities { return c.Status().Capabilities }

// Status returns the current client runtime snapshot.
func (c *Client) Status() Status {
	left, right := c.Left.State(), c.Right.State()
	audioAvailable := c.audioAvailable.Load()
	c.settingsMu.RLock()
	settings := c.settings
	c.settingsMu.RUnlock()
	return Status{
		Device: c.device, Left: left, Right: right, Ready: left == Ready && right == Ready,
		AudioAvailable: audioAvailable,
		Capabilities: Capabilities{
			Text: true, NativeText: true, NativeList: true, Image: true, MicrophoneLC3: audioAvailable, InputEvents: true,
			DeviceSettings: true, Brightness: true, HeadUp: true, ScreenPosition: true, Dashboard: true,
		},
		Settings: settings, SettingsKnown: c.settingsKnown.Load(),
	}
}

// SubscribeStatus publishes the current snapshot and subsequent state changes.
func (c *Client) SubscribeStatus(ctx context.Context) <-chan Status {
	updates := make(chan Status, 8)
	c.subscriberMu.Lock()
	id := c.nextSubscriberID
	c.nextSubscriberID++
	c.statusSubscribers[id] = updates
	c.subscriberMu.Unlock()
	updates <- c.Status()
	go func() {
		<-ctx.Done()
		c.subscriberMu.Lock()
		delete(c.statusSubscribers, id)
		close(updates)
		c.subscriberMu.Unlock()
	}()
	return updates
}

func (c *Client) publishStatus() {
	status := c.Status()
	c.subscriberMu.Lock()
	defer c.subscriberMu.Unlock()
	for _, updates := range c.statusSubscribers {
		select {
		case updates <- status:
		default:
		}
	}
}

// Connection tracks one arm independently.
type Connection struct {
	mu    sync.RWMutex
	state State
}

// State returns the current arm state.
func (c *Connection) State() State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

func (c *Connection) setState(state State) {
	c.mu.Lock()
	c.state = state
	c.mu.Unlock()
}
