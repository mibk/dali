package dali

import "testing"

var placeholderTests = []struct {
	sql    string
	args   []interface{}
	expSQL string
	expErr error
}{
	{"SELECT * FROM [x] WHERE a = ? AND b = ?", Args{3, "four"},
		"SELECT * FROM {x} WHERE a = 3 AND b = 'four'", nil},

	{"INSERT INTO [user] ?values", Args{User{1, "Salvador", 0}},
		"INSERT INTO {user} ({id}, {user_name}) VALUES (1, 'Salvador')", nil},
	{"INSERT INTO [user] ?values...", Args{[]User{
		{1, "Salvador", 0},
		{2, "John", 1},
	}},
		"INSERT INTO {user} ({id}, {user_name}) VALUES (1, 'Salvador'), " +
			"(2, 'John')", nil},
	{"UPDATE [user] ?set WHERE [id] = ?", Args{User{10, "Selma", 0}, 1},
		"UPDATE {user} SET {id} = 10, {user_name} = 'Selma' WHERE {id} = 1", nil},
}

func TestPlaceholders(t *testing.T) {
	preproc := NewPreprocessor(FakeDriver{})
	for _, test := range placeholderTests {
		str, err := preproc.Process(test.sql, test.args)
		if err != test.expErr {
			t.Errorf("\ngot error: %v\nwant: %v", err, test.expErr)
		}
		if str != test.expSQL {
			t.Errorf("\n got: %v\nwant: %v", str, test.expSQL)
		}
	}
}

type Args []interface{}

type User struct {
	ID     int64
	Name   string `db:"user_name"`
	Ignore int    `db:"-"`
}
