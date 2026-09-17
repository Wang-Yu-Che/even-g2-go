package g2

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

type NotificationConfig = protocol.NotificationConfig

type PhoneNotification struct {
	ID          string
	PackageName string
	Title       string
	Subtitle    string
	Message     string
	Timestamp   time.Time
	DisplayName string
}

// ConfigureNotifications configures the native notification centre.
func (c *Client) ConfigureNotifications(ctx context.Context, config NotificationConfig) error {
	return c.configureNotifications(ctx, config, true)
}

func (c *Client) restoreNotifications(ctx context.Context) error {
	c.notificationConfigMu.Lock()
	config := c.notificationConfig
	c.notificationConfigMu.Unlock()
	if config == nil {
		return nil
	}
	return c.configureNotifications(ctx, *config, false)
}

func (c *Client) configureNotifications(ctx context.Context, config NotificationConfig, remember bool) error {
	if config.DurationSeconds <= 0 {
		config.DurationSeconds = 5
	}
	if err := c.sendService(ctx, protocol.NotificationServiceID, protocol.BuildNotificationControl(c.nextEvenHubMagic(), config), ble.Right); err != nil {
		return err
	}
	if config.Enabled {
		if err := sleepContext(ctx, 400*time.Millisecond); err != nil {
			return err
		}
		if err := c.sendService(ctx, protocol.NotificationServiceID, protocol.BuildNotificationWhitelistControl(c.nextEvenHubMagic(), true), ble.Right); err != nil {
			return err
		}
	}
	if remember {
		c.notificationConfigMu.Lock()
		c.notificationConfig = &config
		c.notificationConfigMu.Unlock()
	}
	return nil
}

// PushNotification uploads one Android notification JSON document to the glasses.
func (c *Client) PushNotification(ctx context.Context, notification PhoneNotification) error {
	id, err := c.notificationID(notification.ID)
	if err != nil {
		return err
	}
	payload, err := protocol.BuildNotificationJSON(protocol.PhoneNotification{
		ID: id, PackageName: notification.PackageName, Title: notification.Title,
		Subtitle: notification.Subtitle, Message: notification.Message,
		Timestamp: notification.Timestamp, DisplayName: notification.DisplayName,
	})
	if err != nil {
		return err
	}
	status, err := c.TransferFile(ctx, protocol.FileTypeNotification, protocol.NotificationFilePath, payload)
	if err != nil {
		return err
	}
	if status != 0 {
		return fmt.Errorf("notification file transfer was rejected: %s (%d)", protocol.FileStatusName(status), status)
	}
	return nil
}

func (c *Client) notificationID(phoneID string) (int, error) {
	c.notificationConfigMu.Lock()
	defer c.notificationConfigMu.Unlock()
	if phoneID != "" {
		if id := c.notificationIDs[phoneID]; id != 0 {
			return id, nil
		}
	}
	if c.nextNotificationID > 9999 {
		return 0, errors.New("notification ID capacity exhausted")
	}
	id := c.nextNotificationID
	c.nextNotificationID++
	if phoneID != "" {
		c.notificationIDs[phoneID] = id
	}
	return id, nil
}
