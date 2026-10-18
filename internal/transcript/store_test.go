package transcript

import (
	"path/filepath"
	"testing"
	"time"
)

func TestListOrdersByLatestActivityAndSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "transcript.db")
	store := openStore(t, path)

	older := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	must(t, store.Upsert(ChatUpdate{
		Address:      "111@s.whatsapp.net",
		Kind:         KindDirect,
		PushName:     "Ada",
		LastActivity: older,
	}))
	must(t, store.Upsert(ChatUpdate{
		Address:      "group@g.us",
		Kind:         KindGroup,
		Subject:      "Weekend",
		LastActivity: newer,
	}))

	store.Close()
	store = openStore(t, path)
	t.Cleanup(func() { store.Close() })

	got := mustList(t, store)
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].DisplayName != "Weekend" || got[1].DisplayName != "Ada" {
		t.Fatalf("order = %#v", got)
	}
}

func TestDisplayNameFallsBack(t *testing.T) {
	store := openStore(t, filepath.Join(t.TempDir(), "transcript.db"))
	t.Cleanup(func() { store.Close() })
	now := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)

	must(t, store.Upsert(ChatUpdate{Address: "a@s.whatsapp.net", Kind: KindDirect, ContactName: "Ada Lovelace", PushName: "Ada", Phone: "1555", LastActivity: now}))
	must(t, store.Upsert(ChatUpdate{Address: "b@s.whatsapp.net", Kind: KindDirect, PushName: "Bea", Phone: "1666", LastActivity: now}))
	must(t, store.Upsert(ChatUpdate{Address: "c@s.whatsapp.net", Kind: KindDirect, Phone: "1777", LastActivity: now}))

	got := mustList(t, store)
	names := map[string]string{}
	for _, chat := range got {
		names[chat.Address] = chat.DisplayName
	}
	if names["a@s.whatsapp.net"] != "Ada Lovelace" || names["b@s.whatsapp.net"] != "Bea" || names["c@s.whatsapp.net"] != "1777" {
		t.Fatalf("names = %#v", names)
	}
}

func TestTwoAddressesForOnePersonAreOneDirectChat(t *testing.T) {
	store := openStore(t, filepath.Join(t.TempDir(), "transcript.db"))
	t.Cleanup(func() { store.Close() })
	when := time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC)

	must(t, store.Upsert(ChatUpdate{Address: "111@s.whatsapp.net", Kind: KindDirect, ContactName: "Ada", LastActivity: when}))
	must(t, store.Put(Message{Chat: "111@s.whatsapp.net", ID: "m1", Kind: KindText, Body: "from phone", Timestamp: when}))
	must(t, store.Upsert(ChatUpdate{Address: "111@lid", Kind: KindDirect, PushName: "Ada", LastActivity: when.Add(time.Minute)}))
	must(t, store.Put(Message{Chat: "111@lid", ID: "m2", Kind: KindText, Body: "from lid", Timestamp: when.Add(time.Minute)}))

	must(t, store.Merge("111@s.whatsapp.net", "111@lid"))

	got := mustList(t, store)
	if len(got) != 1 {
		t.Fatalf("chats = %#v", got)
	}
	if got[0].Address != "111@s.whatsapp.net" || got[0].DisplayName != "Ada" {
		t.Fatalf("chat = %#v", got[0])
	}
	msgs := mustMessages(t, store, "111@s.whatsapp.net")
	if len(msgs) != 2 || msgs[0].Body != "from phone" || msgs[1].Body != "from lid" {
		t.Fatalf("messages = %#v", msgs)
	}
	if again := mustMessages(t, store, "111@lid"); len(again) != 2 {
		t.Fatalf("alias lookup = %#v", again)
	}
}

func openStore(t *testing.T, path string) *Store {
	t.Helper()
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func mustList(t *testing.T, store *Store) []Chat {
	t.Helper()
	got, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func mustMessages(t *testing.T, store *Store, address string) []Message {
	t.Helper()
	got, err := store.Messages(address)
	if err != nil {
		t.Fatal(err)
	}
	return got
}
