package sql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"log"
	"reflect"
)

const createTable = `create table events (seq INTEGER PRIMARY KEY AUTOINCREMENT, id VARCHAR NOT NULL, version INTEGER, reason VARCHAR, type VARCHAR, timestamp VARCHAR, data BLOB, metadata BLOB);`
const createTableMySql = `create table events (seq INT UNIQUE PRIMARY KEY AUTO_INCREMENT, id VARCHAR(255) NOT NULL, version INTEGER, reason VARCHAR(255), type VARCHAR(255), timestamp VARCHAR(255), data BLOB, metadata BLOB);`
const createTablePSQL = `create table events (seq SERIAL PRIMARY KEY, id VARCHAR NOT NULL, version INTEGER, reason VARCHAR, "type" VARCHAR, timestamp VARCHAR, data bytea, metadata bytea);`

func getCreateTableStmt(driver driver.Driver) string {
	dv := reflect.ValueOf(driver)
	switch dv.Type().String() {
	case "*pq.Driver":
		log.Print("Use migrating statement for PostgresSQL")
		return createTablePSQL
	case "*mysql.MySQLDriver":
		log.Print("Use migrating statement for MySQL")
		return createTableMySql
	case "*sqlite3.SQLiteDriver":
		log.Print("Use migrating statement for SQLite")
		return createTable
	}
	log.Print("Driver could not be identified, use default statement (SQLite)")
	return createTable
}

// Migrate the database
func (s *SQL) Migrate() error {
	sqlStmt := []string{
		getCreateTableStmt(s.db.Driver()),
		`create unique index id_type_version on events (id, type, version);`,
		`create index id_type on events (id, type);`,
	}
	return s.migrate(sqlStmt)
}

func (s *SQL) migrate(stm []string) error {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer func(tx *sql.Tx) {
		err := tx.Rollback()
		if err != nil {
			log.Printf("could not rollback transaction %v", err)
		}
	}(tx)

	// check if the migration is already done
	rows, err := tx.Query(`Select count(*) from events`)
	if err == nil {
		err := rows.Close()
		if err != nil {
			return err
		}
		return nil
	}

	for _, b := range stm {
		_, err := tx.Exec(b)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
