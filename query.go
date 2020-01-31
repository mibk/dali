package dali

import (
	"context"
	"database/sql"
	"fmt"
)

// Query represents an arbitrary SQL statement.
// The SQL is preprocessed by Preprocessor before running.
type Query struct {
	ctx    context.Context
	execer Execer
	query  string
	args   []interface{}
	err    error
}

// Exec executes the query that shouldn't return rows.
// For example: INSERT or UPDATE.
func (q *Query) Exec() (sql.Result, error) {
	if q.err != nil {
		return nil, q.err
	}
	return q.execer.ExecContext(q.ctx, q.query, q.args...)
}

// Rows executes that query that should return rows, typically a SELECT.
func (q *Query) Rows() (*sql.Rows, error) {
	if q.err != nil {
		return nil, q.err
	}
	return q.execer.QueryContext(q.ctx, q.query, q.args...)
}

// ScanRow executes the query that is expected to return at most one row.
// It copies the columns from the matched row into the values
// pointed at by dest. If more than one row matches the query,
// ScanRow uses the first row and discards the rest. If no row matches
// the query, ScanRow returns sql.ErrNoRows.
func (q *Query) ScanRow(dest ...interface{}) error {
	if q.err != nil {
		return q.err
	}
	return q.execer.QueryRowContext(q.ctx, q.query, q.args...).Scan(dest...)
}

func (q *Query) String() string {
	if q.err != nil {
		panic(q.err)
	}
	if q.args != nil {
		return fmt.Sprintf("%s /* args: %v */", q.query, q.args)
	}
	return q.query
}
