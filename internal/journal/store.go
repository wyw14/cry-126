package journal

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Event struct {
	ID                  uuid.UUID       `json:"id"`
	Cursor              uint64          `json:"cursor"`
	OperationID         uuid.UUID       `json:"operation_id"`
	OperationGeneration uint64          `json:"operation_generation"`
	Type                string          `json:"type"`
	Entity              string          `json:"entity"`
	OccurredAt          time.Time       `json:"occurred_at"`
	Data                json.RawMessage `json:"data"`
}

type Store struct {
	mu   sync.Mutex
	path string
	next uint64
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("journal path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create journal directory: %w", err)
	}
	store := &Store{path: path, next: 1}
	events, err := ReadAll(path)
	if err != nil {
		return nil, err
	}
	for _, event := range events {
		if event.Cursor >= store.next {
			store.next = event.Cursor + 1
		}
	}
	return store, nil
}

func NewEvent(operationID uuid.UUID, generation uint64, eventType, entity string, payload any, now time.Time) (Event, error) {
	if operationID == uuid.Nil || generation == 0 {
		return Event{}, errors.New("event operation identity is required")
	}
	if eventType == "" || entity == "" {
		return Event{}, errors.New("event type and entity are required")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("encode event payload: %w", err)
	}
	return Event{
		ID:                  uuid.New(),
		OperationID:         operationID,
		OperationGeneration: generation,
		Type:                eventType,
		Entity:              entity,
		OccurredAt:          now.UTC(),
		Data:                data,
	}, nil
}

func (store *Store) Append(event Event) (Event, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	event.Cursor = store.next
	if err := validateEvent(event); err != nil {
		return Event{}, err
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return Event{}, fmt.Errorf("encode event: %w", err)
	}
	file, err := os.OpenFile(store.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return Event{}, fmt.Errorf("open journal: %w", err)
	}
	writeErr := error(nil)
	if _, err = file.Write(append(encoded, '\n')); err != nil {
		writeErr = fmt.Errorf("write journal: %w", err)
	} else if err = file.Sync(); err != nil {
		writeErr = fmt.Errorf("sync journal: %w", err)
	}
	closeErr := file.Close()
	if writeErr != nil {
		return Event{}, writeErr
	}
	if closeErr != nil {
		return Event{}, fmt.Errorf("close journal: %w", closeErr)
	}
	store.next++
	return event, nil
}

func (store *Store) NextCursor() uint64 {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.next
}

func (store *Store) Path() string {
	return store.path
}

func validateEvent(event Event) error {
	if event.ID == uuid.Nil || event.OperationID == uuid.Nil {
		return errors.New("event identities are required")
	}
	if event.Cursor == 0 || event.OperationGeneration == 0 {
		return errors.New("event cursor and generation are required")
	}
	if event.Type == "" || event.Entity == "" || event.OccurredAt.IsZero() {
		return errors.New("event classification is incomplete")
	}
	if !json.Valid(event.Data) {
		return errors.New("event payload is invalid JSON")
	}
	return nil
}

func ReadAll(path string) ([]Event, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []Event{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open journal for replay: %w", err)
	}
	defer file.Close()
	result := make([]Event, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, fmt.Errorf("decode journal event: %w", err)
		}
		if err := validateEvent(event); err != nil {
			return nil, fmt.Errorf("validate journal event: %w", err)
		}
		result = append(result, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan journal: %w", err)
	}
	return result, nil
}
