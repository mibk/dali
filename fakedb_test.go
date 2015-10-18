package dali

import (
	"database/sql/driver"
	"fmt"
	"io"
	"reflect"
)

type FakeDriver struct {
	cols   *[]string
	result *[]interface{}
}

func NewFakeDriver() *FakeDriver {
	cols := make([]string, 0)
	result := make([]interface{}, 0)
	return &FakeDriver{&cols, &result}
}

func (FakeDriver) EscapeIdent(w io.Writer, ident string) { fmt.Fprintf(w, "{%s}", ident) }
func (FakeDriver) EscapeBool(w io.Writer, v bool)        { fmt.Fprint(w, v) }
func (FakeDriver) EscapeString(w io.Writer, s string)    { fmt.Fprintf(w, "'%s'", s) }

func (d *FakeDriver) Open(name string) (driver.Conn, error) { return FakeConn{d.cols, d.result}, nil }

func (d *FakeDriver) SetColumns(cols ...string) *FakeDriver {
	*d.cols = cols
	return d
}

func (d *FakeDriver) SetResult(result ...interface{}) *FakeDriver {
	*d.result = result
	return d
}

type FakeConn struct {
	cols   *[]string
	result *[]interface{}
}

func (c FakeConn) Prepare(query string) (driver.Stmt, error) { return FakeStmt{c.cols, c.result}, nil }
func (FakeConn) Close() error                                { return nil }
func (FakeConn) Begin() (driver.Tx, error)                   { return FakeTx{}, nil }

type FakeStmt struct {
	cols   *[]string
	result *[]interface{}
}

func (FakeStmt) Close() error                                    { return nil }
func (FakeStmt) NumInput() int                                   { return -1 }
func (FakeStmt) Exec(args []driver.Value) (driver.Result, error) { return FakeResult{}, nil }
func (s FakeStmt) Query(args []driver.Value) (driver.Rows, error) {
	return &FakeRows{s.cols, s.result}, nil
}

type FakeResult struct{}

func (FakeResult) LastInsertId() (int64, error) { return 0, nil }
func (FakeResult) RowsAffected() (int64, error) { return 0, nil }

type FakeRows struct {
	cols   *[]string
	result *[]interface{}
}

func (FakeRows) Close() error { return nil }

// implement
func (r *FakeRows) Columns() []string { return *r.cols }
func (r *FakeRows) Next(dest []driver.Value) error {
	if len(*r.result) == 0 {
		return io.EOF
	}
	row := (*r.result)[0]
	*r.result = (*r.result)[1:]
	// if len(row
	rowv := reflect.ValueOf(row)
	if rowv.Kind() != reflect.Struct {
		panic("fake driver: result must be a slice of structs")
	}
	cols := r.Columns()
	if len(dest) != len(cols) {
		panic("fake driver: dest len and column count must match")
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
	return nil
}
func (FakeTx) Rollback() error {
	panic("not implemented")
	return nil
}
