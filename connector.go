package dalgo2duckdb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"github.com/duckdb/duckdb-go/v2"
)

// DuckDB supports READ ONLY in SQL, although its Go driver rejects the
// database/sql transaction option. Preserve the driver's other interfaces by
// embedding its concrete connection and translate only that option.
type connector struct{ *duckdb.Connector }

func (c connector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &connection{Conn: conn.(*duckdb.Conn)}, nil
}

type connection struct{ *duckdb.Conn }

func (c *connection) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if !opts.ReadOnly {
		return c.Conn.BeginTx(ctx, opts)
	}
	if sql.IsolationLevel(opts.Isolation) != sql.LevelDefault {
		return nil, fmt.Errorf("dalgo2duckdb: unsupported transaction isolation")
	}
	if _, err := c.Conn.ExecContext(ctx, "BEGIN TRANSACTION READ ONLY", nil); err != nil {
		return nil, err
	}
	return readOnlyTransaction{conn: c.Conn}, nil
}

type readOnlyTransaction struct{ conn *duckdb.Conn }

func (t readOnlyTransaction) Commit() error {
	_, err := t.conn.ExecContext(context.Background(), "COMMIT", nil)
	return err
}
func (t readOnlyTransaction) Rollback() error {
	_, err := t.conn.ExecContext(context.Background(), "ROLLBACK", nil)
	return err
}
