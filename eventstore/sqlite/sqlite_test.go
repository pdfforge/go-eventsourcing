package sqlite_test

import (
	sqldriver "database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/hallgren/eventsourcing/core"
	"github.com/hallgren/eventsourcing/core/testsuite"
	"github.com/hallgren/eventsourcing/eventstore/sqlite"
	_ "github.com/mattn/go-sqlite3"
)

func TestSuite(t *testing.T) {
	f := func() (core.EventStore, func(), error) {
		return eventstore()
	}
	testsuite.Test(t, f)
}

func eventstore() (*sqlite.SQLite, func(), error) {
	db, err := sqldriver.Open("sqlite3", "file::memory:?cache=shared")
	if err != nil {
		return nil, nil, errors.New(fmt.Sprintf("could not open database %v", err))
	}
	err = db.Ping()
	if err != nil {
		return nil, nil, errors.New(fmt.Sprintf("could not ping database %v", err))
	}

	es := sqlite.Open(db)
	err = es.Migrate()
	if err != nil {
		return nil, nil, errors.New(fmt.Sprintf("could not migrate database %v", err))
	}
	return es, func() {
		es.Close()
	}, nil
}
