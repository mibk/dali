package dali

import (
	"database/sql"

	"github.com/mibk/dali/dialects"
)

// DB wraps the sql.DB and provides slightly different
// API for communication with the database. The primary method
// is Query which provides methods for executing queries
// or scanning results.
type DB struct {
	DB         *sql.DB
	preproc    *Preprocessor
	middleware func(Execer) Execer
}

// NewDB instantiates DB from the given database/sql DB handle
// in the particular dialect.
func NewDB(db *sql.DB, dialect dialects.Dialect) *DB {
	return &DB{
		DB:         db,
		preproc:    NewPreprocessor(dialect),
		middleware: func(e Execer) Execer { return e },
	}
}

// Open opens a database by calling sql.Open. It returns a new DB and
// selects the appropriate dialect which is inferred from the driverName.
// It panics if the dialect is not supported by dali itself.
func Open(driverName, dataSourceName string) (*DB, error) {
	var dialect dialects.Dialect
	switch driverName {
	case "mysql":
		dialect = dialects.MySQL()
	default:
		panic("dali: unsupported dialect")
	}
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	return NewDB(db, dialect), nil
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

// Query is a fundamental method of DB. It returns a Query struct
// which is capable of executing the sql (given by the query and
// the args) or loading the result into structs or primitive values.
func (db *DB) Query(query string, args ...interface{}) *Query {
	sql, err := db.preproc.Process(query, args)
	return &Query{
		execer:  db.middleware(db.DB),
		preproc: db.preproc,
		query:   sql,
		err:     err,
	}
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
	sql, err := db.preproc.ProcessPreparedStmt(query, args)
	if err != nil {
		return nil, err
	}
	stmt, err := db.DB.Prepare(sql)
	if err != nil {
		return nil, err
	}
	return &Stmt{stmt, db, sql}, nil
}

// Begin starts a transaction. The isolation level is dependent on
// the driver.
func (db *DB) Begin() (*Tx, error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return nil, err
	}
	return &Tx{
		db: db,
		Tx: tx,
	}, nil
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
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
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
