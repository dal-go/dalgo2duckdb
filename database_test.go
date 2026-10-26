package dalgo2duckdb

import (
	"context"
	"github.com/dal-go/dalgo/dal"
	"path/filepath"
	"testing"
)

func TestDatabaseTextQuery(t *testing.T) {
	db, err := NewDatabase(filepath.Join(t.TempDir(), "test.duckdb"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.SQLDB().ExecContext(context.Background(), "CREATE TABLE items(id INTEGER, name VARCHAR); INSERT INTO items VALUES (1, 'DuckDB')"); err != nil {
		t.Fatal(err)
	}
	reader, err := db.ExecuteQueryToRecordsReader(context.Background(), dal.NewTextQuery("SELECT name FROM items WHERE id = ?", nil, dal.QueryArg{Value: 1}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = reader.Next(); err != nil {
		t.Fatal(err)
	}
	if err = reader.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestReadOnlySingleSelect(t *testing.T) {
	db, err := NewReadOnlyDatabase("")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, text := range []string{"SELECT 1; SELECT 2", "CREATE TABLE forbidden(id INTEGER); SELECT 1", "COMMIT; CREATE TABLE forbidden(id INTEGER)", "COPY (SELECT 1) TO '/tmp/dalgo-duckdb-forbidden.parquet' (FORMAT PARQUET)", "SET enable_external_access=true", "SELECT 1) AS datatug_query; CREATE TABLE forbidden(id INTEGER); SELECT (1"} {
		if err = db.ValidateReadQuery(context.Background(), text); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
	if err = db.ValidateReadQuery(context.Background(), "WITH t AS (SELECT ';' AS value) SELECT * FROM t"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.SQLDB().QueryRow("SELECT count(*) FROM information_schema.tables WHERE table_name='forbidden'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("side effect count=%d err=%v", count, err)
	}
	if _, err = db.SQLDB().Exec("CREATE TABLE forbidden(id INTEGER)"); err == nil {
		t.Fatal("read-only connection accepted write")
	}
}
