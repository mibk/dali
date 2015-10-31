package dali

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

var placeholderTests = []struct {
	sql    string
	args   []interface{}
	expSQL string
}{
	{"SELECT * FROM [x] WHERE a = ? AND b = ?", Args{3, "four"},
		"SELECT * FROM {x} WHERE a = 3 AND b = 'four'"},

	{"SELECT ?ident.id", Args{"my_table"}, "SELECT {my_table}.id"},
	{"SELECT ?ident...", Args{[]string{"col1", "col2", "another"}},
		"SELECT {col1}, {col2}, {another}"},

	{"SELECT ?raw", Args{"* FROM user"}, "SELECT * FROM user"},

	{"INSERT INTO [user] ?values", Args{User{1, "Salvador", 0}},
		"INSERT INTO {user} ({id}, {user_name}) VALUES (1, 'Salvador')"},
	{"INSERT INTO [user] ?values...", Args{[]User{
		{1, "Salvador", 0},
		{2, "John", 1},
	}},
		"INSERT INTO {user} ({id}, {user_name}) VALUES (1, 'Salvador'), " +
			"(2, 'John')"},
	{"UPDATE [user] ?set WHERE [id] = ?", Args{User{10, "Selma", 0}, 1},
		"UPDATE {user} SET {id} = 10, {user_name} = 'Selma' WHERE {id} = 1"},

	{"SELECT * FROM user WHERE id IN (?...)", Args{[]int{1, 4, 7, 11}},
		"SELECT * FROM user WHERE id IN (1, 4, 7, 11)"},

	{"INSERT ?values", Args{&User{1, "Rudolf", 0}},
		"INSERT ({id}, {user_name}) VALUES (1, 'Rudolf')"},
	{"INSERT ?values...", Args{[]*User{{1, "Martin", 0}}},
		"INSERT ({id}, {user_name}) VALUES (1, 'Martin')"},
	{"INSERT ?values", Args{V{1, "Syd"}},
		"INSERT ({V_name}) VALUES ('Syd')"},

	// nested structs
	{"INSERT ?values", Args{E{1, Name{"John", "Doe"}}},
		"INSERT ({id}, {first}, {last}) VALUES (1, 'John', 'Doe')"},

	// ignored nested structs
	{"?values", Args{SpecialStruct{"Waking up", parseTime("2015-04-05 06:07:08"), NullTime{}}},
		"({event}, {started}, {finished}) VALUES ('Waking up', " +
			"'2015-04-05 06:07:08 +0000 UTC', NULL)"},
	{"?values", Args{SpecialStruct{"Waking up", parseTime("2015-04-05 06:07:08"),
		NullTime{parseTime("2015-04-05 06:38:15"), true}}},
		"({event}, {started}, {finished}) VALUES ('Waking up', " +
			"'2015-04-05 06:07:08 +0000 UTC', '2015-04-05 06:38:15 +0000 UTC')"},

	// ignore valuer but not scanner
	{"?values", Args{VS{Val{2, 3}, Scan{"A", "B"}}}, "({val}, {a}, {b}) VALUES (5, 'A', 'B')"},

	// ,omitinsert
	{"INSERT ?values", Args{Omit{Name: "John", Age: 21}},
		"INSERT ({name}, {age}) VALUES ('John', 21)"},
	{"INSERT ?values", Args{Omit2{Name: "Rudolf", Age: 28}},
		"INSERT ({name}, {age}) VALUES ('Rudolf', 28)"},
}

func TestPlaceholders(t *testing.T) {
	preproc := NewPreprocessor(FakeDialect{})
	for _, tt := range placeholderTests {
		str, err := preproc.Process(tt.sql, tt.args)
		if err != nil {
			t.Fatalf("unexpected err: %s:\n %v", tt.sql, err)
		}
		if str != tt.expSQL {
			t.Errorf("\n got: %v\nwant: %v", str, tt.expSQL)
		}
	}
}

type Args []interface{}

type User struct {
	ID     int64
	Name   string `db:"user_name"`
	Ignore int    `db:"-"`
}

type Omit struct {
	ID   int64 `db:",omitinsert"`
	Name string
	Age  int
}

type Omit2 struct {
	ID   int64 `db:"Id_user,omitinsert"`
	Name string
	Age  int
}

type Omit2res struct {
	Id_user int64
	Name    string
	Age     int
}

type SpecialStruct struct {
	Event    string
	Started  time.Time
	Finished NullTime
}

type SpecialStructRes struct {
	Event    string
	Started  time.Time
	Finished interface{}
}

type Val struct {
	A int
	B int
}

func (v Val) Value() (driver.Value, error) { return v.A + v.B, nil }

type Scan struct {
	A string
	B string
}

var _ sql.Scanner = new(Scan)

func (s *Scan) Scan(v interface{}) error {
	if v, ok := v.(string); ok {
		str := strings.Split(v, ":")
		if len(str) != 2 {
			panic("v should contain just one colon (:)")
		}
		s.A, s.B = str[0], str[1]
		return nil
	}
	panic("v is not string")
}

type VS struct {
	Val  Val
	Scan Scan
}

var errorTests = []struct {
	sql  string
	args []interface{}
	err  string
}{
	{"SELECT [user FROM", Args{}, "dali: identifier not terminated"},
	{"INSERT INTO ?ident", Args{}, "dali: there is not enough args for placeholders"},
	{"SELECT ?, ?", Args{3, 4, 5}, "dali: only 2 args are expected"},
	{"INSERT INTO ?ident", Args{5}, "dali: ?ident expects the argument to be a string"},
	{"INSERT INTO ?raw", Args{5}, "dali: ?raw expects the argument to be a string"},
	{"INSERT INTO ?u", Args{5}, "dali: unknown placeholder ?u"},
	{"INSERT ?u...", Args{nil}, "dali: ?u cannot be expanded (...) or doesn't exist"},
	{"INSERT INTO ?", Args{func() {}}, "dali: invalid argument type: func()"},
	{"WHERE IN ?...", Args{14}, "dali: ?... expects the argument to be a slice"},
	{"INSERT ?values", Args{ptrPtrUser()}, "dali: argument must be a pointer to a struct"},
	{"INSERT ?values...", Args{[]**User{}},
		"dali: ?values... expects the argument to be a slice of structs"},
}

func TestErrors(t *testing.T) {
	preproc := NewPreprocessor(FakeDialect{})
	for _, tt := range errorTests {
		_, err := preproc.Process(tt.sql, tt.args)
		if err == nil {
			t.Errorf("%s: an error was expected but none given", tt.sql)
			continue
		}
		if err.Error() != tt.err {
			t.Errorf("%s:\n got: %v,\nwant: %v", tt.sql, err, tt.err)
		}
	}
}

const sqlTimeFmt = "2006-01-02 15:04:05"

func parseTime(s string) time.Time {
	t, err := time.Parse(sqlTimeFmt, s)
	if err != nil {
		panic(err)
	}
	return t
}

var sometime = parseTime("2015-03-05 10:42:43")

var typesTests = []struct {
	sql    string
	args   []interface{}
	expSQL string
}{
	{"?, ?", Args{true, false}, "true, false"},
	{"?, ?, ?, ?, ?", Args{int(-1), int8(-2), int16(-3), int32(-4), int64(-5)},
		"-1, -2, -3, -4, -5"},
	{"?, ?, ?, ?, ?", Args{uint(1), uint8(2), uint16(3), uint32(4), uint64(5)},
		"1, 2, 3, 4, 5"},
	{"?, ?", Args{float32(1.5), float64(2.71828)}, "1.5, 2.71828"},
	{"?", Args{"příliš žluťoučký kůň úpěl ďábelské ódy"},
		"'příliš žluťoučký kůň úpěl ďábelské ódy'"},
	{"?", Args{[]byte("binary text")}, "`binary text`"},
	{"?", Args{sometime}, "'2015-03-05 10:42:43 +0000 UTC'"},

	// NULL
	{"?, ?", Args{sql.NullString{String: "Homer", Valid: true}, sql.NullString{String: "Homer"}},
		"'Homer', NULL"},
	{"?, ?", Args{NullTime{sometime, true}, NullTime{sometime, false}},
		"'2015-03-05 10:42:43 +0000 UTC', NULL"},
}

func TestPreprocessingTypes(t *testing.T) {
	preproc := NewPreprocessor(FakeDialect{})
	preproc.setMapperFunc(func(s string) string { return s })
	for _, tt := range typesTests {
		str, err := preproc.Process(tt.sql, tt.args)
		if err != nil {
			t.Fatalf("unexpected err: %s:\n %v", tt.sql, err)
		}
		if str != tt.expSQL {
			t.Errorf("\n got: %v\nwant: %v", str, tt.expSQL)
		}
	}
}

func ptrPtrUser() **User {
	u := &User{}
	return &u
}

var preparedStmtTests = []struct {
	sql     string
	args    []interface{}
	wantSQL string
	wantErr string
}{

	{"SELECT ?ident WHERE [id] = ?", Args{"name"}, "SELECT {name} WHERE {id} = &1", ""},
	{"WHERE [name] LIKE ? AND [age] < ?", Args{}, "WHERE {name} LIKE &1 AND {age} < &2", ""},

	// arg count mismatch
	{"SELECT ?ident", Args{}, "", "dali: there is not enough args for placeholders"},
	{"SELECT ?ident", Args{"name", 3}, "", "dali: only 1 args are expected"},

	// unsupported placeholders
	{"SELECT ?...", Args{}, "", "dali: ?... cannot be used in prepared statements"},
	{"INSERT ?values", Args{}, "", "dali: ?values cannot be used in prepared statements"},
	{"INSERT ?values...", Args{}, "", "dali: ?values... cannot be used in prepared statements"},
	{"INSERT ?set", Args{}, "", "dali: ?set cannot be used in prepared statements"},
}

func TestPreparedStmts(t *testing.T) {
	preproc := NewPreprocessor(FakeDialect{})
	for _, tt := range preparedStmtTests {
		str, err := preproc.ProcessPreparedStmt(tt.sql, tt.args)
		var gotErr string
		if err != nil {
			gotErr = err.Error()
		}
		if gotErr != tt.wantErr {
			var wantErr error
			if tt.wantErr != "" {
				wantErr = fmt.Errorf(tt.wantErr)
			}
			t.Errorf("%s:\ngot err: %v\n   want: %v", tt.sql, err, wantErr)
		}
		if str != tt.wantSQL {
			t.Errorf("\n got: %v\nwant: %v", str, tt.wantSQL)
		}
	}
}

type NopDriver struct{}

func (NopDriver) EscapeIdent(w io.Writer, ident string)   {}
func (NopDriver) EscapeBool(w io.Writer, v bool)          {}
func (NopDriver) EscapeString(w io.Writer, s string)      {}
func (NopDriver) EscapeBytes(w io.Writer, b []byte)       {}
func (NopDriver) EscapeTime(w io.Writer, t time.Time)     {}
func (NopDriver) PrintPlaceholderSign(w io.Writer, n int) {}

var preproc = NewPreprocessor(NopDriver{})

func init() {
	preproc.setMapperFunc(func(s string) string { return s })
}

func BenchmarkColumnEscaping(b *testing.B) {
	for i := 0; i < b.N; i++ {
		preproc.Process(`SELECT [first_name], [last_name] FROM [user]`, nil)
	}
}

func BenchmarkBasicInterpolation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		preproc.Process(`INSERT INTO ?ident (a, b) VALUES (?, ?)`,
			Args{"table", 3, "four"})
	}
}

func BenchmarkSimpleValueExpansion(b *testing.B) {
	for i := 0; i < b.N; i++ {
		preproc.Process(`SELECT WHERE a IN (?...)`,
			Args{[]int64{8, 99, 1013, 1202}})
	}
}

func BenchmarkValuesClause(b *testing.B) {
	for i := 0; i < b.N; i++ {
		preproc.Process(`INSERT ?values`, Args{struct {
			ID     int64
			Name   string
			Create time.Time
		}{56, "Jacob", time.Time{}}})
	}
}

func BenchmarkSetClause(b *testing.B) {
	for i := 0; i < b.N; i++ {
		preproc.Process(`UPDATE ?set`, Args{struct {
			ID   int64 `db:",omitinsert"`
			Name string
			Age  int
		}{0, "Veronika", 18}})
	}
}

func BenchmarkMultiValuesClause(b *testing.B) {
	for i := 0; i < b.N; i++ {
		preproc.Process(`INSERT ?values...`, Args{[]struct {
			ID   int64
			Name string
		}{
			{56, "Jacob"},
			{104, "Luboš"},
			{889, "Ibrahim"},
		}})
	}
}
