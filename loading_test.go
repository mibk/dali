package dali

import (
	"database/sql"
	"reflect"
	"testing"
)

var (
	dvr  *FakeDriver
	conn *Connection
)

func init() {
	dvr = NewFakeDriver()
	sql.Register("dali", dvr)
	db, err := sql.Open("dali", "")
	if err != nil {
		panic(err)
	}
	conn = NewConnection(db, dvr)
	conn.SetMapperFunc(func(s string) string { return s })
}

func TestScanRow(t *testing.T) {
	var (
		id   int64
		name string
	)
	dvr.SetColumns("ID").SetResult(U{1, "John"})
	conn.Query("").ScanRow(&id)
	if id != 1 {
		t.Errorf("id: got %v, want %v", id, 1)
	}
	dvr.SetColumns("Name").SetResult(U{1, "John"})
	conn.Query("").ScanRow(&name)
	if name != "John" {
		t.Errorf("name: got %v, want %v", name, "John")
	}
}

func Test_One_and_All(t *testing.T) {
	tests := []struct {
		cols     []string
		result   []interface{}
		v        interface{} // value to load
		method   func(q *Query, dest interface{}) error
		expected interface{}
	}{
		{cols("ID"), result(U{1, "John"}, U{2, "Peter"}, U{13, "Carmen"}),
			newTypeOf([]int64{}), (*Query).All,
			[]int64{1, 2, 13},
		},
		{cols("ID", "Name"), result(U{1, "Alice"}, U{2, "Bob"}, U{13, "Carmen"}),
			newTypeOf([]U{}), (*Query).All,
			[]U{{1, "Alice"}, {2, "Bob"}, {13, "Carmen"}},
		},
		{cols("ID", "Name"), result(U{1, "Alice"}, U{2, "Bob"}, U{13, "Carmen"}),
			newTypeOf([]V{}), (*Query).All,
			[]V{{0, "Alice"}, {0, "Bob"}, {0, "Carmen"}},
		},

		{cols("ID", "Name"), result(U{1, "Alice"}, U{2, "Bob"}, U{13, "Carmen"}),
			newTypeOf(U{}), (*Query).One,
			U{1, "Alice"},
		},
	}

	for _, tt := range tests {
		dvr.SetColumns(tt.cols...).SetResult(tt.result...)
		if err := tt.method(conn.Query(""), tt.v); err != nil {
			panic(err)
		}
		vv := reflect.ValueOf(tt.v)
		v := reflect.Indirect(vv).Interface()
		if !reflect.DeepEqual(v, tt.expected) {
			t.Errorf("loading:\n got: %v,\nwant: %v", v, tt.expected)
		}
	}
}

type U struct {
	ID   int64
	Name string
}

type V struct {
	ID   int64 `db:"-"`
	Name string
}

func newTypeOf(v interface{}) interface{}   { return reflect.New(reflect.TypeOf(v)).Interface() }
func cols(s ...string) []string             { return s }
func result(v ...interface{}) []interface{} { return v }
