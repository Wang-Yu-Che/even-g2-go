package g2

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

var ErrAuthenticationFailed = errors.New("G2 authentication failed")

// Client coordinates protocol operations across the two G2 arms.
type Client struct {
	Left  *Connection
	Right *Connection

	transport ble.Transport
	debug     bool
	output    io.Writer
	now       func() time.Time

	authMu         sync.Mutex
	leftToRightGap time.Duration
	afterRightGap  time.Duration
	packetGap      time.Duration
	authWait       time.Duration

	heartbeatMu       sync.Mutex
	heartbeatCancel   context.CancelFunc
	heartbeatWG       sync.WaitGroup
	heartbeatInterval time.Duration
	heartbeatSequence byte
	lifecycleStop     func() bool
	errors            chan error
	events            chan protocol.EvenHubEvent
	notificationMu    sync.Mutex
	notificationStops []context.CancelFunc
	notificationWG    sync.WaitGroup
	disconnects       chan struct{}
	audioFrames       chan AudioFrame
	audioAvailable    bool
	reconnectMu       sync.Mutex
	reconnectCancel   context.CancelFunc
	reconnectWG       sync.WaitGroup

	evenHubMu        sync.Mutex
	evenHubWriteMu   sync.Mutex
	evenHubSequence  byte
	evenHubMagic     int
	evenHubChunkSize int
	pendingAcks      map[int]chan evenHubAck
	evenHubActive    bool

	nativeMu           sync.Mutex
	nativeCreated      bool
	nativeShape        nativeShape
	nativePreludeDelay time.Duration
	nativeCreateDelay  time.Duration
	evenHubAckTimeout  time.Duration

	eventHandlerMu sync.Mutex
	eventHandlers  EventHandlers
	lastTapAt      time.Time
	lastBackAt     time.Time

	imageMu          sync.Mutex
	imagePrimed      bool
	imageWarmed      bool
	imageSession     int
	imageSessionJump bool

	displayMu              sync.Mutex
	displayRefreshMu       sync.Mutex
	displayRefreshCancel   context.CancelFunc
	displayRefreshWG       sync.WaitGroup
	displayRefreshInterval time.Duration
	displayConfigDelay     time.Duration
	displayInitDelay       time.Duration
	displayFirstPageDelay  time.Duration
	displayMarkerDelay     time.Duration
	displayLatePageDelay   time.Duration
	displaySyncDelay       time.Duration
}

// NewClient creates a dual-arm G2 client.
func NewClient(transport ble.Transport, debug bool, output io.Writer) *Client {
	if output == nil {
		output = io.Discard
	}
	return &Client{
		Left:                   &Connection{},
		Right:                  &Connection{},
		transport:              transport,
		debug:                  debug,
		output:                 output,
		now:                    time.Now,
		leftToRightGap:         18 * time.Millisecond,
		afterRightGap:          12 * time.Millisecond,
		packetGap:              100 * time.Millisecond,
		authWait:               500 * time.Millisecond,
		heartbeatInterval:      1500 * time.Millisecond,
		heartbeatSequence:      0xC0,
		errors:                 make(chan error, 1),
		events:                 make(chan protocol.EvenHubEvent, 32),
		disconnects:            make(chan struct{}, 1),
		audioFrames:            make(chan AudioFrame, 64),
		evenHubMagic:           1,
		evenHubChunkSize:       180,
		pendingAcks:            make(map[int]chan evenHubAck),
		nativePreludeDelay:     500 * time.Millisecond,
		nativeCreateDelay:      200 * time.Millisecond,
		evenHubAckTimeout:      3 * time.Second,
		displayRefreshInterval: 11 * time.Second,
		displayConfigDelay:     150 * time.Millisecond,
		displayInitDelay:       300 * time.Millisecond,
		displayFirstPageDelay:  45 * time.Millisecond,
		displayMarkerDelay:     45 * time.Millisecond,
		displayLatePageDelay:   100 * time.Millisecond,
		displaySyncDelay:       45 * time.Millisecond,
	}
}

// Authenticate sends the verified seven-packet handshake to both arms.
func (c *Client) Authenticate(ctx context.Context) error {
	c.authMu.Lock()
	defer c.authMu.Unlock()

	c.stopLifecycle()
	c.stopDisplayRefresh()
	c.stopHeartbeat()
	c.setState(Authenticating)
	packets := protocol.BuildAuthPackets(c.now())
	for index, packet := range packets {
		c.log("[AUTH] packet %d/%d\n", index+1, len(packets))
		if err := c.sendBoth(ctx, packet); err != nil {
			return c.authenticationFailure(err)
		}
		if err := sleepContext(ctx, c.packetGap); err != nil {
			return c.authenticationFailure(err)
		}
	}
	if err := sleepContext(ctx, c.authWait); err != nil {
		return c.authenticationFailure(err)
	}

	c.setState(Ready)
	c.log("[AUTH] complete\n")
	c.startHeartbeat(ctx)
	return nil
}

// Errors reports asynchronous connection failures such as heartbeat writes.
func (c *Client) Errors() <-chan error {
	return c.errors
}

// States returns the current left and right arm states.
func (c *Client) States() (State, State) {
	return c.Left.State(), c.Right.State()
}

// Close stops background work before closing both BLE connections.
func (c *Client) Close() error {
	c.stopAutoReconnect()
	c.stopDisplayRefresh()
	c.stopLifecycle()
	c.stopHeartbeat()
	c.stopNotifications()
	c.failPendingAcks(ble.ErrDisconnected)
	c.setState(Disconnected)
	err := c.transport.Close()
	c.notificationWG.Wait()
	return err
}

func (c *Client) sendBoth(ctx context.Context, packet []byte) error {
	c.logPacket("TX", ble.Left, packet)
	if err := c.transport.Write(ctx, ble.Left, packet); err != nil {
		return err
	}
	if err := sleepContext(ctx, c.leftToRightGap); err != nil {
		return err
	}
	c.logPacket("TX", ble.Right, packet)
	if err := c.transport.Write(ctx, ble.Right, packet); err != nil {
		return err
	}
	return sleepContext(ctx, c.afterRightGap)
}

func (c *Client) authenticationFailure(cause error) error {
	c.stopLifecycle()
	c.stopDisplayRefresh()
	c.stopHeartbeat()
	c.setState(Disconnected)
	closeErr := c.transport.Close()
	return errors.Join(fmt.Errorf("%w: %w", ErrAuthenticationFailed, cause), closeErr)
}

func (c *Client) startHeartbeat(parent context.Context) {
	if c.heartbeatInterval <= 0 {
		return
	}
	c.stopLifecycle()
	c.stopHeartbeat()
	c.heartbeatMu.Lock()
	stopLifecycle := c.lifecycleStop
	c.lifecycleStop = nil
	c.heartbeatMu.Unlock()
	if stopLifecycle != nil {
		stopLifecycle()
	}

	ctx, cancel := context.WithCancel(parent)
	c.heartbeatMu.Lock()
	c.heartbeatCancel = cancel
	c.lifecycleStop = context.AfterFunc(parent, func() {
		c.setState(Disconnected)
		_ = c.transport.Close()
	})
	c.heartbeatWG.Add(1)
	c.heartbeatMu.Unlock()

	c.log("[HEARTBEAT] started\n")
	go c.heartbeatLoop(ctx)
}

func (c *Client) stopLifecycle() {
	c.heartbeatMu.Lock()
	stop := c.lifecycleStop
	c.lifecycleStop = nil
	c.heartbeatMu.Unlock()
	if stop != nil {
		stop()
	}
}

func (c *Client) stopHeartbeat() {
	c.heartbeatMu.Lock()
	cancel := c.heartbeatCancel
	c.heartbeatCancel = nil
	c.heartbeatMu.Unlock()
	if cancel != nil {
		cancel()
		c.heartbeatWG.Wait()
	}
}

func (c *Client) heartbeatLoop(ctx context.Context) {
	defer c.heartbeatWG.Done()
	for {
		if err := c.sendHeartbeat(ctx); err != nil {
			if !errors.Is(err, context.Canceled) {
				wrapped := fmt.Errorf("heartbeat: %w", err)
				c.setState(Disconnected)
				_ = c.transport.Close()
				c.emitError(wrapped)
			}
			return
		}
		if err := sleepContext(ctx, c.heartbeatInterval); err != nil {
			return
		}
	}
}

func (c *Client) sendHeartbeat(ctx context.Context) error {
	c.evenHubMu.Lock()
	evenHubActive := c.evenHubActive
	if evenHubActive {
		sequence := c.evenHubSequence
		c.evenHubSequence++
		magic := c.evenHubMagic
		c.evenHubMagic++
		if c.evenHubMagic > 120 {
			c.evenHubMagic = 1
		}
		c.evenHubMu.Unlock()
		frames, err := protocol.FrameEvenHub(sequence, protocol.EvenHubServiceID, protocol.EvenHubRequest, protocol.BuildEvenHubHeartbeat(magic), c.evenHubChunkSize)
		if err != nil {
			return err
		}
		for _, frame := range frames {
			c.logPacket("TX", ble.Right, frame)
			if err := c.transport.Write(ctx, ble.Right, frame); err != nil {
				return err
			}
		}
		return nil
	}
	c.evenHubMu.Unlock()

	sequence := c.heartbeatSequence
	packet := protocol.BuildHeartbeat(sequence)
	c.heartbeatSequence++
	if c.heartbeatSequence == 0 {
		c.heartbeatSequence = 0xC0
	}

	for _, arm := range []ble.Arm{ble.Left, ble.Right} {
		c.logPacket("TX", arm, packet)
		if err := c.transport.Write(ctx, arm, packet); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) emitError(err error) {
	select {
	case c.errors <- err:
	default:
	}
}

func (c *Client) setState(state State) {
	c.Left.setState(state)
	c.Right.setState(state)
}

func (c *Client) log(format string, args ...any) {
	if c.debug {
		fmt.Fprintf(c.output, format, args...)
	}
}

func (c *Client) logPacket(direction string, arm ble.Arm, packet []byte) {
	if c.debug {
		fmt.Fprintf(c.output, "[%s][%s] % X\n", direction, arm, packet)
	}
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
