package g2

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Wang-Yu-Che/even-g2-go/ble"
	"github.com/Wang-Yu-Che/even-g2-go/protocol"
)

var (
	ErrFileServiceUnavailable    = errors.New("G2 file service is unavailable")
	ErrFileServiceNeedsReconnect = errors.New("G2 file service requires reconnect")
)

type fileReply struct {
	status int
	err    error
}

// AttachFileNotifications routes file-service acknowledgements into the client.
func (c *Client) AttachFileNotifications(ctx context.Context, notifications <-chan []byte) {
	ctx, cancel := context.WithCancel(ctx)
	c.notificationMu.Lock()
	c.notificationStops = append(c.notificationStops, cancel)
	c.notificationMu.Unlock()
	c.notificationWG.Add(1)
	go func() {
		defer c.notificationWG.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case frame, ok := <-notifications:
				if !ok {
					return
				}
				command, status, err := protocol.ParseFileAck(frame)
				if err != nil {
					continue
				}
				c.fileStateMu.Lock()
				if command == c.filePendingCommand && c.filePendingAck != nil {
					select {
					case c.filePendingAck <- fileReply{status: status}:
					default:
					}
				}
				c.fileStateMu.Unlock()
			}
		}
	}()
}

// TransferFile uploads one payload through services 0xC4/0xC5.
func (c *Client) TransferFile(ctx context.Context, fileType int, filename string, data []byte) (status int, err error) {
	transport, ok := c.transport.(ble.FileTransport)
	if !ok {
		return 0, ErrFileServiceUnavailable
	}
	c.fileMu.Lock()
	defer c.fileMu.Unlock()
	c.fileStateMu.Lock()
	needsReconnect := c.fileNeedsReconnect
	c.fileStateMu.Unlock()
	if needsReconnect {
		return 0, ErrFileServiceNeedsReconnect
	}
	start, buildErr := protocol.BuildFileStart(fileType, filename, data)
	if buildErr != nil {
		return 0, buildErr
	}
	defer func() {
		if err != nil {
			c.fileStateMu.Lock()
			c.fileNeedsReconnect = true
			c.fileStateMu.Unlock()
		}
	}()

	status, err = c.filePhase(ctx, protocol.FileCommandStart, func() error {
		return c.writeFileService(ctx, transport, protocol.FileCommandServiceID, start)
	})
	if err != nil || status != 0 {
		return status, err
	}
	status, err = c.filePhase(ctx, protocol.FileCommandData, func() error {
		if writeErr := c.writeFileService(ctx, transport, protocol.FileCommandServiceID, protocol.BuildFileDataCommand()); writeErr != nil {
			return writeErr
		}
		return c.writeFileService(ctx, transport, protocol.FileDataServiceID, data)
	})
	if err != nil || status != 0 {
		return status, err
	}
	return c.filePhase(ctx, protocol.FileCommandResultCheck, func() error {
		return c.writeFileService(ctx, transport, protocol.FileCommandServiceID, protocol.BuildFileResultCheck())
	})
}

func (c *Client) filePhase(ctx context.Context, command int, write func() error) (int, error) {
	replies := make(chan fileReply, 1)
	c.fileStateMu.Lock()
	c.filePendingCommand = command
	c.filePendingAck = replies
	c.fileStateMu.Unlock()
	defer func() {
		c.fileStateMu.Lock()
		if c.filePendingAck == replies {
			c.filePendingCommand = -1
			c.filePendingAck = nil
		}
		c.fileStateMu.Unlock()
	}()
	if err := write(); err != nil {
		return 0, err
	}
	timer := time.NewTimer(c.fileTimeout)
	defer timer.Stop()
	select {
	case reply := <-replies:
		return reply.status, reply.err
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-timer.C:
		return 0, fmt.Errorf("file service command %d acknowledgement timed out", command)
	}
}

func (c *Client) writeFileService(ctx context.Context, transport ble.FileTransport, serviceID byte, payload []byte) error {
	c.evenHubMu.Lock()
	sequence := c.evenHubSequence
	c.evenHubSequence++
	c.evenHubMu.Unlock()
	frames, err := protocol.FrameEvenHub(sequence, serviceID, 0, payload, c.evenHubChunkSize)
	if err != nil {
		return err
	}
	for _, frame := range frames {
		c.logPacket("TX-FILE", ble.Right, frame)
		if err := transport.WriteFile(ctx, ble.Right, frame); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) resetFileService() {
	c.fileStateMu.Lock()
	if c.filePendingAck != nil {
		select {
		case c.filePendingAck <- fileReply{err: ble.ErrDisconnected}:
		default:
		}
	}
	c.filePendingCommand = -1
	c.filePendingAck = nil
	c.fileNeedsReconnect = false
	c.fileStateMu.Unlock()
}
