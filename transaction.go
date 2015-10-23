package dali

import "database/sql"

// Tx wraps the sql.Tx to provide the Query method instead
// of the sql.Tx's original methods for comunication with
// the database.
type Tx struct {
	conn *Connection
	Tx   *sql.Tx
}

// Query creates Query by the raw SQL query and args.
func (tx *Tx) Query(query string, args ...interface{}) *Query {
	return &Query{
		execer:  tx.Tx,
		preproc: tx.conn.preproc,
		query:   query,
		args:    args,
	}
}

// Commit commits the transaction.
func (tx *Tx) Commit() error { return tx.Tx.Commit() }

// Rollback aborts the transaction.
func (tx *Tx) Rollback() error { return tx.Tx.Rollback() }
