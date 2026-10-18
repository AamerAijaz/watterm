package companion

import (
	"testing"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/AamerAijaz/watterm/internal/link"
)

func TestMapLinkEvents(t *testing.T) {
	connected, ok := mapEvent(&events.Connected{})
	if !ok || connected != (link.Connected{}) {
		t.Fatalf("connected = %#v, %v", connected, ok)
	}
	disconnected, ok := mapEvent(&events.Disconnected{})
	if !ok || disconnected != (link.Disconnected{}) {
		t.Fatalf("disconnected = %#v, %v", disconnected, ok)
	}
	loggedOut, ok := mapEvent(&events.LoggedOut{})
	if !ok || loggedOut != (link.LoggedOut{}) {
		t.Fatalf("logged out = %#v, %v", loggedOut, ok)
	}
	if _, ok := mapEvent(&events.Message{}); ok {
		t.Fatal("a message is not a link event")
	}
}

func TestMapQRCode(t *testing.T) {
	ev, ok := mapQR(whatsmeow.QRChannelItem{Event: whatsmeow.QRChannelEventCode, Code: "pair-me"})
	if !ok {
		t.Fatal("expected a QR event")
	}
	qr, ok := ev.(link.QR)
	if !ok || qr.Code != "pair-me" {
		t.Fatalf("qr = %#v", ev)
	}
	if _, ok := mapQR(whatsmeow.QRChannelTimeout); ok {
		t.Fatal("timeout is not a pairing code")
	}
}
