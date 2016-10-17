package dali

import "database/sql"

// Tx wraps the sql.Tx to provide the Query method instead
// of the sql.Tx's original methods for comunication with
// the database.
type Tx struct {
	db *DB
	Tx *sql.Tx
}

// Query is a (*DB).Query equivalent for transactions.
func (tx *Tx) Query(query string, args ...interface{}) *Query {
	sql, err := tx.db.preproc.Process(query, args)
	return &Query{
		execer:  tx.db.middleware(tx.Tx),
		preproc: tx.db.preproc,
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
func (tx *Tx) Prepare(query string, args ...interface{}) (*Stmt, error) {
	sql, err := tx.db.preproc.ProcessPreparedStmt(query, args)
	if err != nil {
		return nil, err
	}
	stmt, err := tx.Tx.Prepare(sql)
	if err != nil {
		return nil, err
	}
	return &Stmt{stmt, tx.db, sql}, nil
}

// Commit commits the transaction.
func (tx *Tx) Commit() error { return tx.Tx.Commit() }

// Rollback aborts the transaction.
func (tx *Tx) Rollback() error { return tx.Tx.Rollback() }
