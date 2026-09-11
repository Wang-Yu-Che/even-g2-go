package g2

import "sync"

// State is the lifecycle state of a G2 arm.
type State int

const (
	Disconnected State = iota
	Scanning
	Connecting
	Authenticating
	Ready
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
	default:
		return "Disconnected"
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
