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
// The caller must call the statement's Close method
// when the statement is no longer needed.
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

// MustPrepare is like Prepare but panics on error.
func (tx *Tx) MustPrepare(query string, args ...interface{}) *Stmt {
	s, err := tx.Prepare(query, args...)
	if err != nil {
		panic(err)
	}
	return s
}

// Commit commits the transaction.
func (tx *Tx) Commit() error { return tx.Tx.Commit() }

// Rollback aborts the transaction.
func (tx *Tx) Rollback() error { return tx.Tx.Rollback() }
