package server

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/AamerAijaz/watterm/internal/queue"
	"go.mau.fi/whatsmeow/types/events"
)

func TestEmptySessionFileIsNotPaired(t *testing.T) {
	wa := NewWhatsApp(filepath.Join(t.TempDir(), "session.db"))
	stored, err := wa.sessionStored(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = wa.Close() })
	if stored {
		t.Fatal("a new Session file is already paired")
	}
}

func TestAQuietNetworkDropIsReconnecting(t *testing.T) {
	status, ok := linkStatus(&events.KeepAliveTimeout{})
	if !ok || status != queue.Reconnecting {
		t.Fatalf("keepalive timeout = %v, %v; want reconnecting", status, ok)
	}
	status, ok = linkStatus(&events.Disconnected{})
	if !ok || status != queue.Reconnecting {
		t.Fatalf("disconnect = %v, %v; want reconnecting", status, ok)
	}
	status, ok = linkStatus(&events.KeepAliveRestored{})
	if !ok || status != queue.Linked {
		t.Fatalf("keepalive restored = %v, %v; want linked", status, ok)
	}
	status, ok = linkStatus(&events.Connected{})
	if !ok || status != queue.Linked {
		t.Fatalf("connected = %v, %v; want linked", status, ok)
	}
	if _, ok = linkStatus(&events.LoggedOut{}); ok {
		t.Fatal("logout was treated as a network drop")
	}
}
