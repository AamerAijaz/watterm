package server

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/AamerAijaz/watterm/internal/queue"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	_ "modernc.org/sqlite"
)

// WhatsApp is the whatsmeow adapter. It stores the Session in a sqlite file.
type WhatsApp struct {
	path      string
	container *sqlstore.Container
	client    *whatsmeow.Client
	qr        <-chan whatsmeow.QRChannelItem

	mu       sync.Mutex
	changes  chan queue.Link
	pending  Snapshot
	snapWake chan struct{}
	watch    sync.Once
}

// NewWhatsApp pairs through the Session file at path.
func NewWhatsApp(path string) *WhatsApp {
	return &WhatsApp{
		path:     path,
		changes:  make(chan queue.Link, 1),
		snapWake: make(chan struct{}, 1),
	}
}

// Open connects. A stored Session is AlreadyLinked and does not ask for a QR code.
// A connect failure after a stored Session is still AlreadyLinked.
func (w *WhatsApp) Open(ctx context.Context) (Status, error) {
	stored, err := w.sessionStored(ctx)
	if err != nil {
		_ = w.Close()
		return 0, err
	}

	// A dropped socket reconnects. Logout is a later slice.
	w.client.EnableAutoReconnect = true
	w.watch.Do(func() {
		w.client.AddEventHandler(w.onEvent)
	})
	if stored {
		return AlreadyLinked, w.client.ConnectContext(ctx)
	}

	qr, err := w.client.GetQRChannel(ctx)
	if err != nil {
		_ = w.Close()
		return 0, err
	}
	w.qr = qr
	if err = w.client.ConnectContext(ctx); err != nil {
		_ = w.Close()
		return 0, err
	}
	return NeedsPair, nil
}

// sessionStored opens the Session file and reports whether a device is saved.
// whatsmeow writes the device during a successful scan.
func (w *WhatsApp) sessionStored(ctx context.Context) (bool, error) {
	dsn := "file:" + filepath.ToSlash(w.path) + "?_pragma=foreign_keys(1)"
	container, err := sqlstore.New(ctx, "sqlite", dsn, nil)
	if err != nil {
		return false, err
	}
	w.container = container

	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return false, err
	}
	w.client = whatsmeow.NewClient(device, nil)
	return device.ID != nil, nil
}

// NextCode returns the next QR payload. ErrLinked means the scan stored the Session.
func (w *WhatsApp) NextCode(ctx context.Context) (string, error) {
	if w.qr == nil {
		return "", errors.New("pairing was not started")
	}
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case item, ok := <-w.qr:
			if !ok {
				return "", errors.New("pairing ended")
			}
			switch item.Event {
			case whatsmeow.QRChannelEventCode:
				if item.Code == "" {
					continue
				}
				return item.Code, nil
			case whatsmeow.QRChannelSuccess.Event:
				return "", ErrLinked
			case whatsmeow.QRChannelEventPasskeyRequest, whatsmeow.QRChannelEventPasskeyResponse:
				continue
			case whatsmeow.QRChannelEventError:
				if item.Error != nil {
					return "", item.Error
				}
				return "", errors.New("pairing failed")
			default:
				return "", fmt.Errorf("pairing %s", item.Event)
			}
		}
	}
}

// Close disconnects and closes the Session file.
func (w *WhatsApp) Close() error {
	if w.client != nil {
		w.client.Disconnect()
		w.client = nil
	}
	if w.container != nil {
		err := w.container.Close()
		w.container = nil
		return err
	}
	return nil
}

// ReadySnapshot returns a history snapshot that arrived before the caller asked.
func (w *WhatsApp) ReadySnapshot() (Snapshot, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pending.empty() {
		return Snapshot{}, false
	}
	snap := w.pending
	w.pending = Snapshot{}
	return snap, true
}

// Wait reports a dropped socket, its return, or a snapshot. Neither carries a QR code.
func (w *WhatsApp) Wait(ctx context.Context) (Happening, error) {
	for {
		select {
		case <-ctx.Done():
			return Happening{}, ctx.Err()
		case status := <-w.changes:
			return Happening{Link: status, HasLink: true}, nil
		case <-w.snapWake:
			snap, ok := w.ReadySnapshot()
			if !ok {
				continue
			}
			return Happening{Snapshot: &snap}, nil
		}
	}
}

func (w *WhatsApp) onEvent(evt any) {
	if hs, ok := evt.(*events.HistorySync); ok {
		w.addSnap(snapshotFromHistory(hs.Data))
		return
	}
	status, ok := linkStatus(evt)
	if !ok {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	select {
	case <-w.changes:
	default:
	}
	w.changes <- status
}

func (w *WhatsApp) addSnap(snap Snapshot) {
	if snap.empty() {
		return
	}
	w.mu.Lock()
	w.pending.Chats = append(w.pending.Chats, snap.Chats...)
	w.pending.Contacts = append(w.pending.Contacts, snap.Contacts...)
	w.pending.Pushes = append(w.pending.Pushes, snap.Pushes...)
	w.pending.Mappings = append(w.pending.Mappings, snap.Mappings...)
	w.mu.Unlock()
	select {
	case w.snapWake <- struct{}{}:
	default:
	}
}

// linkStatus maps a connection event to a State link.
// A keepalive timeout is a dropped network: whatsmeow then disconnects without emitting Disconnected.
// Logout is not a drop.
func linkStatus(evt any) (queue.Link, bool) {
	switch evt.(type) {
	case *events.Disconnected, *events.KeepAliveTimeout:
		return queue.Reconnecting, true
	case *events.Connected, *events.KeepAliveRestored:
		return queue.Linked, true
	default:
		return 0, false
	}
}
