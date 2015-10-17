package dali

import "testing"

var placeholderTests = []struct {
	sql    string
	args   []interface{}
	expSQL string
}{
	{"SELECT * FROM [x] WHERE a = ? AND b = ?", Args{3, "four"},
		"SELECT * FROM {x} WHERE a = 3 AND b = 'four'"},

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

	// embedded structs
	{"INSERT ?values", Args{E{1, Name{"John", "Doe"}}},
		"INSERT ({id}, {first}, {last}) VALUES (1, 'John', 'Doe')"},
}

func TestPlaceholders(t *testing.T) {
	preproc := NewPreprocessor(FakeDriver{})
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

var errorTests = []struct {
	sql  string
	args []interface{}
	err  string
}{
	{"SELECT [user FROM", Args{}, "dali: identifier not terminated"},
	{"INSERT INTO ?ident", Args{}, "dali: there is not enough args for placeholders"},
	{"SELECT ?, ?", Args{3, 4, 5}, "dali: only 2 args are expected"},
	{"INSERT INTO ?ident", Args{5}, "dali: ?ident expects the argument to be a string"},
	{"INSERT INTO ?u", Args{5}, "dali: unknown placeholder ?u"},
	{"INSERT INTO ?", Args{func() {}}, "dali: invalid argument type: func"},
	{"WHERE IN ?...", Args{14}, "dali: ?... expects the argument to be a slice"},
	{"INSERT ?values", Args{ptrPtrUser()}, "dali: argument must be a pointer to a struct"},
	{"INSERT ?values...", Args{[]**User{}},
		"dali: ?values... expects the argument to be a slice of structs"},
}

func TestErrors(t *testing.T) {
	preproc := NewPreprocessor(FakeDriver{})
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

func ptrPtrUser() **User {
	u := &User{}
	return &u
}
