package companion

import (
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/AamerAijaz/watterm/internal/link"
)

func mapEvent(evt any) (any, bool) {
	switch evt.(type) {
	case *events.Connected:
		return link.Connected{}, true
	case *events.Disconnected:
		return link.Disconnected{}, true
	case *events.LoggedOut:
		return link.LoggedOut{}, true
	default:
		return nil, false
	}
}

func mapQR(item whatsmeow.QRChannelItem) (any, bool) {
	if item.Event != whatsmeow.QRChannelEventCode || item.Code == "" {
		return nil, false
	}
	return link.QR{Code: item.Code}, true
}
