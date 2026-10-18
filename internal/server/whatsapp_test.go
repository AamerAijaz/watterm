package server

import (
	"context"
	"path/filepath"
	"testing"
)

func TestEmptySessionFileIsNotPaired(t *testing.T) {
	wa := NewWhatsApp(filepath.Join(t.TempDir(), "session.db"))
	stored, err := wa.sessionStored(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = wa.Close() })
	if stored {
		t.Fatal("a new Session file is already paired")
	}
}
