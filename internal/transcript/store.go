package transcript

import (
	"database/sql"
	"errors"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type ChatKind string

const (
	KindDirect ChatKind = "direct"
	KindGroup  ChatKind = "group"
)

type MsgKind string

const (
	KindText      MsgKind = "text"
	KindImage     MsgKind = "image"
	KindMarker    MsgKind = "marker"
	KindTombstone MsgKind = "tombstone"
)

// Chat is one row in the chat list.
type Chat struct {
	Address      string
	Kind         ChatKind
	DisplayName  string
	LastActivity time.Time
	Unread       int
}

// ChatUpdate is the name and activity information for one address.
type ChatUpdate struct {
	Address      string
	Kind         ChatKind
	ContactName  string
	PushName     string
	Phone        string
	Subject      string
	LastActivity time.Time
	Unread       int
}

// Message is one item in a Chat.
type Message struct {
	Chat        string
	ID          string
	Sender      string
	SenderName  string
	Kind        MsgKind
	Body        string
	QuoteBody   string
	QuoteSender string
	Filename    string
	ImagePath   string
	Receipt     string
	FromMe      bool
	Timestamp   time.Time
	ExpiresAt   *time.Time
}

// Store is the Companion's Transcript.
type Store struct {
	db *sql.DB
}

// Open opens or creates the Transcript at path.
func Open(path string) (*Store, error) {
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

// Close closes the Transcript.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS chats (
	address TEXT PRIMARY KEY,
	kind TEXT NOT NULL,
	contact_name TEXT NOT NULL DEFAULT '',
	push_name TEXT NOT NULL DEFAULT '',
	phone TEXT NOT NULL DEFAULT '',
	subject TEXT NOT NULL DEFAULT '',
	display_name TEXT NOT NULL DEFAULT '',
	last_activity INTEGER NOT NULL,
	unread INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS aliases (
	address TEXT PRIMARY KEY,
	canonical TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS messages (
	chat TEXT NOT NULL,
	id TEXT NOT NULL,
	sender TEXT NOT NULL DEFAULT '',
	sender_name TEXT NOT NULL DEFAULT '',
	kind TEXT NOT NULL,
	body TEXT NOT NULL DEFAULT '',
	quote_body TEXT NOT NULL DEFAULT '',
	quote_sender TEXT NOT NULL DEFAULT '',
	filename TEXT NOT NULL DEFAULT '',
	image_path TEXT NOT NULL DEFAULT '',
	receipt TEXT NOT NULL DEFAULT '',
	from_me INTEGER NOT NULL DEFAULT 0,
	timestamp INTEGER NOT NULL,
	expires_at INTEGER,
	PRIMARY KEY (chat, id)
);`)
	return err
}

// Upsert records a Chat. A later update does not blank a name that is already known.
func (s *Store) Upsert(in ChatUpdate) error {
	address, err := s.canonical(in.Address)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
INSERT INTO chats (address, kind, contact_name, push_name, phone, subject, display_name, last_activity, unread)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(address) DO UPDATE SET
	contact_name = CASE WHEN excluded.contact_name != '' THEN excluded.contact_name ELSE chats.contact_name END,
	push_name = CASE WHEN excluded.push_name != '' THEN excluded.push_name ELSE chats.push_name END,
	phone = CASE WHEN excluded.phone != '' THEN excluded.phone ELSE chats.phone END,
	subject = CASE WHEN excluded.subject != '' THEN excluded.subject ELSE chats.subject END,
	last_activity = CASE WHEN excluded.last_activity > chats.last_activity THEN excluded.last_activity ELSE chats.last_activity END`,
		address, string(in.Kind), in.ContactName, in.PushName, in.Phone, in.Subject,
		displayName(in.Kind, in.ContactName, in.PushName, in.Phone, in.Subject),
		in.LastActivity.UnixMilli(), in.Unread)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
UPDATE chats SET display_name = CASE kind
	WHEN 'group' THEN CASE WHEN subject != '' THEN subject ELSE display_name END
	ELSE CASE
		WHEN contact_name != '' THEN contact_name
		WHEN push_name != '' THEN push_name
		WHEN phone != '' THEN phone
		ELSE display_name
	END
END
WHERE address = ?`, address)
	return err
}

// Associate makes two addresses one Direct chat. preferred is kept when both already exist.
func (s *Store) Associate(preferred, other string) error {
	preferred, err := s.canonical(preferred)
	if err != nil {
		return err
	}
	other, err = s.canonical(other)
	if err != nil {
		return err
	}
	if preferred == "" || other == "" || preferred == other {
		return nil
	}
	prefExists, err := s.hasChat(preferred)
	if err != nil {
		return err
	}
	otherExists, err := s.hasChat(other)
	if err != nil {
		return err
	}
	switch {
	case prefExists && otherExists:
		return s.Merge(preferred, other)
	case otherExists:
		return s.alias(preferred, other)
	default:
		return s.alias(other, preferred)
	}
}

// SetNames fills a contact name or push name on a Chat that already exists.
func (s *Store) SetNames(address, contact, push string) error {
	address, err := s.canonical(address)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(`
UPDATE chats SET
	contact_name = CASE WHEN ? != '' THEN ? ELSE contact_name END,
	push_name = CASE WHEN ? != '' THEN ? ELSE push_name END
WHERE address = ?`, contact, contact, push, push, address)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil || n == 0 {
		return err
	}
	_, err = s.db.Exec(`
UPDATE chats SET display_name = CASE kind
	WHEN 'group' THEN CASE WHEN subject != '' THEN subject ELSE display_name END
	ELSE CASE
		WHEN contact_name != '' THEN contact_name
		WHEN push_name != '' THEN push_name
		WHEN phone != '' THEN phone
		ELSE display_name
	END
END
WHERE address = ?`, address)
	return err
}

func (s *Store) hasChat(address string) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM chats WHERE address = ?`, address).Scan(&n)
	return n > 0, err
}

func (s *Store) alias(address, canonical string) error {
	if address == canonical {
		return nil
	}
	_, err := s.db.Exec(`
INSERT INTO aliases (address, canonical) VALUES (?, ?)
ON CONFLICT(address) DO UPDATE SET canonical = excluded.canonical`, address, canonical)
	return err
}

// Merge makes drop and keep one Direct chat. keep is the address that remains.
func (s *Store) Merge(keep, drop string) error {
	keep, err := s.canonical(keep)
	if err != nil {
		return err
	}
	drop, err = s.canonical(drop)
	if err != nil {
		return err
	}
	if keep == drop {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
UPDATE chats SET
	contact_name = CASE WHEN contact_name != '' THEN contact_name ELSE (SELECT contact_name FROM chats WHERE address = ?) END,
	push_name = CASE WHEN push_name != '' THEN push_name ELSE (SELECT push_name FROM chats WHERE address = ?) END,
	phone = CASE WHEN phone != '' THEN phone ELSE (SELECT phone FROM chats WHERE address = ?) END,
	last_activity = MAX(last_activity, COALESCE((SELECT last_activity FROM chats WHERE address = ?), last_activity)),
	unread = unread + COALESCE((SELECT unread FROM chats WHERE address = ?), 0)
WHERE address = ?`, drop, drop, drop, drop, drop, keep); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE OR IGNORE messages SET chat = ? WHERE chat = ?`, keep, drop); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM messages WHERE chat = ?`, drop); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM chats WHERE address = ?`, drop); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE aliases SET canonical = ? WHERE canonical = ?`, keep, drop); err != nil {
		return err
	}
	if _, err := tx.Exec(`
INSERT INTO aliases (address, canonical) VALUES (?, ?)
ON CONFLICT(address) DO UPDATE SET canonical = excluded.canonical`, drop, keep); err != nil {
		return err
	}
	if _, err := tx.Exec(`
UPDATE chats SET display_name = CASE kind
	WHEN 'group' THEN CASE WHEN subject != '' THEN subject ELSE display_name END
	ELSE CASE
		WHEN contact_name != '' THEN contact_name
		WHEN push_name != '' THEN push_name
		WHEN phone != '' THEN phone
		ELSE display_name
	END
END
WHERE address = ?`, keep); err != nil {
		return err
	}
	return tx.Commit()
}

// Put stores a Message under the canonical Chat address.
func (s *Store) Put(m Message) error {
	chat, err := s.canonical(m.Chat)
	if err != nil {
		return err
	}
	var expires any
	if m.ExpiresAt != nil {
		expires = m.ExpiresAt.UnixMilli()
	}
	fromMe := 0
	if m.FromMe {
		fromMe = 1
	}
	_, err = s.db.Exec(`
INSERT INTO messages (chat, id, sender, sender_name, kind, body, quote_body, quote_sender, filename, image_path, receipt, from_me, timestamp, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(chat, id) DO UPDATE SET
	body = excluded.body,
	kind = excluded.kind,
	sender_name = CASE WHEN excluded.sender_name != '' THEN excluded.sender_name ELSE messages.sender_name END,
	quote_body = CASE WHEN excluded.quote_body != '' THEN excluded.quote_body ELSE messages.quote_body END,
	quote_sender = CASE WHEN excluded.quote_sender != '' THEN excluded.quote_sender ELSE messages.quote_sender END,
	filename = CASE WHEN excluded.filename != '' THEN excluded.filename ELSE messages.filename END,
	image_path = CASE WHEN excluded.image_path != '' THEN excluded.image_path ELSE messages.image_path END,
	receipt = CASE WHEN excluded.receipt != '' THEN excluded.receipt ELSE messages.receipt END`,
		chat, m.ID, m.Sender, m.SenderName, string(m.Kind), m.Body, m.QuoteBody, m.QuoteSender,
		m.Filename, m.ImagePath, m.Receipt, fromMe, m.Timestamp.UnixMilli(), expires)
	return err
}

// List returns Chats, newest activity first.
func (s *Store) List() ([]Chat, error) {
	rows, err := s.db.Query(`
SELECT address, kind, display_name, last_activity, unread
FROM chats
ORDER BY last_activity DESC, address`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Chat
	for rows.Next() {
		var chat Chat
		var kind string
		var activity int64
		if err := rows.Scan(&chat.Address, &kind, &chat.DisplayName, &activity, &chat.Unread); err != nil {
			return nil, err
		}
		chat.Kind = ChatKind(kind)
		chat.LastActivity = time.UnixMilli(activity).UTC()
		out = append(out, chat)
	}
	return out, rows.Err()
}

// Messages returns the Messages of a Chat, oldest first. address may be an alias.
func (s *Store) Messages(address string) ([]Message, error) {
	chat, err := s.canonical(address)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`
SELECT chat, id, sender, sender_name, kind, body, quote_body, quote_sender, filename, image_path, receipt, from_me, timestamp, expires_at
FROM messages WHERE chat = ? ORDER BY timestamp, id`, chat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		var kind string
		var fromMe int
		var ts int64
		var expires sql.NullInt64
		if err := rows.Scan(&m.Chat, &m.ID, &m.Sender, &m.SenderName, &kind, &m.Body, &m.QuoteBody, &m.QuoteSender, &m.Filename, &m.ImagePath, &m.Receipt, &fromMe, &ts, &expires); err != nil {
			return nil, err
		}
		m.Kind = MsgKind(kind)
		m.FromMe = fromMe == 1
		m.Timestamp = time.UnixMilli(ts).UTC()
		if expires.Valid {
			exp := time.UnixMilli(expires.Int64).UTC()
			m.ExpiresAt = &exp
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) canonical(address string) (string, error) {
	var canonical string
	err := s.db.QueryRow(`SELECT canonical FROM aliases WHERE address = ?`, address).Scan(&canonical)
	if errors.Is(err, sql.ErrNoRows) {
		return address, nil
	}
	return canonical, err
}

func displayName(kind ChatKind, contact, push, phone, subject string) string {
	if kind == KindGroup {
		return subject
	}
	if contact != "" {
		return contact
	}
	if push != "" {
		return push
	}
	return phone
}
