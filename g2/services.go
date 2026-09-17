package g2

import (
	"context"
	"errors"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

// StartCompass starts navigation heading and calibration notifications.
func (c *Client) StartCompass(ctx context.Context) error {
	return c.sendService(ctx, protocol.NavigationServiceID, protocol.BuildNavigationStart(c.nextEvenHubMagic()), ble.Right)
}

// StopCompass stops navigation heading notifications.
func (c *Client) StopCompass(ctx context.Context) error {
	return c.sendService(ctx, protocol.NavigationServiceID, protocol.BuildNavigationExit(c.nextEvenHubMagic()), ble.Right)
}

// SendNavigationHeartbeat keeps an active navigation session alive.
func (c *Client) SendNavigationHeartbeat(ctx context.Context) error {
	return c.sendService(ctx, protocol.NavigationServiceID, protocol.BuildNavigationHeartbeat(c.nextEvenHubMagic()), ble.Right)
}

// SetHeyEven enables or disables the built-in wake word.
func (c *Client) SetHeyEven(ctx context.Context, enabled bool) error {
	return c.sendService(ctx, protocol.EvenAIServiceID, protocol.BuildEvenAIConfig(c.nextEvenHubMagic(), enabled), ble.Right)
}

// ControlEvenAI changes the built-in AI session state.
func (c *Client) ControlEvenAI(ctx context.Context, status int) error {
	return c.sendService(ctx, protocol.EvenAIServiceID, protocol.BuildEvenAIControl(c.nextEvenHubMagic(), status), ble.Right)
}

// AskEvenAI sends resolved ASR text into the built-in AI session.
func (c *Client) AskEvenAI(ctx context.Context, text string) error {
	return c.sendService(ctx, protocol.EvenAIServiceID, protocol.BuildEvenAIAsk(c.nextEvenHubMagic(), text, false), ble.Right)
}

// TriggerEvenAISkill invokes one built-in Even AI skill.
func (c *Client) TriggerEvenAISkill(ctx context.Context, skillID, skillParam int, text string) error {
	return c.sendService(ctx, protocol.EvenAIServiceID, protocol.BuildEvenAISkill(c.nextEvenHubMagic(), skillID, skillParam, text), ble.Right)
}

// ShowNotificationsPanel follows the verified ENTER, ASK, SKILL sequence.
func (c *Client) ShowNotificationsPanel(ctx context.Context) error {
	if err := c.ControlEvenAI(ctx, protocol.EvenAIStatusEnter); err != nil {
		return err
	}
	if err := sleepContext(ctx, 400*time.Millisecond); err != nil {
		return err
	}
	if err := c.AskEvenAI(ctx, " "); err != nil {
		return err
	}
	if err := sleepContext(ctx, 400*time.Millisecond); err != nil {
		return err
	}
	return c.TriggerEvenAISkill(ctx, protocol.EvenAISkillNotification, 1, " ")
}

// SkipOnboarding marks the firmware onboarding process complete.
func (c *Client) SkipOnboarding(ctx context.Context) error {
	return c.sendService(ctx, protocol.OnboardingServiceID, protocol.BuildOnboardingFinish(c.nextEvenHubMagic()), ble.Right)
}

// InitializeGestureControl initializes the gesture lifecycle service.
func (c *Client) InitializeGestureControl(ctx context.Context) error {
	return c.sendService(ctx, protocol.GestureControlServiceID, protocol.BuildGestureControlInit(c.nextEvenHubMagic()), ble.Right)
}

// SyncTime pushes local wall-clock time to both arms.
func (c *Client) SyncTime(ctx context.Context, instant time.Time) error {
	return c.sendService(ctx, protocol.DeviceSettingsServiceID, protocol.BuildDeviceTimeSync(c.nextEvenHubMagic(), instant), ble.Left, ble.Right)
}

// SetRingConnection connects or disconnects a six-byte R1 MAC through the G2.
func (c *Client) SetRingConnection(ctx context.Context, connected bool, mac []byte, name string) error {
	if len(mac) != 6 {
		return errors.New("ring MAC must contain 6 bytes")
	}
	return c.sendService(ctx, protocol.DeviceSettingsServiceID, protocol.BuildRingConnection(c.nextEvenHubMagic(), connected, mac, name), ble.Right)
}

// SetIMU enables or disables gravity-normalized accelerometer reports.
func (c *Client) SetIMU(ctx context.Context, enabled bool, reportFrequency int) error {
	if enabled && (reportFrequency < 100 || reportFrequency > 1000 || reportFrequency%100 != 0) {
		return errors.New("IMU report frequency must be P100..P1000")
	}
	if enabled && !c.nativeCreated {
		if err := c.ShowText(ctx, "imu", " "); err != nil {
			return err
		}
	}
	magic := c.nextEvenHubMagic()
	return c.sendService(ctx, protocol.EvenHubServiceID, protocol.BuildEvenHubIMUControl(enabled, reportFrequency, magic), ble.Right)
}

func (c *Client) sendService(ctx context.Context, serviceID byte, payload []byte, arms ...ble.Arm) error {
	if c.Left.State() != Ready || c.Right.State() != Ready {
		return ErrNotReady
	}
	c.evenHubMu.Lock()
	sequence := c.evenHubSequence
	c.evenHubSequence++
	c.evenHubMu.Unlock()
	frames, err := protocol.FrameEvenHub(sequence, serviceID, protocol.EvenHubRequest, payload, c.evenHubChunkSize)
	if err != nil {
		return err
	}
	c.evenHubWriteMu.Lock()
	defer c.evenHubWriteMu.Unlock()
	for _, arm := range arms {
		for _, frame := range frames {
			c.logPacket("TX", arm, frame)
			if err := c.transport.Write(ctx, arm, frame); err != nil {
				return err
			}
		}
	}
	return nil
}
