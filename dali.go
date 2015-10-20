package dali

import (
	"database/sql"
	"fmt"

	"github.com/mibk/dali/drivers"
)

// Connection is a connection to the database.
type Connection struct {
	DB      *sql.DB
	preproc *Preprocessor
}

// NewConnection instantiates a Connection for the given database/sql connection.
func NewConnection(db *sql.DB, driver drivers.Driver) *Connection {
	return &Connection{db, NewPreprocessor(driver)}
}

// Open opens a database by calling sql.Open. It returns new Connection
// with nil EventReceiver.
func Open(driverName, dataSourceName string) (*Connection, error) {
	var driver drivers.Driver
	switch driverName {
	case "mysql":
		driver = drivers.MySQL{}
	default:
		return nil, fmt.Errorf("dali: unsupported driver")
	}
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	return NewConnection(db, driver), nil
}

// MustOpen is like Open but panics on error.
func MustOpen(driverName, dataSourceName string) *Connection {
	conn, err := Open(driverName, dataSourceName)
	if err != nil {
		panic(err)
	}
	return conn
}

// MustOpenAndVerify is like MustOpen but it verifies the connection and
// panics on error.
func MustOpenAndVerify(driverName, dataSourceName string) *Connection {
	conn := MustOpen(driverName, dataSourceName)
	if err := conn.Ping(); err != nil {
		panic(err)
	}
	return conn
}

// Close closes the database, releasing any open resources.
func (c *Connection) Close() error {
	return c.DB.Close()
}

// Ping verifies a connection to the database is still alive, establishing
// a connection if necessary.
func (c *Connection) Ping() error {
	return c.DB.Ping()
}

// Query creates Query by the raw SQL query and args.
func (c *Connection) Query(query string, args ...interface{}) *Query {
	return &Query{
		execer:  c.DB,
		preproc: c.preproc,
		query:   query,
		args:    args,
	}
}

// Begin starts a transaction. The isolation level is dependent on
// the driver.
func (c *Connection) Begin() (*Tx, error) {
	tx, err := c.DB.Begin()
	if err != nil {
		return nil, err
	}
	return &Tx{
		conn: c,
		Tx:   tx}, nil
}

// SetMapperFunc sets a mapper func which is used when deriving
// column names from field names. It none is set, the dali.ToUnderscore
// func is used.
func (c *Connection) SetMapperFunc(f func(string) string) {
	c.preproc.setMapperFunc(f)
}

type execer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}
