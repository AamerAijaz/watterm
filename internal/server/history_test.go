package server

import (
	"testing"

	"go.mau.fi/whatsmeow/proto/waHistorySync"
)

func TestSnapshotFromHistoryReadsTheChatList(t *testing.T) {
	name := "Family"
	id := "120363000@g.us"
	full := "Ada Lovelace"
	phone := "15550000002@s.whatsapp.net"
	hidden := "222@lid"
	pushID := "15550000003@s.whatsapp.net"
	pushName := "Pushy"
	var activity uint64 = 30
	data := &waHistorySync.HistorySync{
		Conversations: []*waHistorySync.Conversation{{
			ID:                    &id,
			Name:                  &name,
			ConversationTimestamp: &activity,
		}},
		Pushnames: []*waHistorySync.Pushname{{
			ID:       &pushID,
			Pushname: &pushName,
		}},
		InlineContacts: []*waHistorySync.InlineContact{{
			PnJID:    &phone,
			FullName: &full,
		}},
		PhoneNumberToLidMappings: []*waHistorySync.PhoneNumberToLIDMapping{{
			PnJID:  &phone,
			LidJID: &hidden,
		}},
	}

	snap := snapshotFromHistory(data)
	if len(snap.Chats) != 1 || snap.Chats[0].Subject != "Family" || snap.Chats[0].Activity != 30 {
		t.Fatalf("chats = %+v", snap.Chats)
	}
	if len(snap.Pushes) != 1 || snap.Pushes[0].Name != "Pushy" {
		t.Fatalf("pushes = %+v", snap.Pushes)
	}
	if len(snap.Contacts) != 1 || snap.Contacts[0].Full != "Ada Lovelace" {
		t.Fatalf("contacts = %+v", snap.Contacts)
	}
	if len(snap.Mappings) != 1 || snap.Mappings[0].Hidden != "222@lid" {
		t.Fatalf("mappings = %+v", snap.Mappings)
	}
}
