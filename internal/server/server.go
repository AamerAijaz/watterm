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
	// ReadySnapshot returns a snapshot that is already waiting.
	ReadySnapshot() (Snapshot, bool)
	// Wait blocks until the connection changes or a snapshot arrives.
	Wait(ctx context.Context) (Happening, error)
	Close() error
}

// Happening is a connection change or a snapshot.
type Happening struct {
	Link     queue.Link
	HasLink  bool
	Snapshot *Snapshot
}

// Run publishes pairing State until ctx is cancelled.
// With no Session it publishes Pairing, then each QR code.
// After a scan, or when a Session is already stored, it publishes Linked.
// The first snapshot is written to the Transcript and included on that state.
func Run(ctx context.Context, end queue.Server, link Link, chats Transcript) error {
	status, err := link.Open(ctx)
	if err != nil && status != AlreadyLinked {
		return err
	}
	defer link.Close()

	if status == AlreadyLinked {
		return publishLinked(ctx, end, link, chats, err, false)
	}

	if err := end.Publish(queue.State{Link: queue.Pairing}); err != nil {
		return err
	}
	for {
		code, err := link.NextCode(ctx)
		if errors.Is(err, ErrLinked) {
			return publishLinked(ctx, end, link, chats, nil, true)
		}
		if err != nil {
			return err
		}
		if err := end.Publish(queue.State{Link: queue.Pairing, QRCode: code}); err != nil {
			return err
		}
	}
}

// publishLinked publishes Linked, then returns connectErr.
// The state is Linked and QRCode is empty when the dial failed.
// If ReadySnapshot already has a snapshot, fold writes it into the Transcript before that publish.
// A dropped connection then publishes reconnecting with the same chats and an empty QRCode.
func publishLinked(ctx context.Context, end queue.Server, link Link, chats Transcript, connectErr error, justPaired bool) error {
	if chats == nil {
		chats = noopChats{}
	}
	list, err := chats.Load(ctx)
	if err != nil {
		return err
	}
	if snap, ok := link.ReadySnapshot(); ok {
		list = fold(list, snap)
		if err := chats.Save(ctx, list); err != nil {
			return err
		}
	} else if justPaired && len(list) == 0 {
		ev, err := link.Wait(ctx)
		if err != nil {
			return err
		}
		if ev.Snapshot != nil {
			list = fold(list, *ev.Snapshot)
			if err := chats.Save(ctx, list); err != nil {
				return err
			}
		}
	}
	if err := end.Publish(queue.State{Link: queue.Linked, Chats: list}); err != nil {
		return err
	}
	if connectErr != nil {
		return connectErr
	}
	current := queue.Linked
	for {
		ev, err := link.Wait(ctx)
		if err != nil {
			return err
		}
		if ev.Snapshot != nil {
			list = fold(list, *ev.Snapshot)
			if err := chats.Save(ctx, list); err != nil {
				return err
			}
		}
		if ev.HasLink {
			current = ev.Link
		}
		if err := end.Publish(queue.State{Link: current, Chats: list}); err != nil {
			return err
		}
	}
}
