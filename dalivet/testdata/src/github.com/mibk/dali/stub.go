// Stub package for analysistest. Contains only the types and method
// signatures needed by dalivet's test cases.
package dali

import "context"

type DB struct{}

func (db *DB) Query(query string, args ...any) *Query                            { return nil }
func (db *DB) QueryWithContext(ctx context.Context, query string, args ...any) *Query { return nil }
func (db *DB) Prepare(query string, args ...any) (*Stmt, error)                  { return nil, nil }
func (db *DB) PrepareContext(ctx context.Context, query string, args ...any) (*Stmt, error) {
	return nil, nil
}

type Tx struct{}

func (tx *Tx) Query(query string, args ...any) *Query                            { return nil }
func (tx *Tx) QueryWithContext(ctx context.Context, query string, args ...any) *Query { return nil }
func (tx *Tx) Prepare(query string, args ...any) (*Stmt, error)                  { return nil, nil }
func (tx *Tx) PrepareContext(ctx context.Context, query string, args ...any) (*Stmt, error) {
	return nil, nil
}

type Query struct{}

func (q *Query) One(dest any) error            { return nil }
func (q *Query) All(dest any) error            { return nil }
func (q *Query) ScanAllRows(dests ...any) error { return nil }

type Stmt struct{}

func (s *Stmt) Bind(args ...any) *Query                          { return nil }
func (s *Stmt) BindContext(ctx context.Context, args ...any) *Query { return nil }

// Map is an alias for map[string]any.
type Map map[string]any

// Translator processes SQL queries.
type Translator struct{}

// Marshaler is the interface for types that marshal to SQL.
type Marshaler interface {
	MarshalSQL(t Translator) (string, error)
}
