package server

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"

	"github.com/AamerAijaz/watterm/internal/queue"
)

// Transcript is the Companion's own record of the chat list, separate from the Session.
type Transcript interface {
	Load(ctx context.Context) ([]queue.Chat, error)
	Save(ctx context.Context, chats []queue.Chat) error
}

type noopChats struct{}

func (noopChats) Load(context.Context) ([]queue.Chat, error) { return nil, nil }
func (noopChats) Save(context.Context, []queue.Chat) error   { return nil }

// TranscriptFile is the sqlite Transcript.
type TranscriptFile struct {
	db *sql.DB
}

// OpenTranscript opens transcript.db, creating it if needed.
func OpenTranscript(ctx context.Context, path string) (*TranscriptFile, error) {
	dsn := "file:" + filepath.ToSlash(path) + "?_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	_, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS chat (
		address TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		activity INTEGER NOT NULL
	)`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &TranscriptFile{db: db}, nil
}

// Close closes the file.
func (f *TranscriptFile) Close() error {
	if f == nil || f.db == nil {
		return nil
	}
	return f.db.Close()
}

// Load returns the stored list, newest activity first.
// An earlier Transcript wrote that list into chats. Those rows are copied into chat
// once, so a restart shows them before another snapshot arrives.
func (f *TranscriptFile) Load(ctx context.Context) ([]queue.Chat, error) {
	chats, err := f.loadStored(ctx)
	if err != nil || len(chats) > 0 {
		return chats, err
	}
	chats, err = f.loadLegacy(ctx)
	if err != nil || len(chats) == 0 {
		return chats, err
	}
	if err := f.Save(ctx, chats); err != nil {
		return nil, err
	}
	return chats, nil
}

func (f *TranscriptFile) loadStored(ctx context.Context) ([]queue.Chat, error) {
	rows, err := f.db.QueryContext(ctx, `SELECT address, name, activity FROM chat ORDER BY activity DESC, address ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var chats []queue.Chat
	for rows.Next() {
		var chat queue.Chat
		if err := rows.Scan(&chat.Address, &chat.Name, &chat.Activity); err != nil {
			return nil, err
		}
		chats = append(chats, chat)
	}
	return chats, rows.Err()
}

type legacyRow struct {
	address  string
	kind     string
	contact  string
	push     string
	phone    string
	subject  string
	display  string
	activity int64
}

func (f *TranscriptFile) loadLegacy(ctx context.Context) ([]queue.Chat, error) {
	ok, err := f.tableExists(ctx, "chats")
	if err != nil || !ok {
		return nil, err
	}
	rows, err := f.db.QueryContext(ctx, `SELECT address, kind, contact_name, push_name, phone, subject, display_name, last_activity FROM chats`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stored []legacyRow
	for rows.Next() {
		var row legacyRow
		if err := rows.Scan(&row.address, &row.kind, &row.contact, &row.push, &row.phone, &row.subject, &row.display, &row.activity); err != nil {
			return nil, err
		}
		stored = append(stored, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	aliases, err := f.legacyAliases(ctx)
	if err != nil {
		return nil, err
	}
	return foldLegacy(stored, aliases), nil
}

func (f *TranscriptFile) legacyAliases(ctx context.Context) (map[string][]string, error) {
	ok, err := f.tableExists(ctx, "aliases")
	if err != nil || !ok {
		return nil, err
	}
	rows, err := f.db.QueryContext(ctx, `SELECT address, canonical FROM aliases`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	adj := map[string][]string{}
	for rows.Next() {
		var from, to string
		if err := rows.Scan(&from, &to); err != nil {
			return nil, err
		}
		if from == "" || to == "" || from == to {
			continue
		}
		adj[from] = append(adj[from], to)
		adj[to] = append(adj[to], from)
	}
	return adj, rows.Err()
}

func (f *TranscriptFile) tableExists(ctx context.Context, name string) (bool, error) {
	var got string
	err := f.db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&got)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func foldLegacy(stored []legacyRow, aliases map[string][]string) []queue.Chat {
	by := map[string]*queue.Chat{}
	rank := map[string]int{}
	for _, row := range stored {
		if skippedAddress(row.address) {
			continue
		}
		address := row.address
		group := row.kind == "group" || strings.HasSuffix(row.address, "@g.us")
		if !group {
			if phone := phoneAddress(row.address, aliases); phone != "" {
				address = phone
			}
		}
		name, pri := legacyName(row, address, group)
		activity := unixSeconds(row.activity)
		if existing := by[address]; existing != nil {
			if activity > existing.Activity {
				existing.Activity = activity
			}
			if pri > rank[address] {
				existing.Name = name
				rank[address] = pri
			}
			continue
		}
		by[address] = &queue.Chat{Address: address, Name: name, Activity: activity}
		rank[address] = pri
	}
	out := make([]queue.Chat, 0, len(by))
	for _, chat := range by {
		out = append(out, *chat)
	}
	return sorted(out)
}

func legacyName(row legacyRow, address string, group bool) (string, int) {
	if group {
		if row.subject != "" {
			return row.subject, 4
		}
		if row.display != "" {
			return row.display, 1
		}
		return userOf(address), 0
	}
	if row.contact != "" {
		return row.contact, 4
	}
	if row.push != "" {
		return row.push, 3
	}
	if row.phone != "" {
		return row.phone, 2
	}
	if row.display != "" {
		return row.display, 1
	}
	return userOf(address), 0
}

func phoneAddress(start string, aliases map[string][]string) string {
	seen := map[string]bool{start: true}
	q := []string{start}
	var phones []string
	for len(q) > 0 {
		cur := q[0]
		q = q[1:]
		if strings.HasSuffix(cur, "@s.whatsapp.net") {
			phones = append(phones, cur)
		}
		for _, next := range aliases[cur] {
			if !seen[next] {
				seen[next] = true
				q = append(q, next)
			}
		}
	}
	if len(phones) == 0 {
		return ""
	}
	best := phones[0]
	for _, phone := range phones[1:] {
		if phone < best {
			best = phone
		}
	}
	return best
}

func skippedAddress(address string) bool {
	return strings.HasSuffix(address, "@broadcast") || strings.HasSuffix(address, "@newsletter")
}

func userOf(address string) string {
	user, _, _ := strings.Cut(address, "@")
	return user
}

func unixSeconds(v int64) int64 {
	if v >= 100_000_000_000 {
		return v / 1000
	}
	return v
}

// Save replaces the stored list.
func (f *TranscriptFile) Save(ctx context.Context, chats []queue.Chat) error {
	tx, err := f.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM chat`); err != nil {
		return err
	}
	for _, chat := range chats {
		if _, err := tx.ExecContext(ctx, `INSERT INTO chat (address, name, activity) VALUES (?, ?, ?)`, chat.Address, chat.Name, chat.Activity); err != nil {
			return err
		}
	}
	return tx.Commit()
}
