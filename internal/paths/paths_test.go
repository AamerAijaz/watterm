package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLogoutRemovesSessionAndKeepsTranscript(t *testing.T) {
	lay, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(lay.Session, "session")
	write(lay.Session+"-wal", "wal")
	write(lay.Transcript, "transcript")
	write(filepath.Join(lay.Media, "pic.jpg"), "image")

	if err := RemoveSession(lay); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lay.Session); !os.IsNotExist(err) {
		t.Fatalf("session stat err = %v, want not exist", err)
	}
	if _, err := os.Stat(lay.Session + "-wal"); !os.IsNotExist(err) {
		t.Fatalf("session wal stat err = %v, want not exist", err)
	}
	body, err := os.ReadFile(lay.Transcript)
	if err != nil || string(body) != "transcript" {
		t.Fatalf("transcript = %q, %v", body, err)
	}
	body, err = os.ReadFile(filepath.Join(lay.Media, "pic.jpg"))
	if err != nil || string(body) != "image" {
		t.Fatalf("media = %q, %v", body, err)
	}
}
