// Package server owns pairing and the WhatsApp connection.
// It publishes State and does not call the UI.
package server

import (
	"context"
	"errors"

	"github.com/AamerAijaz/watterm/internal/queue"
)

// Status is whether a Session is already stored.
type Status int

const (
	NeedsPair Status = iota + 1
	AlreadyLinked
)

// ErrLinked means the scan stored the Session.
var ErrLinked = errors.New("linked")

// Link is the WhatsApp pairing port. The production adapter uses whatsmeow.
// An in-memory adapter stands in for it in tests.
type Link interface {
	// Open starts the connection and reports whether a Session is already stored.
	Open(ctx context.Context) (Status, error)
	// NextCode waits for the next QR code. It returns ErrLinked once the Session is stored.
	NextCode(ctx context.Context) (string, error)
	// NextLink waits until the connection drops or returns.
	// A drop is Reconnecting and carries no QR code. The return is Linked.
	NextLink(ctx context.Context) (queue.Link, error)
	Close() error
}

// Run publishes pairing State until ctx is cancelled.
// With no Session it publishes Pairing, then each QR code.
// After a scan, or when a Session is already stored, it publishes Linked and no code.
func Run(ctx context.Context, end queue.Server, link Link) error {
	status, err := link.Open(ctx)
	if err != nil && status != AlreadyLinked {
		return err
	}
	defer link.Close()

	if status == AlreadyLinked {
		return publishLinked(ctx, end, link, err)
	}

	if err := end.Publish(queue.State{Link: queue.Pairing}); err != nil {
		return err
	}
	for {
		code, err := link.NextCode(ctx)
		if errors.Is(err, ErrLinked) {
			return publishLinked(ctx, end, link, nil)
		}
		if err != nil {
			return err
		}
		if err := end.Publish(queue.State{Link: queue.Pairing, QRCode: code}); err != nil {
			return err
		}
	}
}

// publishLinked tells the UI the Session is stored. connectErr is returned after that
// so a failed dial still counts as linked and does not ask for a QR code.
// After that, a dropped connection is reconnecting until the socket returns.
func publishLinked(ctx context.Context, end queue.Server, link Link, connectErr error) error {
	if err := end.Publish(queue.State{Link: queue.Linked}); err != nil {
		return err
	}
	if connectErr != nil {
		return connectErr
	}
	for {
		status, err := link.NextLink(ctx)
		if err != nil {
			return err
		}
		if err := end.Publish(queue.State{Link: status}); err != nil {
			return err
		}
	}
}
