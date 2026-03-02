package dali

import (
	"context"
	"database/sql"
)

// Stmt is a prepared statement.
type Stmt struct {
	stmt       *sql.Stmt
	sql        string
	middleware func(Execer) Execer
}

// BindContext binds args to the prepared statement and returns a Query struct
// ready to be executed. See (*DB).Query method.
func (s *Stmt) BindContext(ctx context.Context, args ...any) *Query {
	return &Query{
		ctx:    ctx,
		execer: s.middleware(stmtExecer{s.stmt}),
		query:  s.sql,
		args:   args,
	}
}

// Bind binds args to the prepared statement and returns a Query struct
// ready to be executed. See (*DB).Query method.
func (s *Stmt) Bind(args ...any) *Query {
	return s.BindContext(context.Background(), args...)
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

func (s stmtExecer) ExecContext(ctx context.Context, _ string, args ...any) (sql.Result, error) {
	return s.stmt.ExecContext(ctx, args...)
}

func (s stmtExecer) QueryContext(ctx context.Context, _ string, args ...any) (*sql.Rows, error) {
	return s.stmt.QueryContext(ctx, args...)
}

func (s stmtExecer) QueryRowContext(ctx context.Context, _ string, args ...any) *sql.Row {
	return s.stmt.QueryRowContext(ctx, args...)
}
