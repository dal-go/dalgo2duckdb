// Package dalgo2duckdb adapts embedded DuckDB to DALgo using dalgo2sql,
// following the composition used by dalgo2sqlite. DuckDB requires cgo.
package dalgo2duckdb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo2sql"
	"github.com/duckdb/duckdb-go/v2"
)

// Database embeds the DALgo SQL adapter and owns its connection pool.
type Database struct {
	dal.DB
	sqlDB *sql.DB
}

// NewDatabase opens a DuckDB file; an empty path creates an in-memory database.
func NewDatabase(path string) (*Database, error) {
	return NewDatabaseWithOptions(path, dal.NewSchema(nil, nil), dalgo2sql.DbOptions{})
}

// NewDatabaseWithOptions supplies DALgo schema and SQL adapter options.
func NewDatabaseWithOptions(path string, schema dal.Schema, opts dalgo2sql.DbOptions) (*Database, error) {
	c, err := duckdb.NewConnector(path, nil)
	if err != nil {
		return nil, fmt.Errorf("dalgo2duckdb: open: %w", err)
	}
	db := sql.OpenDB(connector{c})
	if err = db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("dalgo2duckdb: connect: %w", err)
	}
	return &Database{DB: dalgo2sql.NewDatabase(db, schema, opts), sqlDB: db}, nil
}

// SQLDB returns the underlying pool, for DuckDB-native operations such as COPY.
func (d *Database) SQLDB() *sql.DB { return d.sqlDB }

// Close releases the connection pool.
func (d *Database) Close() error { return d.sqlDB.Close() }

// Adapter identifies this adapter.
func (d *Database) Adapter() dal.Adapter { return dal.NewAdapter("dalgo2duckdb", "0.1.0") }

var _ dal.DB = (*Database)(nil)

// NewReadOnlyDatabase starts a read-only transaction on every physical
// connection, including replacements after cancellation. Files must use a DSN
// containing access_mode=read_only; an empty DSN creates an in-memory session.
// Read-only transactions prevent database writes, but SELECTs can still read
// external files and URLs. This constructor does not restrict statements sent
// through the embedded dal.DB or SQLDB. Call ValidateReadQuery before executing
// untrusted SQL to require exactly one SELECT; resource access requires a
// separate operator-level authorization decision.
func NewReadOnlyDatabase(dsn string) (*Database, error) {
	connector, err := duckdb.NewConnector(dsn, func(execer driver.ExecerContext) error {
		_, err := execer.ExecContext(context.Background(), "BEGIN TRANSACTION READ ONLY", nil)
		return err
	})
	if err != nil {
		return nil, err
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	if err = db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Database{DB: dalgo2sql.NewDatabase(db, dal.NewSchema(nil, nil), dalgo2sql.DbOptions{}), sqlDB: db}, nil
}

// ValidateReadQuery asks DuckDB to parse exactly one SELECT without executing
// preceding statements. Deliberately use the driver's legacy Prepare method:
// PrepareContext executes all but the last statement of a multi-statement input.
func (d *Database) ValidateReadQuery(ctx context.Context, text string) error {
	conn, err := d.sqlDB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Raw(func(raw any) error {
		var duck *duckdb.Conn
		switch c := raw.(type) {
		case *duckdb.Conn:
			duck = c
		case *connection:
			duck = c.Conn
		default:
			return fmt.Errorf("dalgo2duckdb: unexpected connection type")
		}
		stmt, err := duck.Prepare(text)
		if err != nil {
			return fmt.Errorf("dalgo2duckdb: expected one SELECT query: %w", err)
		}
		defer stmt.Close()
		kind, err := stmt.(*duckdb.Stmt).StatementType()
		if err != nil {
			return err
		}
		if kind != duckdb.STATEMENT_TYPE_SELECT {
			return fmt.Errorf("dalgo2duckdb: expected one SELECT query")
		}
		return nil
	})
}
