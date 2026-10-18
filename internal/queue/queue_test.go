package queue_test

import (
	"context"
	"testing"

	"github.com/AamerAijaz/watterm/internal/queue"
)

func TestNextStateReturnsThePublishedState(t *testing.T) {
	ui, srv := queue.New()

	want := queue.State{Link: queue.Pairing}
	if err := srv.Publish(want); err != nil {
		t.Fatal(err)
	}

	got, err := ui.NextState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("NextState() = %+v, want %+v", got, want)
	}
}

func TestNewerUnreadStateReplacesTheOlderOne(t *testing.T) {
	ui, srv := queue.New()

	if err := srv.Publish(queue.State{Link: queue.Pairing, QRCode: "stale"}); err != nil {
		t.Fatal(err)
	}
	if err := srv.Publish(queue.State{Link: queue.Pairing, QRCode: "fresh"}); err != nil {
		t.Fatal(err)
	}

	got, err := ui.NextState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.QRCode != "fresh" {
		t.Fatalf("QRCode = %q, want fresh", got.QRCode)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ui.NextState(ctx); err == nil {
		t.Fatal("NextState returned a second state; the stale code was still queued")
	}
}
