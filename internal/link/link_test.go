package link

import "testing"

func TestDisconnectedWhileLinkedReconnectsWithoutQR(t *testing.T) {
	snap := Initial(true).Apply(Disconnected{})

	if snap.Phase != PhaseReconnecting {
		t.Fatalf("phase = %v, want reconnecting", snap.Phase)
	}
	if snap.QR != "" {
		t.Fatalf("qr = %q, want empty", snap.QR)
	}
}

func TestLoggedOutReturnsToPairing(t *testing.T) {
	snap := Initial(true).Apply(LoggedOut{})

	if snap.Phase != PhasePairing {
		t.Fatalf("phase = %v, want pairing", snap.Phase)
	}
}

func TestFreshCompanionStartsUnpaired(t *testing.T) {
	snap := Initial(false)

	if snap.Phase != PhasePairing {
		t.Fatalf("phase = %v, want pairing", snap.Phase)
	}
}

func TestQRDoesNotReplaceALiveLink(t *testing.T) {
	snap := Initial(true).Apply(QR{Code: "should-not-stick"})

	if snap.Phase != PhaseLinked {
		t.Fatalf("phase = %v, want linked", snap.Phase)
	}
	if snap.QR != "" {
		t.Fatalf("qr = %q, want empty", snap.QR)
	}
}

func TestConnectedClearsQR(t *testing.T) {
	snap := Initial(false).Apply(QR{Code: "abc"}).Apply(Connected{})

	if snap.Phase != PhaseLinked {
		t.Fatalf("phase = %v, want linked", snap.Phase)
	}
	if snap.QR != "" {
		t.Fatalf("qr = %q, want empty", snap.QR)
	}
}
