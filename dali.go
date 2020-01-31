package dali

import (
	"context"
	"database/sql"

	"github.com/mibk/dali/dialect"
)

// DB wraps the sql.DB and provides slightly different
// API for communication with the database. The primary method
// is Query which provides methods for executing queries
// or scanning results.
type DB struct {
	DB         *sql.DB
	dialect    dialect.Dialect
	middleware func(Execer) Execer
}

// NewDB instantiates DB from the given database/sql DB handle
// in the particular dialect.
func NewDB(db *sql.DB, d dialect.Dialect) *DB {
	return &DB{
		DB:         db,
		dialect:    d,
		middleware: func(e Execer) Execer { return e },
	}
}

// Open opens a database by calling sql.Open. It returns a new DB and
// selects the appropriate dialect which is inferred from the driverName.
// It panics if the dialect is not supported by dali itself.
func Open(driverName, dataSourceName string) (*DB, error) {
	var d dialect.Dialect
	switch driverName {
	case "mysql":
		d = dialect.MySQL
	default:
		panic("dali: unsupported dialect")
	}
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	return NewDB(db, d), nil
}

// Close closes the database, releasing any open resources.
func (db *DB) Close() error {
	return db.DB.Close()
}

// Ping verifies a connection to the database is still alive, establishing
// a connection if necessary.
func (db *DB) Ping() error {
	return db.DB.Ping()
}

// QueryWithContext is a fundamental method of DB. It returns a Query struct
// which is capable of executing the sql (given by the query and
// the args) or loading the result into structs or primitive values.
func (db *DB) QueryWithContext(ctx context.Context, query string, args ...interface{}) *Query {
	sql, err := translate(db.dialect, query, args)
	return &Query{
		ctx:    ctx,
		execer: db.middleware(db.DB),
		query:  sql,
		err:    err,
	}
}

// Query is a fundamental method of DB. It returns a Query struct
// which is capable of executing the sql (given by the query and
// the args) or loading the result into structs or primitive values.
func (db *DB) Query(query string, args ...interface{}) *Query {
	return db.QueryWithContext(context.Background(), query, args...)
}

// PrepareContext creates a prepared statement for later queries or executions.
// The caller must call the statement's Close method when the statement
// is no longer needed. Unlike the Prepare methods in database/sql this
// method also accepts args, which are meant only for query building.
// Therefore, only ?ident, ?ident..., ?sql are interpolated in this phase.
// Apart of that, ? is the only other placeholder allowed (this one
// will be transformed into a dialect specific one to allow the parameter
// binding.
//
// The provided context is used for the preparation of the statement, not
// for the execution of the statement.
func (db *DB) PrepareContext(ctx context.Context, query string, args ...interface{}) (*Stmt, error) {
	sql, err := translatePreparedStmt(db.dialect, query, args)
	if err != nil {
		return nil, err
	}
	stmt, err := db.DB.PrepareContext(ctx, sql)
	if err != nil {
		return nil, err
	}
	return &Stmt{stmt, sql, db.middleware}, nil
}

// Prepare creates a prepared statement for later queries or executions.
// The caller must call the statement's Close method when the statement
// is no longer needed. Unlike the Prepare methods in database/sql this
// method also accepts args, which are meant only for query building.
// Therefore, only ?ident, ?ident..., ?sql are interpolated in this phase.
// Apart of that, ? is the only other placeholder allowed (this one
// will be transformed into a dialect specific one to allow the parameter
// binding.
func (db *DB) Prepare(query string, args ...interface{}) (*Stmt, error) {
	return db.PrepareContext(context.Background(), query, args...)
}

// BeginTx starts a transaction.
//
// The provided context is used until the transaction is committed or rolled back.
// If the context is canceled, the sql package will roll back
// the transaction. Tx.Commit will return an error if the context provided to
// BeginTx is canceled.
//
// The provided TxOptions is optional and may be nil if defaults should be used.
// If a non-default isolation level is used that the driver doesn't support,
// an error will be returned.
func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	tx, err := db.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Tx{
		Tx:         tx,
		dialect:    db.dialect,
		middleware: db.middleware,
	}, nil
}

// Begin starts a transaction. The isolation level is dependent on
// the driver.
func (db *DB) Begin() (*Tx, error) {
	return db.BeginTx(context.Background(), nil)
}

// SetMiddlewareFunc changes the DB middleware func. Default func
// passes the Execer unchanged. SetMiddlewareFunc allowes the user
// to set his own middleware to perform additional operations (e.g.
// profiling) when executing queries.
func (db *DB) SetMiddlewareFunc(f func(Execer) Execer) {
	db.middleware = f
}

// Execer is an interface that Query works with.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// LastInsertID is a helper that wraps a call to a function returning
// (res sql.Result, err error). It returns err if it is not nil, otherwise
// it returns res.LastInsertId(). It is intended for uses such as
//	q := db.Query(...)
//	id, err := dali.LastInsertID(q.Exec())
func LastInsertID(res sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Map is just an alias for map[string]interface{}.
type Map map[string]interface{}
