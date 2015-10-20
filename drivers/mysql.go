package drivers

import (
	"io"
	"strings"
	"time"
)

type MySQL struct{}

func (MySQL) EscapeIdent(w io.Writer, ident string) {
	writeByte(w, '`')
	r := strings.NewReplacer("`", "``")
	writeString(w, r.Replace(ident))
	writeByte(w, '`')
}

func (MySQL) EscapeBool(w io.Writer, v bool) {
	if v {
		writeByte(w, '1')
	} else {
		writeByte(w, '0')
	}
}

func (MySQL) EscapeString(w io.Writer, s string) {
	escapeBytes(w, []byte(s))
}

func (MySQL) EscapeBytes(w io.Writer, b []byte) {
	writeString(w, "_binary")
	escapeBytes(w, b)
}

func escapeBytes(w io.Writer, bytes []byte) {
	writeByte(w, '\'')
	for _, b := range bytes {
		// See https://dev.mysql.com/doc/refman/5.7/en/string-literals.html
		// for more information on how to escape string literals in MySQL.
		switch b {
		case 0:
			writeString(w, `\0`)
		case '\'':
			writeString(w, `\'`)
		case '"':
			writeString(w, `\"`)
		case '\b':
			writeString(w, `\b`)
		case '\n':
			writeString(w, `\n`)
		case '\r':
			writeString(w, `\r`)
		case '\t':
			writeString(w, `\t`)
		case 0x1A:
			writeString(w, `\Z`)
		case '\\':
			writeString(w, `\\`)
		default:
			writeByte(w, b)
		}
	}
	writeByte(w, '\'')
}

const mysqlTimeFormat = "2006-01-02 15:04:05"

func (MySQL) EscapeTime(w io.Writer, t time.Time) {
	writeByte(w, '\'')
	writeString(w, t.Format(mysqlTimeFormat))
	writeByte(w, '\'')
}
