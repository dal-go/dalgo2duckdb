# dalgo2duckdb

Embedded DuckDB adapter for DALgo, following dalgo2sqlite's composition of
`dalgo2sql` and `database/sql`.

```go
db, err := dalgo2duckdb.NewDatabase("analytics.duckdb") // "" for in-memory
if err != nil { return err }
defer db.Close()
reader, err := db.ExecuteQueryToRecordsReader(ctx,
    dal.NewTextQuery("SELECT * FROM read_parquet('data.parquet')", nil))
```

Uses the official `github.com/duckdb/duckdb-go/v2` driver and requires
`CGO_ENABLED=1` and a C/C++ toolchain. DuckDB's Parquet reader is built in;
no preview flag or extension installation is required.

`NewDatabaseWithOptions` accepts DALgo schema and SQL adapter options.
`SQLDB()` exposes the underlying pool for native operations such as `COPY`.
This adapter currently provides DALgo's SQL query/transaction surface;
DuckDB-specific schema introspection and DDL interfaces are not implemented.
