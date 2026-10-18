package companion

import (
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"

	"go.mau.fi/whatsmeow/proto/waHistorySync"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/AamerAijaz/watterm/internal/transcript"
)

func TestApplySyncListsDirectChatsAndGroupsAndMergesLID(t *testing.T) {
	store, err := transcript.Open(filepath.Join(t.TempDir(), "transcript.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	err = ApplySync(store, &events.HistorySync{Data: &waHistorySync.HistorySync{
		Conversations: []*waHistorySync.Conversation{
			{
				ID:                    proto.String("111@lid"),
				DisplayName:           proto.String("Ada Lovelace"),
				Name:                  proto.String("Ada"),
				ConversationTimestamp: proto.Uint64(1_700_000_000),
				PnJID:                 proto.String("15551212@s.whatsapp.net"),
				LidJID:                proto.String("111@lid"),
			},
			{
				ID:                    proto.String("777@g.us"),
				Name:                  proto.String("Weekend"),
				ConversationTimestamp: proto.Uint64(1_700_000_100),
			},
			{
				ID:   proto.String("status@broadcast"),
				Name: proto.String("Status"),
			},
			{
				ID:                    proto.String("15550001@s.whatsapp.net"),
				ConversationTimestamp: proto.Uint64(1_600_000_000),
			},
			{
				ID:                    proto.String("abc@lid"),
				ConversationTimestamp: proto.Uint64(1_600_000_050),
			},
		},
		PhoneNumberToLidMappings: []*waHistorySync.PhoneNumberToLIDMapping{
			{
				PnJID:  proto.String("15550001@s.whatsapp.net"),
				LidJID: proto.String("abc@lid"),
			},
		},
		Pushnames: []*waHistorySync.Pushname{
			{
				ID:       proto.String("15550001@s.whatsapp.net"),
				Pushname: proto.String("Bea"),
			},
		},
		InlineContacts: []*waHistorySync.InlineContact{
			{
				PnJID:    proto.String("15551212@s.whatsapp.net"),
				FullName: proto.String("Ada Lovelace"),
			},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}

	got, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("chats = %#v", got)
	}
	if got[0].DisplayName != "Weekend" || got[0].Kind != transcript.KindGroup {
		t.Fatalf("first = %#v", got[0])
	}
	if got[1].DisplayName != "Ada Lovelace" || got[1].Address != "15551212@s.whatsapp.net" || got[1].Kind != transcript.KindDirect {
		t.Fatalf("second = %#v", got[1])
	}
	if got[2].DisplayName != "Bea" || got[2].Address != "15550001@s.whatsapp.net" {
		t.Fatalf("third = %#v", got[2])
	}
}
