package dali

import (
	"database/sql"

	"github.com/mibk/dali/drivers"
)

type Query struct {
	execer execer
	driver drivers.Driver
	query  string
	args   []interface{}
}

// SQL returns the raw SQL query and the args.
func (q *Query) SQL() (query string, args []interface{}) {
	return q.query, q.args
}

// Exec executes a query that doesn't return rows.
// For example: an INSERT and UPDATE.
func (q *Query) Exec() (sql.Result, error) {
	sql, err := Preprocess(q.driver, q.query, nil)
	if err != nil {
		return nil, err
	}
	return q.execer.Exec(sql, q.args...)
}

// MustExec is like Exec but panics on error.
func (q *Query) MustExec() sql.Result {
	res, err := q.Exec()
	if err != nil {
		panic(err)
	}
	return res
}

// Query executes a query that returns rows, typically a SELECT.
// The args are for any placeholder parameters in the query.
func (q *Query) Rows() (*sql.Rows, error) {
	sql, err := Preprocess(q.driver, q.query, nil)
	if err != nil {
		return nil, err
	}
	return q.execer.Query(sql, q.args...)
}

// Row executes a query that is expected to return at most one row.
// Row always return a non-nil value. Errors are deferred until
// Row's Scan method is called.
func (q *Query) Row() *Row {
	sql, err := Preprocess(q.driver, q.query, nil)
	if err != nil {
		return &Row{err: err}
	}
	return &Row{Row: q.execer.QueryRow(sql, q.args...)}
}

// Row is a wrapper around sql.Row.
type Row struct {
	*sql.Row
	err error
}

// Scan copies the columns from the matched row into the values
// pointed at by dest.  If more than one row matches the query,
// Scan uses the first row and discards the rest.  If no row matches
// the query, Scan returns sql.ErrNoRows.
func (r *Row) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	return r.Row.Scan(dest...)
}
