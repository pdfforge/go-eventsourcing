package sqlite

import (
	"context"
	"database/sql"
	"sync"

	"github.com/hallgren/eventsourcing/core"
	essql "github.com/hallgren/eventsourcing/eventstore/sql"
)

type SQLite struct {
	sqlES *essql.SQL
	lock  sync.Mutex
}

func Open(db *sql.DB) *SQLite {
	return &SQLite{
		sqlES: essql.Open(db),
		lock:  sync.Mutex{},
	}
}

func (s *SQLite) Close() {
	s.sqlES.Close()
}

// Save persists events to the database
func (s *SQLite) Save(events []core.Event) error {
	// prevent multiple writers
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.sqlES.Save(events)
}

// Get the events from database
func (s *SQLite) Get(ctx context.Context, id string, aggregateType string, afterVersion core.Version) (core.Iterator, error) {
	return s.sqlES.Get(ctx, id, aggregateType, afterVersion)
}

// All iterate over all event in GlobalEvents order
func (s *SQLite) All(start core.Version, count uint64) (core.Iterator, error) {
	return s.sqlES.All(start, count)
}

// Migrate creates the database schema if not already present
func (s *SQLite) Migrate() error {
	return s.sqlES.Migrate()
}
