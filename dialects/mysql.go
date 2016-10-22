package dialects

import (
	"io"
	"strings"
	"time"
)

// MySQL returns the MySQL dialect.
func MySQL() Dialect {
	return mySQL{}
}

type mySQL struct{}

func (mySQL) EscapeIdent(w io.Writer, ident string) {
	writeByte(w, '`')
	r := strings.NewReplacer("`", "``")
	io.WriteString(w, r.Replace(ident))
	writeByte(w, '`')
}

func (mySQL) EscapeBool(w io.Writer, v bool) {
	if v {
		writeByte(w, '1')
	} else {
		writeByte(w, '0')
	}
}

func (mySQL) EscapeString(w io.Writer, s string) {
	escapeBytes(w, []byte(s))
}

func (mySQL) EscapeBytes(w io.Writer, b []byte) {
	io.WriteString(w, "_binary")
	escapeBytes(w, b)
}

func escapeBytes(w io.Writer, bytes []byte) {
	writeByte(w, '\'')
	for _, b := range bytes {
		// See https://dev.mysql.com/doc/refman/5.7/en/string-literals.html
		// for more information on how to escape string literals in MySQL.
		switch b {
		case 0:
			io.WriteString(w, `\0`)
		case '\'':
			io.WriteString(w, `\'`)
		case '"':
			io.WriteString(w, `\"`)
		case '\b':
			io.WriteString(w, `\b`)
		case '\n':
			io.WriteString(w, `\n`)
		case '\r':
			io.WriteString(w, `\r`)
		case '\t':
			io.WriteString(w, `\t`)
		case 0x1A:
			io.WriteString(w, `\Z`)
		case '\\':
			io.WriteString(w, `\\`)
		default:
			writeByte(w, b)
		}
	}
	writeByte(w, '\'')
}

const mysqlTimeFormat = "2006-01-02 15:04:05"

func (mySQL) EscapeTime(w io.Writer, t time.Time) {
	writeByte(w, '\'')
	io.WriteString(w, t.Format(mysqlTimeFormat))
	writeByte(w, '\'')
}

func (mySQL) PrintPlaceholderSign(w io.Writer, n int) {
	writeByte(w, '?')
}
