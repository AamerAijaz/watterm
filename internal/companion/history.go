package companion

import (
	"strings"
	"time"

	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/AamerAijaz/watterm/internal/transcript"
)

// ApplySync writes the initial history push into the Transcript.
// Broadcasts and community containers are skipped. A phone address and a LID for the same person become one Direct chat.
func ApplySync(store *transcript.Store, sync *events.HistorySync) error {
	if store == nil || sync == nil || sync.Data == nil {
		return nil
	}
	for _, conv := range sync.Data.GetConversations() {
		if err := applyConversation(store, conv); err != nil {
			return err
		}
	}
	for _, mapping := range sync.Data.GetPhoneNumberToLidMappings() {
		if err := store.Associate(mapping.GetPnJID(), mapping.GetLidJID()); err != nil {
			return err
		}
	}
	for _, contact := range sync.Data.GetInlineContacts() {
		name := firstText(contact.GetFullName(), contact.GetFirstName())
		for _, address := range []string{contact.GetPnJID(), contact.GetLidJID()} {
			if address == "" {
				continue
			}
			if err := store.SetNames(address, name, ""); err != nil {
				return err
			}
		}
	}
	for _, push := range sync.Data.GetPushnames() {
		if err := store.SetNames(push.GetID(), "", push.GetPushname()); err != nil {
			return err
		}
	}
	return nil
}

func applyConversation(store *transcript.Store, conv *waHistorySync.Conversation) error {
	if conv == nil || conv.GetIsParentGroup() {
		return nil
	}
	id := conv.GetID()
	if id == "" || skippedAddress(id) {
		return nil
	}
	jid, err := types.ParseJID(id)
	if err != nil {
		return nil
	}
	when := syncTime(conv.GetConversationTimestamp())
	if when.IsZero() {
		when = syncTime(conv.GetLastMsgTimestamp())
	}
	pn := conv.GetPnJID()
	lid := conv.GetLidJID()

	update := transcript.ChatUpdate{Address: id, LastActivity: when, Unread: int(conv.GetUnreadCount())}
	if jid.Server == types.GroupServer {
		update.Kind = transcript.KindGroup
		update.Subject = firstText(conv.GetName(), conv.GetDisplayName())
	} else {
		update.Kind = transcript.KindDirect
		update.ContactName = firstText(conv.GetDisplayName(), conv.GetName())
		update.Phone = phoneOf(pn, jid)
		if pn != "" {
			update.Address = pn
		}
	}
	if err := store.Upsert(update); err != nil {
		return err
	}
	keep := update.Address
	for _, alt := range []string{id, lid} {
		if alt == "" || alt == keep || skippedAddress(alt) {
			continue
		}
		altUpdate := update
		altUpdate.Address = alt
		altUpdate.Unread = 0
		if err := store.Upsert(altUpdate); err != nil {
			return err
		}
		if err := store.Merge(keep, alt); err != nil {
			return err
		}
	}
	return nil
}

func skippedAddress(address string) bool {
	jid, err := types.ParseJID(address)
	if err != nil {
		return true
	}
	return jid.Server == types.BroadcastServer || jid.Server == types.NewsletterServer
}

func phoneOf(pn string, jid types.JID) string {
	if pn != "" {
		if parsed, err := types.ParseJID(pn); err == nil && parsed.Server == types.DefaultUserServer {
			return parsed.User
		}
	}
	if jid.Server == types.DefaultUserServer {
		return jid.User
	}
	return ""
}

func firstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func syncTime(raw uint64) time.Time {
	if raw == 0 {
		return time.Time{}
	}
	if raw < 1_000_000_000_000 {
		return time.Unix(int64(raw), 0).UTC()
	}
	return time.UnixMilli(int64(raw)).UTC()
}
