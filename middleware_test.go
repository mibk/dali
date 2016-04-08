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

	db.MustPrepare("#2").Bind().Exec()
	checkmw("#2-exec")
	db.MustPrepare("#2").Bind().Rows()
	checkmw("#2-query")
	db.MustPrepare("#2").Bind().ScanRow()
	checkmw("#2-queryrow")

	db.MustBegin().Query("#3").Exec()
	checkmw("#3-exec")
	db.MustBegin().Query("#3").Rows()
	checkmw("#3-query")
	db.MustBegin().Query("#3").ScanRow()
	checkmw("#3-queryrow")

	db.MustBegin().MustPrepare("#4").Bind().Exec()
	checkmw("#4-exec")
	db.MustBegin().MustPrepare("#4").Bind().Rows()
	checkmw("#4-query")
	db.MustBegin().MustPrepare("#4").Bind().ScanRow()
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