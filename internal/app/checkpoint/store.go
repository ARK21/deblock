package checkpoint

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type State struct {
	LastFinalized uint64    `json:"last_finalized"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Store interface {
	Load(ctx context.Context) (State, error)
	Save(ctx context.Context, st State) error
}

type FileStore struct {
	path string
	mu   sync.Mutex
}

func NewFileStore(path string) *FileStore {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	return &FileStore{path: path}
}

func (s *FileStore) Load(ctx context.Context) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Open(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{}, nil
		}
		return State{}, err
	}
	defer file.Close()
	b, err := io.ReadAll(file)
	if err != nil || len(b) == 0 {
		return State{}, err
	}
	var st State
	if err := json.Unmarshal(b, &st); err != nil {
		return State{}, err
	}
	return st, nil
}

func (s *FileStore) Save(ctx context.Context, st State) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tmp := s.path + ".tmp"
	b, _ := json.MarshalIndent(st, "", "  ")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
