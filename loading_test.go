package dali

import (
	"database/sql"
	"fmt"
	"reflect"
	"testing"
	"time"
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

		// column names from field tags
		{cols("ID", "V_name"), result(Vres{1, "Justin"}, Vres{2, "Martin"}, Vres{13, "Lis"}),
			newTypeOf([]V{}), (*Query).All,
			[]V{{0, "Justin"}, {0, "Martin"}, {0, "Lis"}},
		},

		{cols("ID", "Name"), result(U{1, "Thomas"}, U{2, "Bob"}, U{13, "Carmen"}),
			newTypeOf(U{}), (*Query).One,
			U{1, "Thomas"},
		},

		// nested structs
		{cols("ID", "First", "Last"), result(Eres{1, "Thomas", "Shoe"}, Eres{4, "Bob", "Webber"}),
			newTypeOf([]E{}), (*Query).All,
			[]E{{1, Name{"Thomas", "Shoe"}}, {4, Name{"Bob", "Webber"}}},
		},

		// ignored nested structs
		{cols("Event", "Started", "Finished"), result(SpecialStructRes{"Lunch",
			parseTime("2015-05-05 12:24:32"), nil}),
			newTypeOf(SpecialStruct{}), (*Query).One,
			SpecialStruct{"Lunch", parseTime("2015-05-05 12:24:32"), NullTime{}}},
		{cols("Event", "Started", "Finished"), result(SpecialStructRes{"Lunch",
			parseTime("2015-05-05 12:24:32"), parseTime("2015-05-05 13:08:17")}),
			newTypeOf(SpecialStruct{}), (*Query).One,
			SpecialStruct{"Lunch", parseTime("2015-05-05 12:24:32"),
				NullTime{parseTime("2015-05-05 13:08:17"), true}}},

		// ignore scanner but not valuer
		{cols("A", "B", "Scan"), result(VSres{2, 3, "group:name"}),
			newTypeOf(VS{}), (*Query).One, VS{Val{2, 3}, Scan{"group", "name"}}},

		// ,omitinsert
		{cols("ID", "Name", "Age"), result(Omit{1, "Barbora", 19}, Omit{4, "Bob", 23}),
			newTypeOf([]Omit{}), (*Query).All,
			[]Omit{{1, "Barbora", 19}, {4, "Bob", 23}},
		},
		{cols("Id_user", "Name", "Age"), result(Omit2res{1, "Hubert", 32}, Omit2res{4, "Bob", 23}),
			newTypeOf([]Omit2{}), (*Query).All,
			[]Omit2{{1, "Hubert", 32}, {4, "Bob", 23}},
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
	ID   int64  `db:"-"`
	Name string `db:"V_name"`
}

type Vres struct {
	ID     int64
	V_name string
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

type VSres struct {
	A    int
	B    int
	Scan string
}

func TestLoading_Types(t *testing.T) {
	tests := []struct {
		result   []interface{}
		v        interface{} // value to load
		expected interface{}
	}{
		{result(struct{ A int64 }{-27}), newTypeOf(int64(0)), int64(-27)},
		{result(struct{ A float64 }{-2.71828}), newTypeOf(float64(0)), float64(-2.71828)},
		{result(struct{ A bool }{true}), newTypeOf(false), true},
		{result(struct{ A bool }{false}), newTypeOf(false), false},
		{result(struct{ A string }{"Carl"}), newTypeOf(""), "Carl"},
		{result(struct{ A string }{"Lucas"}), newTypeOf(sql.NullString{}),
			sql.NullString{String: "Lucas", Valid: true}},
		{result(struct{ A interface{} }{nil}), newTypeOf(sql.NullString{}),
			sql.NullString{}},

		// TIME
		{result(struct{ A time.Time }{sometime}), newTypeOf(NullTime{}),
			NullTime{sometime, true}},
		{result(struct{ A interface{} }{nil}), newTypeOf(NullTime{}),
			NullTime{time.Time{}, false}},
	}

	for _, tt := range tests {
		dvr.SetColumns("A").SetResult(tt.result...)
		if err := conn.Query("").ScanRow(tt.v); err != nil {
			panic(err)
		}
		vv := reflect.ValueOf(tt.v)
		v := reflect.Indirect(vv).Interface()
		if !reflect.DeepEqual(v, tt.expected) {
			t.Errorf("loading:\n got: %v (%T)\nwant: %v (%T)", v, v, tt.expected,
				tt.expected)
		}
	}
}

func TestErrNoRows(t *testing.T) {
	var u U
	dvr.SetResult()
	if err := conn.Query("").One(&u); err != sql.ErrNoRows {
		t.Errorf("Query.One should return sql.ErrNoRows if there are no rows\ngot %v", err)
	}

}

func newTypeOf(v interface{}) interface{}   { return reflect.New(reflect.TypeOf(v)).Interface() }
func cols(s ...string) []string             { return s }
func result(v ...interface{}) []interface{} { return v }

func BenchmarkLoadingOne(b *testing.B) {
	dvr.SetColumns("ID", "Name").SetResult(U{1, "John"})
	b.ResetTimer()
	var u U
	for i := 0; i < b.N; i++ {
		conn.Query("").One(&u)
	}
}

func BenchmarkLoadingAll(b *testing.B) {
	dvr.SetColumns("ID", "Name").SetResult(U{1, "John"}, U{2, "Patrik"}, U{3, "Tomáš"})
	b.ResetTimer()
	var u []U
	for i := 0; i < b.N; i++ {
		conn.Query("").All(&u)
	}
}
