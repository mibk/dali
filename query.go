package dali

import "database/sql"

type Query struct {
	execer  execer
	preproc *Preprocessor
	query   string
	args    []interface{}
}

// SQL returns the raw SQL query and the args.
func (q *Query) SQL() (query string, args []interface{}) {
	return q.query, q.args
}

// Exec executes a query that doesn't return rows.
// For example: an INSERT and UPDATE.
func (q *Query) Exec() (sql.Result, error) {
	sql, err := q.process()
	if err != nil {
		return nil, err
	}
	return q.execer.Exec(sql)
}

// MustExec is like Exec but panics on error.
func (q *Query) MustExec() sql.Result {
	res, err := q.Exec()
	if err != nil {
		panic(err)
	}
	return res
}

// Rows executes a query that returns rows, typically a SELECT.
func (q *Query) Rows() (*sql.Rows, error) {
	sql, err := q.process()
	if err != nil {
		return nil, err
	}
	return q.execer.Query(sql)
}

// ScanRow executes a query that is expected to return at most one row
// and copies the columns from the matched row into the values
// pointed at by dest. If more than one row matches the query,
// ScanRow uses the first row and discards the rest. If no row matches
// the query, ScanRow returns sql.ErrNoRows.
func (q *Query) ScanRow(dest ...interface{}) error {
	sql, err := q.process()
	if err != nil {
		return err
	}
	return q.execer.QueryRow(sql).Scan(dest...)
}

func (q *Query) String() string {
	sql, err := q.process()
	if err != nil {
		panic(err)
	}
	return sql
}

// process returns a preprocessed SQL query.
func (q *Query) process() (sql string, err error) {
	return q.preproc.Process(q.SQL())
}
