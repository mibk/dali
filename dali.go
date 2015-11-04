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
		preproc:    NewPreprocessor(dialect, ToUnderscore),
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
		dialect = dialects.MySQL{}
	default:
		panic("dali: unsupported dialect")
	}
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	return NewDB(db, dialect), nil
}

// MustOpen is like Open but panics on error.
func MustOpen(driverName, dataSourceName string) *DB {
	db, err := Open(driverName, dataSourceName)
	if err != nil {
		panic(err)
	}
	return db
}

// MustOpenAndVerify is like MustOpen but it verifies the connection and
// panics on error.
func MustOpenAndVerify(driverName, dataSourceName string) *DB {
	db := MustOpen(driverName, dataSourceName)
	if err := db.Ping(); err != nil {
		panic(err)
	}
	return db
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
// The caller must call the statement's Close method
// when the statement is no longer needed.
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

// MustPrepare is like Prepare but panics on error.
func (db *DB) MustPrepare(query string, args ...interface{}) *Stmt {
	s, err := db.Prepare(query, args...)
	if err != nil {
		panic(err)
	}
	return s
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

// MustBegin is like Begin but panics on error.
func (db *DB) MustBegin() *Tx {
	tx, err := db.Begin()
	if err != nil {
		panic(err)
	}
	return tx
}

// SetMapperFunc sets a mapper func which is used when deriving
// column names from field names. If none is set, ToUnderscore
// func is used.
func (db *DB) SetMapperFunc(f func(string) string) {
	db.preproc.setMapperFunc(f)
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
