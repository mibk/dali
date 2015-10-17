package dali

import (
	"database/sql"
	"fmt"
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
		{cols("ID"), result(U{1, "John"}, U{3, "Peter"}, U{10, "Carmen"}),
			newTypeOf([]int64{}), (*Query).All,
			[]int64{1, 3, 10},
		},
		{cols("ID", "Name"), result(U{2, "Caroline"}, U{3, "Mark"}, U{4, "Lucas"}),
			newTypeOf([]U{}), (*Query).All,
			[]U{{2, "Caroline"}, {3, "Mark"}, {4, "Lucas"}},
		},
		{cols("ID", "Name"), result(U{1, "Alice"}, U{2, "Bob"}, U{13, "Carmen"}),
			newTypeOf([]*U{}), (*Query).All,
			[]*U{{1, "Alice"}, {2, "Bob"}, {13, "Carmen"}},
		},
		{cols("ID", "Name"), result(U{1, "Justin"}, U{2, "Martin"}, U{13, "Lis"}),
			newTypeOf([]V{}), (*Query).All,
			[]V{{0, "Justin"}, {0, "Martin"}, {0, "Lis"}},
		},

		{cols("ID", "Name"), result(U{1, "Thomas"}, U{2, "Bob"}, U{13, "Carmen"}),
			newTypeOf(U{}), (*Query).One,
			U{1, "Thomas"},
		},

		// embedded structs
		{cols("ID", "First", "Last"), result(Eres{1, "Thomas", "Shoe"}, Eres{4, "Bob", "Webber"}),
			newTypeOf([]E{}), (*Query).All,
			[]E{{1, Name{"Thomas", "Shoe"}}, {4, Name{"Bob", "Webber"}}},
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
			t.Errorf("loading:\n got: %v\nwant: %v", v, tt.expected)
		}
	}
}

type U struct {
	ID   int64
	Name string
}

func (u *U) String() string { return fmt.Sprintf("&%v", *u) }

type V struct {
	ID   int64 `db:"-"`
	Name string
}

type E struct {
	ID   int64
	Name Name
}

type Name struct {
	First string
	Last  string
}

type Eres struct {
	ID    int64
	First string
	Last  string
}

func newTypeOf(v interface{}) interface{}   { return reflect.New(reflect.TypeOf(v)).Interface() }
func cols(s ...string) []string             { return s }
func result(v ...interface{}) []interface{} { return v }
