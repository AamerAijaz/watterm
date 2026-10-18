package companion

import (
	"context"
	"path/filepath"
	"sync"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	_ "modernc.org/sqlite"

	"github.com/AamerAijaz/watterm/internal/link"
	"github.com/AamerAijaz/watterm/internal/paths"
	"github.com/AamerAijaz/watterm/internal/transcript"
)

// Running is one whatsmeow Companion opened against a Layout.
type Running struct {
	client    *whatsmeow.Client
	container *sqlstore.Container
	once      sync.Once
	done      chan struct{}
}

// Open opens the Session store. It does not connect.
func Open(ctx context.Context, lay paths.Layout) (*Running, error) {
	container, err := sqlstore.New(ctx, "sqlite", sessionDSN(lay.Session), waLog.Noop)
	if err != nil {
		return nil, err
	}
	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		container.Close()
		return nil, err
	}
	return &Running{
		client:    whatsmeow.NewClient(device, waLog.Noop),
		container: container,
		done:      make(chan struct{}),
	}, nil
}

// Paired reports whether a Session is already stored.
func (r *Running) Paired() bool {
	return r.client.Store.ID != nil
}

// Snapshot is the link phase at process start.
func (r *Running) Snapshot() link.Snapshot {
	return link.Initial(r.Paired())
}

// TranscriptChanged means the chat list should be read again.
type TranscriptChanged struct{}

// Start connects and returns link events. record receives the initial history push.
// A logged-out Companion also stops reconnecting.
func (r *Running) Start(ctx context.Context, record *transcript.Store) (<-chan any, error) {
	out := make(chan any, 32)
	r.client.AddEventHandler(func(evt any) {
		if hist, ok := evt.(*events.HistorySync); ok && record != nil {
			if err := ApplySync(record, hist); err != nil {
				return
			}
			select {
			case out <- TranscriptChanged{}:
			case <-r.done:
			}
			return
		}
		mapped, ok := mapEvent(evt)
		if !ok {
			return
		}
		if _, loggedOut := mapped.(link.LoggedOut); loggedOut {
			r.client.EnableAutoReconnect = false
		}
		select {
		case out <- mapped:
		case <-r.done:
		}
	})
	if !r.Paired() {
		qr, err := r.client.GetQRChannel(ctx)
		if err != nil {
			return nil, err
		}
		go func() {
			for item := range qr {
				mapped, ok := mapQR(item)
				if !ok {
					continue
				}
				select {
				case out <- mapped:
				case <-r.done:
					return
				}
			}
		}()
	}
	if err := r.client.Connect(); err != nil {
		return nil, err
	}
	return out, nil
}

// Close disconnects and closes the Session store. It is safe to call more than once.
func (r *Running) Close() {
	r.once.Do(func() {
		close(r.done)
		r.client.Disconnect()
		r.container.Close()
	})
}

func sessionDSN(path string) string {
	return "file:" + filepath.ToSlash(path) + "?_pragma=foreign_keys(1)"
}
