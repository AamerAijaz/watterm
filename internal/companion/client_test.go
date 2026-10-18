package companion

import (
	"context"
	"testing"

	"github.com/AamerAijaz/watterm/internal/link"
	"github.com/AamerAijaz/watterm/internal/paths"
)

func TestFreshOpenIsUnpaired(t *testing.T) {
	lay, err := paths.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	running, err := Open(context.Background(), lay)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(running.Close)

	if running.Paired() {
		t.Fatal("a new Session is paired")
	}
	snap := running.Snapshot()
	if snap.Phase != link.PhasePairing {
		t.Fatalf("phase = %v, want pairing", snap.Phase)
	}
}
