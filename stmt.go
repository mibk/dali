package dali

import "database/sql"

// Stmt is a prepared statement.
type Stmt struct {
	stmt *sql.Stmt
	db   *DB
	sql  string
}

// Bind binds args to the prepared statement and returns a Query struct
// ready to be executed. See (*DB).Query method.
func (s *Stmt) Bind(args ...interface{}) *Query {
	return &Query{
		execer:  s.db.middleware(stmtExecer{s.stmt}),
		preproc: s.db.preproc,
		query:   s.sql,
		args:    args,
	}
}

// Close closes the statement.
func (s *Stmt) Close() error {
	return s.stmt.Close()
}

func (s *Stmt) String() string {
	return s.sql
}

// stmtExecer is just an adapter for Execer interface.
type stmtExecer struct {
	stmt *sql.Stmt
}

func (s stmtExecer) Exec(_ string, args ...interface{}) (sql.Result, error) {
	return s.stmt.Exec(args...)
}

func (s stmtExecer) Query(_ string, args ...interface{}) (*sql.Rows, error) {
	return s.stmt.Query(args...)
}

func (s stmtExecer) QueryRow(_ string, args ...interface{}) *sql.Row {
	return s.stmt.QueryRow(args...)
}
