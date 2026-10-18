package instance

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecondAcquireSaysAlreadyRunning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "companion.lock")

	first, err := Acquire(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { first.Release() })

	_, err = Acquire(path)
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("err = %v, want already running", err)
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Fatalf("message = %q, want it to say one is already running", err.Error())
	}
}
