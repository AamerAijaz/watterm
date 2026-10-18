package paths

import (
	"os"
	"path/filepath"
)

// Layout is the on-disk siblings for one Companion.
type Layout struct {
	Root       string
	Session    string
	Transcript string
	Media      string
	Lock       string
}

// Open creates the app-data directory and returns its layout.
func Open(root string) (Layout, error) {
	media := filepath.Join(root, "media")
	if err := os.MkdirAll(media, 0o700); err != nil {
		return Layout{}, err
	}
	return Layout{
		Root:       root,
		Session:    filepath.Join(root, "session.db"),
		Transcript: filepath.Join(root, "transcript.db"),
		Media:      media,
		Lock:       filepath.Join(root, "companion.lock"),
	}, nil
}

// DefaultRoot is %AppData%\Watterm, or ~/.watterm when APPDATA is unset.
func DefaultRoot() string {
	if app := os.Getenv("APPDATA"); app != "" {
		return filepath.Join(app, "Watterm")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "Watterm"
	}
	return filepath.Join(home, ".watterm")
}

// RemoveSession deletes the Session files and leaves the Transcript and media in place.
func RemoveSession(lay Layout) error {
	for _, path := range []string{lay.Session, lay.Session + "-wal", lay.Session + "-shm"} {
		err := os.Remove(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
