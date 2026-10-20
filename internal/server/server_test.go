package server_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
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

	go func() { _ = server.Run(ctx, srv, link, nil) }()

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
	go func() { _ = server.Run(ctx, end, link, nil) }()

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
		errCh <- server.Run(ctx, srv, linkOpen{status: server.AlreadyLinked, err: errDial}, nil)
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
func (l linkOpen) Close() error                           { return nil }
func (l linkOpen) ReadySnapshot() (server.Snapshot, bool) { return server.Snapshot{}, false }
func (l linkOpen) Wait(ctx context.Context) (server.Happening, error) {
	<-ctx.Done()
	return server.Happening{}, ctx.Err()
}

func TestScanStoresTheSessionAndALaterStartIsLinked(t *testing.T) {
	link := newMemLink()
	ctx, cancel := context.WithCancel(context.Background())

	ui, srv := queue.New()
	go func() { _ = server.Run(ctx, srv, link, nil) }()

	if _, err := ui.NextState(ctx); err != nil {
		t.Fatal(err)
	}
	link.code("111")
	if _, err := ui.NextState(ctx); err != nil {
		t.Fatal(err)
	}

	link.scan()
	link.sync(server.Snapshot{})

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
	go func() { _ = server.Run(ctx, srv, link, nil) }()

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
	go func() { _ = server.Run(ctx, srv, link, nil) }()

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
	snaps   chan server.Snapshot
}

func newMemLink() *memLink {
	return &memLink{
		codes:   make(chan string),
		scanned: make(chan struct{}),
		links:   make(chan queue.Link),
		snaps:   make(chan server.Snapshot, 4),
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

func (m *memLink) ReadySnapshot() (server.Snapshot, bool) {
	select {
	case snap := <-m.snaps:
		return snap, true
	default:
		return server.Snapshot{}, false
	}
}

func (m *memLink) Wait(ctx context.Context) (server.Happening, error) {
	select {
	case <-ctx.Done():
		return server.Happening{}, ctx.Err()
	case status := <-m.links:
		return server.Happening{Link: status, HasLink: true}, nil
	case snap := <-m.snaps:
		return server.Happening{Snapshot: &snap}, nil
	}
}

func (m *memLink) sync(snap server.Snapshot) { m.snaps <- snap }

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

func TestFirstSyncPublishesTheChatListAndKeepsIt(t *testing.T) {
	ctx := context.Background()
	store, err := server.OpenTranscript(ctx, filepath.Join(t.TempDir(), "transcript.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ui, srv := queue.New()
	link := newMemLink()
	runCtx, cancel := context.WithCancel(ctx)
	go func() { _ = server.Run(runCtx, srv, link, store) }()

	if _, err := ui.NextState(runCtx); err != nil {
		t.Fatal(err)
	}
	link.scan()
	link.sync(firstSnapshot())

	got, err := ui.NextState(runCtx)
	if err != nil {
		t.Fatal(err)
	}
	want := []queue.Chat{
		{Address: "15550000001@s.whatsapp.net", Name: "Sam", Activity: 50},
		{Address: "15550000002@s.whatsapp.net", Name: "Ada Lovelace", Activity: 40},
		{Address: "120363000@g.us", Name: "Family", Activity: 30},
		{Address: "15550001111@s.whatsapp.net", Name: "15550001111", Activity: 20},
		{Address: "15550000003@s.whatsapp.net", Name: "Pushy", Activity: 10},
	}
	if !sameChats(got.Chats, want) {
		t.Fatalf("chats = %+v, want %+v", got.Chats, want)
	}
	if got.Link != queue.Linked || got.QRCode != "" {
		t.Fatalf("state = %+v, want linked and no code", got)
	}

	link.sync(server.Snapshot{Pushes: []server.SnapPush{{ID: "15550001111@s.whatsapp.net", Name: "Bare"}}})
	got, err = ui.NextState(runCtx)
	if err != nil {
		t.Fatal(err)
	}
	want[3].Name = "Bare"
	if !sameChats(got.Chats, want) {
		t.Fatalf("after a later push name, chats = %+v, want %+v", got.Chats, want)
	}
	cancel()

	ui, srv = queue.New()
	link = newMemLink()
	link.paired = true
	runCtx, cancel = context.WithCancel(ctx)
	defer cancel()
	go func() { _ = server.Run(runCtx, srv, link, store) }()

	got, err = ui.NextState(runCtx)
	if err != nil {
		t.Fatal(err)
	}
	if !sameChats(got.Chats, want) {
		t.Fatalf("restart chats = %+v, want %+v", got.Chats, want)
	}
}

func TestRestartShowsChatsAlreadyInTheFile(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "transcript.db")
	seedLegacyChats(t, path)

	store, err := server.OpenTranscript(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ui, srv := queue.New()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		_ = server.Run(runCtx, srv, linkOpen{status: server.AlreadyLinked}, store)
	}()

	got, err := ui.NextState(runCtx)
	if err != nil {
		t.Fatal(err)
	}
	want := []queue.Chat{
		{Address: "15550000001@s.whatsapp.net", Name: "Sam", Activity: 1_792_310_050},
		{Address: "120363000@g.us", Name: "Family", Activity: 1_792_310_030},
		{Address: "15550000004@s.whatsapp.net", Name: "Nia", Activity: 1_792_310_010},
	}
	if got.Link != queue.Linked || got.QRCode != "" {
		t.Fatalf("state = %+v, want linked and no code", got)
	}
	if !sameChats(got.Chats, want) {
		t.Fatalf("chats = %+v, want %+v", got.Chats, want)
	}
}

func seedLegacyChats(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`
CREATE TABLE chats (
	address TEXT PRIMARY KEY,
	kind TEXT NOT NULL,
	contact_name TEXT NOT NULL DEFAULT '',
	push_name TEXT NOT NULL DEFAULT '',
	phone TEXT NOT NULL DEFAULT '',
	subject TEXT NOT NULL DEFAULT '',
	display_name TEXT NOT NULL DEFAULT '',
	last_activity INTEGER NOT NULL,
	unread INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE aliases (
	address TEXT PRIMARY KEY,
	canonical TEXT NOT NULL
);
INSERT INTO chats (address, kind, contact_name, push_name, phone, subject, display_name, last_activity)
VALUES
	('15550000001@s.whatsapp.net', 'direct', 'Sam', '', '15550000001', '', 'Sam', 1792310050000),
	('111111@lid', 'direct', '', 'sammy', '', '', 'sammy', 1792310040000),
	('120363000@g.us', 'group', '', '', '', 'Family', 'Nope', 1792310030000),
	('222222@lid', 'direct', '', 'Nia', '', '', 'Nia', 1792310010000),
	('status@broadcast', 'direct', '', '', '', 'Status', 'Status', 1792310099000);
INSERT INTO aliases (address, canonical) VALUES
	('111111@lid', '15550000001@s.whatsapp.net'),
	('15550000004@s.whatsapp.net', '222222@lid');
`)
	if err != nil {
		t.Fatal(err)
	}
}

func firstSnapshot() server.Snapshot {
	return server.Snapshot{
		Chats: []server.SnapChat{
			{ID: "111111@lid", Phone: "15550000001@s.whatsapp.net", Hidden: "111111@lid", Activity: 50},
			{ID: "15550000001@s.whatsapp.net", Activity: 5},
			{ID: "15550000002@s.whatsapp.net", DisplayName: "Ada", Activity: 40},
			{ID: "120363000@g.us", Subject: "Family", DisplayName: "Nope", Activity: 30},
			{ID: "15550001111@s.whatsapp.net", Activity: 20},
			{ID: "15550000003@s.whatsapp.net", DisplayName: "NotSaved", Activity: 10},
			{ID: "status@broadcast", Subject: "Status", Activity: 99},
			{ID: "999@newsletter", Subject: "News", Activity: 98},
			{ID: "120363999@g.us", Subject: "Community", Parent: true, Activity: 97},
		},
		Contacts: []server.SnapContact{
			{Phone: "15550000001@s.whatsapp.net", Full: "Sam"},
			{Phone: "15550000002@s.whatsapp.net", Full: "Ada Lovelace", First: "Ada"},
		},
		Pushes: []server.SnapPush{
			{ID: "15550000002@s.whatsapp.net", Name: "ada"},
			{ID: "15550000003@s.whatsapp.net", Name: "Pushy"},
		},
	}
}

func sameChats(got, want []queue.Chat) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
