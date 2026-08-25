package journal

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/wyw14/cry-126/internal/model"
)

type SnapshotStore struct {
	mu   sync.Mutex
	path string
}

func NewSnapshotStore(path string) (*SnapshotStore, error) {
	if path == "" {
		return nil, errors.New("snapshot path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create snapshot directory: %w", err)
	}
	return &SnapshotStore{path: path}, nil
}

func (store *SnapshotStore) Save(snapshot model.SystemSnapshot) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	model.SortSnapshot(&snapshot)
	encoded, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}
	temporary := store.path + ".tmp"
	if err := os.WriteFile(temporary, append(encoded, '\n'), 0o644); err != nil {
		return fmt.Errorf("write snapshot temporary file: %w", err)
	}
	file, err := os.OpenFile(temporary, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open snapshot temporary file: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync snapshot temporary file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close snapshot temporary file: %w", err)
	}
	if err := os.Rename(temporary, store.path); err != nil {
		return fmt.Errorf("replace snapshot: %w", err)
	}
	return nil
}

func (store *SnapshotStore) Load() (model.SystemSnapshot, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	encoded, err := os.ReadFile(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return model.SystemSnapshot{}, nil
	}
	if err != nil {
		return model.SystemSnapshot{}, fmt.Errorf("read snapshot: %w", err)
	}
	var snapshot model.SystemSnapshot
	if err := json.Unmarshal(encoded, &snapshot); err != nil {
		return model.SystemSnapshot{}, fmt.Errorf("decode snapshot: %w", err)
	}
	model.SortSnapshot(&snapshot)
	return snapshot, nil
}
