package dali

import (
	"fmt"
	"io"
	"testing"
)

func TestPreprocess(t *testing.T) {
	tests := []struct {
		sql    string
		args   []interface{}
		expSql string
		expErr error
	}{
		{"SELECT * FROM [x] WHERE a = ? AND b = ?", []interface{}{3, "four"},
			"SELECT * FROM {x} WHERE a = 3 AND b = 'four'", nil},
	}

	preproc := NewPreprocessor(FakeDriver{})
	for _, test := range tests {
		str, err := preproc.Process(test.sql, test.args)
		if err != test.expErr {
			t.Errorf("\ngot error: %v\nwant: %v", err, test.expErr)
		}
		if str != test.expSql {
			t.Errorf("\ngot: %v\nwant: %v", str, test.expSql)
		}
	}
}

type FakeDriver struct{}

func (FakeDriver) EscapeIdent(w io.Writer, ident string) { fmt.Fprintf(w, "{%s}", ident) }
func (FakeDriver) EscapeBool(w io.Writer, v bool)        { fmt.Fprint(w, v) }
func (FakeDriver) EscapeString(w io.Writer, s string)    { fmt.Fprintf(w, "'%s'", s) }
