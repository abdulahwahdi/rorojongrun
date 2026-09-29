package shared

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileStore keeps generated files (CSV exports). The local disk implementation needs a volume shared by
// every replica; an object storage implementation can replace it without touching the export module.
type FileStore interface {
	Create(name string) (io.WriteCloser, error)
	Open(name string) (io.ReadCloser, int64, error)
	Delete(name string) error
}

// NewLocalFileStore stores files under dir
func NewLocalFileStore(dir string) FileStore { return &localFileStore{dir: dir} }

type localFileStore struct{ dir string }

func (s *localFileStore) path(name string) string {
	return filepath.Join(s.dir, filepath.Base(strings.ReplaceAll(name, "..", "")))
}

func (s *localFileStore) Create(name string) (io.WriteCloser, error) {
	if err := os.MkdirAll(s.dir, 0o750); err != nil {
		return nil, err
	}
	return os.Create(s.path(name))
}

func (s *localFileStore) Open(name string) (io.ReadCloser, int64, error) {
	f, err := os.Open(s.path(name))
	if err != nil {
		return nil, 0, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, err
	}
	return f, st.Size(), nil
}

func (s *localFileStore) Delete(name string) error {
	if err := os.Remove(s.path(name)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
