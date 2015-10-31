package dali

import (
	"database/sql/driver"
	"fmt"
	"io"
	"reflect"
	"time"
)

type FakeDialect struct {
	cols   []string
	result []interface{}
	cur    int
}

func NewFakeDialect() *FakeDialect {
	return &FakeDialect{}
}

func (FakeDialect) EscapeIdent(w io.Writer, ident string)   { fmt.Fprintf(w, "{%s}", ident) }
func (FakeDialect) EscapeBool(w io.Writer, v bool)          { fmt.Fprint(w, v) }
func (FakeDialect) EscapeString(w io.Writer, s string)      { fmt.Fprintf(w, "'%s'", s) }
func (FakeDialect) EscapeBytes(w io.Writer, b []byte)       { fmt.Fprintf(w, "`%s`", string(b)) }
func (FakeDialect) EscapeTime(w io.Writer, t time.Time)     { fmt.Fprintf(w, "'%v'", t) }
func (FakeDialect) PrintPlaceholderSign(w io.Writer, n int) { fmt.Fprintf(w, "&%d", n) }

func (d *FakeDialect) Open(name string) (driver.Conn, error) { return FakeConn{d}, nil }

func (d *FakeDialect) SetColumns(cols ...string) *FakeDialect {
	d.cols = cols
	return d
}

func (d *FakeDialect) SetResult(result ...interface{}) *FakeDialect {
	d.result = result
	return d
}

type FakeConn struct {
	d *FakeDialect
}

func (c FakeConn) Prepare(query string) (driver.Stmt, error) { return FakeStmt{c.d}, nil }
func (FakeConn) Close() error                                { return nil }
func (FakeConn) Begin() (driver.Tx, error)                   { return FakeTx{}, nil }

type FakeStmt struct {
	d *FakeDialect
}

func (FakeStmt) Close() error                                    { return nil }
func (FakeStmt) NumInput() int                                   { return -1 }
func (FakeStmt) Exec(args []driver.Value) (driver.Result, error) { return FakeResult{}, nil }
func (s FakeStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.d.cur = 0
	return &FakeRows{s.d}, nil
}

type FakeResult struct{}

func (FakeResult) LastInsertId() (int64, error) { return 0, nil }
func (FakeResult) RowsAffected() (int64, error) { return 0, nil }

type FakeRows struct {
	d *FakeDialect
}

func (FakeRows) Close() error { return nil }

// implement
func (r *FakeRows) Columns() []string { return r.d.cols }
func (r *FakeRows) Next(dest []driver.Value) error {
	if r.d.cur >= len(r.d.result) {
		return io.EOF
	}
	row := r.d.result[r.d.cur]
	r.d.cur++
	rowv := reflect.ValueOf(row)
	if rowv.Kind() != reflect.Struct {
		panic("fake db: result must be a slice of structs")
	}
	cols := r.Columns()
	if len(dest) != len(cols) {
		panic("fake db: dest len and column count must match")
	}
	for i, col := range cols {
		v := rowv.FieldByName(col)
		if !v.IsValid() {
			panic(fmt.Sprintf("field %s is not contained in %s struct",
				col, rowv.Type().Name()))
		}
		dest[i] = v.Interface()
	}
	return nil
}

type FakeTx struct{}

func (FakeTx) Commit() error {
	panic("not implemented")
}
func (FakeTx) Rollback() error {
	panic("not implemented")
}
