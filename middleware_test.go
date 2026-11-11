package dali

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
)

func TestMiddleware(t *testing.T) {
	t.Cleanup(func() {
		db.SetMiddlewareFunc(func(e Execer) Execer { return e })
	})
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

func TestAddMiddlewareFunc(t *testing.T) {
	t.Cleanup(func() {
		db.SetMiddlewareFunc(func(e Execer) Execer { return e })
	})
	var order []string
	add := func(name string) {
		db.AddMiddlewareFunc(func(e Execer) Execer {
			return &recorder{e, name, &order}
		})
	}
	add("first")
	add("second")

	db.Query("#1").Exec()
	if exp := []string{"second", "first"}; !reflect.DeepEqual(order, exp) {
		t.Errorf("got %v, want %v", order, exp)
	}
}

type recorder struct {
	Execer
	name  string
	order *[]string
}

func (r *recorder) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	*r.order = append(*r.order, r.name)
	return r.Execer.ExecContext(ctx, query, args...)
}

type middle struct {
	ex    Execer
	lastq string
}

func (p *middle) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	p.lastq = query + "-exec"
	return p.ex.ExecContext(ctx, query, args...)
}

func (p *middle) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	p.lastq = query + "-query"
	return p.ex.QueryContext(ctx, query, args...)
}

func (p *middle) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	p.lastq = query + "-queryrow"
	return p.ex.QueryRowContext(ctx, query, args...)
}

func (db *DB) mustPrepare(query string, args ...any) *Stmt {
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

func (tx *Tx) mustPrepare(query string, args ...any) *Stmt {
	s, err := tx.Prepare(query, args...)
	if err != nil {
		panic(err)
	}
	return s
}
