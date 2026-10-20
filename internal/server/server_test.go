package server_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/AamerAijaz/watterm/internal/queue"
	"github.com/AamerAijaz/watterm/internal/server"
)

func TestNoSessionPublishesPairingThenAQRCode(t *testing.T) {
	ui, srv := queue.New()
	link := newMemLink()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = server.Run(ctx, srv, link) }()

	got, err := ui.NextState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Link != queue.Pairing || got.QRCode != "" {
		t.Fatalf("first state = %+v, want pairing and no code", got)
	}

	link.code("111")

	got, err = ui.NextState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Link != queue.Pairing || got.QRCode != "111" {
		t.Fatalf("second state = %+v, want pairing with code 111", got)
	}
}

func TestNewerQRCodeReplacesAnUnreadOne(t *testing.T) {
	ui, srv := queue.New()
	link := newMemLink()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	end := &countPublish{Server: srv, want: 3, hit: make(chan struct{})}
	go func() { _ = server.Run(ctx, end, link) }()

	if _, err := ui.NextState(ctx); err != nil {
		t.Fatal(err)
	}

	go func() {
		link.code("stale")
		link.code("fresh")
	}()

	select {
	case <-end.hit:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for both codes to be published")
	}

	got, err := ui.NextState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.QRCode != "fresh" {
		t.Fatalf("QRCode = %q, want fresh", got.QRCode)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel2()
	if extra, err := ui.NextState(ctx2); err == nil {
		t.Fatalf("stale code was still queued: %+v", extra)
	}
}

func TestStoredSessionPublishesLinkedWhenConnectFails(t *testing.T) {
	ui, srv := queue.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run(ctx, srv, linkOpen{status: server.AlreadyLinked, err: errDial})
	}()

	got, err := ui.NextState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Link != queue.Linked || got.QRCode != "" {
		t.Fatalf("state = %+v, want linked and no code", got)
	}
	if err := <-errCh; err != errDial {
		t.Fatalf("Run error = %v, want dial failure", err)
	}
}

var errDial = errors.New("dial failed")

type linkOpen struct {
	status server.Status
	err    error
}

func (l linkOpen) Open(context.Context) (server.Status, error) { return l.status, l.err }
func (l linkOpen) NextCode(context.Context) (string, error) {
	return "", errors.New("no code")
}
func (l linkOpen) Close() error { return nil }
func (l linkOpen) NextLink(context.Context) (queue.Link, error) {
	return 0, errors.New("no link change")
}

func TestScanStoresTheSessionAndALaterStartIsLinked(t *testing.T) {
	link := newMemLink()
	ctx, cancel := context.WithCancel(context.Background())

	ui, srv := queue.New()
	go func() { _ = server.Run(ctx, srv, link) }()

	if _, err := ui.NextState(ctx); err != nil {
		t.Fatal(err)
	}
	link.code("111")
	if _, err := ui.NextState(ctx); err != nil {
		t.Fatal(err)
	}

	link.scan()
	got, err := ui.NextState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Link != queue.Linked || got.QRCode != "" {
		t.Fatalf("after scan = %+v, want linked and no code", got)
	}
	cancel()

	ui, srv = queue.New()
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = server.Run(ctx, srv, link) }()

	got, err = ui.NextState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Link != queue.Linked || got.QRCode != "" {
		t.Fatalf("restart = %+v, want linked and no code", got)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel2()
	if extra, err := ui.NextState(ctx2); err == nil {
		t.Fatalf("restart published a QR code: %+v", extra)
	}
}

func TestDroppedNetworkPublishesReconnectingThenLinked(t *testing.T) {
	ui, srv := queue.New()
	link := newMemLink()
	link.paired = true
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = server.Run(ctx, srv, link) }()

	got, err := ui.NextState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Link != queue.Linked || got.QRCode != "" {
		t.Fatalf("before drop = %+v, want linked and no code", got)
	}

	link.drop()
	wait, cancelWait := context.WithTimeout(ctx, 2*time.Second)
	defer cancelWait()
	got, err = ui.NextState(wait)
	if err != nil {
		t.Fatal(err)
	}
	if got.Link != queue.Reconnecting || got.QRCode != "" {
		t.Fatalf("after drop = %+v, want reconnecting and no code", got)
	}

	link.restore()
	got, err = ui.NextState(wait)
	if err != nil {
		t.Fatal(err)
	}
	if got.Link != queue.Linked || got.QRCode != "" {
		t.Fatalf("after restore = %+v, want linked and no code", got)
	}
}

// countPublish reports when the Server end has accepted want publishes.
type countPublish struct {
	queue.Server
	mu   sync.Mutex
	n    int
	want int
	hit  chan struct{}
}

func (c *countPublish) Publish(state queue.State) error {
	err := c.Server.Publish(state)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
	if c.n == c.want {
		close(c.hit)
	}
	return err
}

// memLink is the in-memory stand-in for WhatsApp. A scan stores the Session on this value.
type memLink struct {
	mu      sync.Mutex
	paired  bool
	codes   chan string
	scanned chan struct{}
	links   chan queue.Link
}

func newMemLink() *memLink {
	return &memLink{
		codes:   make(chan string),
		scanned: make(chan struct{}),
		links:   make(chan queue.Link),
	}
}

func (m *memLink) Open(context.Context) (server.Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.paired {
		return server.AlreadyLinked, nil
	}
	return server.NeedsPair, nil
}

func (m *memLink) NextCode(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-m.scanned:
		return "", server.ErrLinked
	case code := <-m.codes:
		return code, nil
	}
}

func (m *memLink) Close() error { return nil }

func (m *memLink) NextLink(ctx context.Context) (queue.Link, error) {
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case status := <-m.links:
		return status, nil
	}
}

func (m *memLink) drop()    { m.links <- queue.Reconnecting }
func (m *memLink) restore() { m.links <- queue.Linked }

func (m *memLink) code(code string) { m.codes <- code }

func (m *memLink) scan() {
	m.mu.Lock()
	m.paired = true
	m.mu.Unlock()
	select {
	case <-m.scanned:
	default:
		close(m.scanned)
	}
}
