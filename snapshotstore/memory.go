package snapshotstore

import (
	"errors"
)

// Handler of snapshot store
type Handler struct {
	store      map[string][]byte
	serializer snapshotSerializer
}

// Snapshot interface
type Snapshot interface {
	ID() string
	UnsavedEvents() bool
}

type snapshotSerializer interface {
	SerializeSnapshot(s Snapshot) ([]byte, error)
	DeserializeSnapshot(data []byte, s Snapshot) error
}

// ErrSnapshotNotFound returns if snapshot not found
var ErrSnapshotNotFound = errors.New("snapshot not found")

// New handler for the snapshot service
func New(serializer snapshotSerializer) *Handler {
	return &Handler{
		store:      make(map[string][]byte),
		serializer: serializer,
	}
}

// Get returns the deserialize snapshot
func (h *Handler) Get(id string, s Snapshot) error {
	v, ok := h.store[id]
	if !ok {
		return ErrSnapshotNotFound
	}
	err := h.serializer.DeserializeSnapshot(v, s)
	if err != nil {
		return err
	}

	return nil
}

// Save persists the snapshot
func (h *Handler) Save(s Snapshot) error {
	if s.ID() == "" {
		return errors.New("aggregate id is empty")
	}
	if s.UnsavedEvents() {
		return errors.New("aggregate holds unsaved events")
	}
	data, err := h.serializer.SerializeSnapshot(s)
	if err != nil {
		return err
	}
	h.store[s.ID()] = data
	return nil
}
