package server

import (
	"sort"

	"go.mau.fi/whatsmeow/types"

	"github.com/AamerAijaz/watterm/internal/queue"
)

// Snapshot is one push of the chat list from WhatsApp.
type Snapshot struct {
	Chats    []SnapChat
	Contacts []SnapContact
	Pushes   []SnapPush
	Mappings []SnapMap
}

// SnapChat is one conversation in that push.
type SnapChat struct {
	ID          string
	Subject     string
	DisplayName string
	Phone       string
	Hidden      string
	Activity    int64
	Parent      bool
}

// SnapContact is a saved contact name.
type SnapContact struct {
	Phone  string
	Hidden string
	Full   string
	First  string
}

// SnapPush is a push name.
type SnapPush struct {
	ID   string
	Name string
}

// SnapMap links a phone-number address to a hidden address.
type SnapMap struct {
	Phone  string
	Hidden string
}

func (s Snapshot) empty() bool {
	return len(s.Chats) == 0 && len(s.Contacts) == 0 && len(s.Pushes) == 0 && len(s.Mappings) == 0
}

// fold merges a snapshot into the stored list.
// Newest activity comes first. A phone address and a hidden address for the same person are one row.
func fold(prev []queue.Chat, snap Snapshot) []queue.Chat {
	if snap.empty() {
		return sorted(prev)
	}

	toPhone := map[string]string{}
	addMap := func(phone, hidden string) {
		pj, okp := cleanJID(phone)
		hj, okh := cleanJID(hidden)
		if okp && okh && pj.Server == types.DefaultUserServer {
			toPhone[hj.String()] = pj.String()
		}
	}
	for _, m := range snap.Mappings {
		addMap(m.Phone, m.Hidden)
	}
	for _, c := range snap.Contacts {
		addMap(c.Phone, c.Hidden)
	}
	for _, c := range snap.Chats {
		addMap(c.Phone, c.Hidden)
		if id, ok := cleanJID(c.ID); ok && id.Server == types.HiddenUserServer {
			addMap(c.Phone, c.ID)
		}
		if id, ok := cleanJID(c.ID); ok && id.Server == types.DefaultUserServer {
			addMap(c.ID, c.Hidden)
		}
	}

	contact := map[string]string{}
	for _, c := range snap.Contacts {
		name := c.Full
		if name == "" {
			name = c.First
		}
		if name == "" {
			continue
		}
		for _, raw := range []string{c.Phone, c.Hidden} {
			if jid, ok := cleanJID(raw); ok {
				contact[jid.String()] = name
			}
		}
	}
	push := map[string]string{}
	for _, p := range snap.Pushes {
		if p.Name == "" || p.Name == "-" {
			continue
		}
		if jid, ok := cleanJID(p.ID); ok {
			push[jid.String()] = p.Name
		}
	}

	rows := map[string]*acc{}
	put := func(row *acc) {
		if existing := rows[row.address]; existing != nil {
			existing.group = existing.group || row.group
			if row.subject != "" {
				existing.subject = row.subject
			}
			if row.display != "" {
				existing.display = row.display
			}
			if row.activity > existing.activity {
				existing.activity = row.activity
			}
			if row.prevName != "" && existing.prevName == "" {
				existing.prevName = row.prevName
			}
			existing.ids = append(existing.ids, row.ids...)
			return
		}
		rows[row.address] = row
	}
	for _, c := range prev {
		addr := c.Address
		if pn, ok := toPhone[addr]; ok {
			addr = pn
		}
		put(&acc{address: addr, prevName: c.Name, activity: c.Activity})
	}
	for _, chat := range snap.Chats {
		id, ok := cleanJID(chat.ID)
		if !ok || chat.Parent {
			continue
		}
		if id.Server == types.BroadcastServer || id.Server == types.NewsletterServer {
			continue
		}
		group := id.Server == types.GroupServer
		address := id.String()
		if !group {
			address = canonicalDirect(id, chat.Phone, toPhone)
		}
		row := &acc{address: address, group: group, activity: chat.Activity}
		if chat.Subject != "" {
			row.subject = chat.Subject
		}
		row.ids = append(row.ids, id.String())
		if pj, ok := cleanJID(chat.Phone); ok {
			row.ids = append(row.ids, pj.String())
		}
		if hj, ok := cleanJID(chat.Hidden); ok {
			row.ids = append(row.ids, hj.String())
		}
		put(row)
	}

	out := make([]queue.Chat, 0, len(rows))
	for _, row := range rows {
		name := row.display
		if row.group {
			if row.subject != "" {
				name = row.subject
			}
		} else {
			name = directName(row, contact, push)
		}
		if name == "" {
			if jid, ok := cleanJID(row.address); ok && jid.User != "" {
				name = jid.User
			} else {
				name = row.address
			}
		}
		out = append(out, queue.Chat{Address: row.address, Name: name, Activity: row.activity})
	}
	return sorted(out)
}

type acc struct {
	address  string
	group    bool
	subject  string
	display  string
	prevName string
	activity int64
	ids      []string
}

func directName(row *acc, contact, push map[string]string) string {
	if name := lookup(contact, row); name != "" {
		return name
	}
	if name := lookup(push, row); name != "" {
		return name
	}
	if jid, ok := cleanJID(row.address); ok && jid.Server == types.DefaultUserServer && jid.User != "" {
		if row.prevName != "" && row.prevName != jid.User {
			return row.prevName
		}
		return jid.User
	}
	if row.prevName != "" {
		return row.prevName
	}
	return ""
}

func lookup(names map[string]string, row *acc) string {
	if name := names[row.address]; name != "" {
		return name
	}
	for _, id := range row.ids {
		if name := names[id]; name != "" {
			return name
		}
	}
	return ""
}

func canonicalDirect(id types.JID, phone string, toPhone map[string]string) string {
	if id.Server == types.DefaultUserServer {
		return id.String()
	}
	if pn, ok := toPhone[id.String()]; ok {
		return pn
	}
	if pj, ok := cleanJID(phone); ok && pj.Server == types.DefaultUserServer {
		return pj.String()
	}
	return id.String()
}

func cleanJID(raw string) (types.JID, bool) {
	if raw == "" {
		return types.JID{}, false
	}
	jid, err := types.ParseJID(raw)
	if err != nil || jid.Server == "" {
		return types.JID{}, false
	}
	return jid.ToNonAD(), true
}

func sorted(in []queue.Chat) []queue.Chat {
	out := append([]queue.Chat(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Activity != out[j].Activity {
			return out[i].Activity > out[j].Activity
		}
		return out[i].Address < out[j].Address
	})
	return out
}
