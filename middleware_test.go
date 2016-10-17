package dali

import (
	"database/sql"
	"testing"
)

func TestMiddleware(t *testing.T) {
	m := &middle{}
	checkmw := func(exp string) {
		if m.lastq != exp {
			t.Errorf("got %s, want %s", m.lastq, exp)
		}
	}
	db.SetMiddlewareFunc(func(e Execer) Execer {
		m.ex = e
		return m
	})

	db.Query("#1").Exec()
	checkmw("#1-exec")
	db.Query("#1").Rows()
	checkmw("#1-query")
	db.Query("#1").ScanRow()
	checkmw("#1-queryrow")

	db.mustPrepare("#2").Bind().Exec()
	checkmw("#2-exec")
	db.mustPrepare("#2").Bind().Rows()
	checkmw("#2-query")
	db.mustPrepare("#2").Bind().ScanRow()
	checkmw("#2-queryrow")

	db.mustBegin().Query("#3").Exec()
	checkmw("#3-exec")
	db.mustBegin().Query("#3").Rows()
	checkmw("#3-query")
	db.mustBegin().Query("#3").ScanRow()
	checkmw("#3-queryrow")

	db.mustBegin().mustPrepare("#4").Bind().Exec()
	checkmw("#4-exec")
	db.mustBegin().mustPrepare("#4").Bind().Rows()
	checkmw("#4-query")
	db.mustBegin().mustPrepare("#4").Bind().ScanRow()
	checkmw("#4-queryrow")
}

type middle struct {
	ex    Execer
	lastq string
}

func (p *middle) Exec(query string, args ...interface{}) (sql.Result, error) {
	p.lastq = query + "-exec"
	return p.ex.Exec(query, args...)
}

func (p *middle) Query(query string, args ...interface{}) (*sql.Rows, error) {
	p.lastq = query + "-query"
	return p.ex.Query(query, args...)
}

func (p *middle) QueryRow(query string, args ...interface{}) *sql.Row {
	p.lastq = query + "-queryrow"
	return p.ex.QueryRow(query, args...)
}

func (db *DB) mustPrepare(query string, args ...interface{}) *Stmt {
	s, err := db.Prepare(query, args...)
	if err != nil {
		panic(err)
	}
	return s
}

func (db *DB) mustBegin() *Tx {
	tx, err := db.Begin()
	if err != nil {
		panic(err)
	}
	return tx
}

func (tx *Tx) mustPrepare(query string, args ...interface{}) *Stmt {
	s, err := tx.Prepare(query, args...)
	if err != nil {
		panic(err)
	}
	return s
}
