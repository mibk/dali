package dali

import (
	"database/sql"

	"github.com/mibk/dali/dialect"
)

// Tx wraps the sql.Tx to provide the Query method instead
// of the sql.Tx's original methods for comunication with
// the database.
type Tx struct {
	Tx         *sql.Tx
	dialect    dialect.Dialect
	middleware func(Execer) Execer
}

// Query is a (*DB).Query equivalent for transactions.
func (tx *Tx) Query(query string, args ...interface{}) *Query {
	sql, err := translate(tx.dialect, query, args)
	return &Query{
		execer: tx.middleware(tx.Tx),
		query:  sql,
		err:    err,
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
func (tx *Tx) Prepare(query string, args ...interface{}) (*Stmt, error) {
	sql, err := translatePreparedStmt(tx.dialect, query, args)
	if err != nil {
		return nil, err
	}
	stmt, err := tx.Tx.Prepare(sql)
	if err != nil {
		return nil, err
	}
	return &Stmt{stmt, sql, tx.middleware}, nil
}

// Stmt returns a transaction-specific prepared statement from an existing
// statement. The returned statement operates within the transaction and can
// no longer be used once the transaction has been committed or rolled back.
func (tx *Tx) Stmt(stmt *Stmt) *Stmt {
	stmt.stmt = tx.Tx.Stmt(stmt.stmt)
	return stmt
}

// Commit commits the transaction.
func (tx *Tx) Commit() error { return tx.Tx.Commit() }

// Rollback aborts the transaction.
func (tx *Tx) Rollback() error { return tx.Tx.Rollback() }
