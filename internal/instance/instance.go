package instance

import (
	"errors"
	"os"

	"golang.org/x/sys/windows"
)

// ErrAlreadyRunning means another Companion process holds the lock.
var ErrAlreadyRunning = errors.New("one Companion is already running")

// Lock is the exclusive hold on one app-data directory.
type Lock struct {
	f *os.File
}

// Acquire takes the Companion lock. A second call while the first is held returns ErrAlreadyRunning.
func Acquire(path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	err = windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		1,
		0,
		&windows.Overlapped{},
	)
	if err != nil {
		f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, ErrAlreadyRunning
		}
		return nil, err
	}
	return &Lock{f: f}, nil
}

// Release drops the lock.
func (l *Lock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	err := windows.UnlockFileEx(windows.Handle(l.f.Fd()), 0, 1, 0, &windows.Overlapped{})
	closeErr := l.f.Close()
	l.f = nil
	if err != nil {
		return err
	}
	return closeErr
}
