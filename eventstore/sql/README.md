It's possible to open the sql event store in two modes.

## Open(db *sql.DB) *SQL 

Creates a new sql event store.

## OpenWithSingelWriter(db *sql.DB) *SQL

Creates a new sql event store that prevents multiple writers to save events concurrently.

