package link

// Phase is how the Companion's link looks right now.
type Phase int

const (
	PhasePairing Phase = iota
	PhaseLinked
	PhaseReconnecting
)

// Snapshot is the link as the TUI should show it.
type Snapshot struct {
	Phase Phase
	QR    string
}

// QR is a pairing code to show.
type QR struct{ Code string }

// Connected means the Companion is linked.
type Connected struct{}

// Disconnected means the network dropped. The Session is still valid.
type Disconnected struct{}

// LoggedOut means the phone unlinked this Companion.
type LoggedOut struct{}

// Initial is the link at process start. paired is true when a Session is already on disk.
func Initial(paired bool) Snapshot {
	if paired {
		return Snapshot{Phase: PhaseLinked}
	}
	return Snapshot{Phase: PhasePairing}
}

// Apply folds one link event into the snapshot.
func (s Snapshot) Apply(ev any) Snapshot {
	switch e := ev.(type) {
	case QR:
		if s.Phase == PhaseLinked || s.Phase == PhaseReconnecting {
			return s
		}
		s.Phase = PhasePairing
		s.QR = e.Code
	case Connected:
		s.Phase = PhaseLinked
		s.QR = ""
	case Disconnected:
		if s.Phase == PhaseLinked || s.Phase == PhaseReconnecting {
			s.Phase = PhaseReconnecting
			s.QR = ""
		}
	case LoggedOut:
		s.Phase = PhasePairing
		s.QR = ""
	}
	return s
}
