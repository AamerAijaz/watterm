package server

import "go.mau.fi/whatsmeow/proto/waHistorySync"

func snapshotFromHistory(data *waHistorySync.HistorySync) Snapshot {
	if data == nil {
		return Snapshot{}
	}
	var snap Snapshot
	for _, chat := range data.GetConversations() {
		activity := chat.GetConversationTimestamp()
		if activity == 0 {
			activity = chat.GetLastMsgTimestamp()
		}
		snap.Chats = append(snap.Chats, SnapChat{
			ID:          chat.GetID(),
			Subject:     chat.GetName(),
			DisplayName: chat.GetDisplayName(),
			Phone:       chat.GetPnJID(),
			Hidden:      chat.GetLidJID(),
			Activity:    int64(activity),
			Parent:      chat.GetIsParentGroup(),
		})
	}
	for _, push := range data.GetPushnames() {
		snap.Pushes = append(snap.Pushes, SnapPush{ID: push.GetID(), Name: push.GetPushname()})
	}
	for _, contact := range data.GetInlineContacts() {
		snap.Contacts = append(snap.Contacts, SnapContact{
			Phone:  contact.GetPnJID(),
			Hidden: contact.GetLidJID(),
			Full:   contact.GetFullName(),
			First:  contact.GetFirstName(),
		})
	}
	for _, mapping := range data.GetPhoneNumberToLidMappings() {
		snap.Mappings = append(snap.Mappings, SnapMap{Phone: mapping.GetPnJID(), Hidden: mapping.GetLidJID()})
	}
	return snap
}
