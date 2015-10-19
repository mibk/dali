package drivers

import (
	"io"
	"strings"
	"time"
)

type MySQL struct{}

func (MySQL) EscapeIdent(w io.Writer, ident string) {
	writeRune(w, '`')
	r := strings.NewReplacer("`", "``")
	writeString(w, r.Replace(ident))
	writeRune(w, '`')
}

func (MySQL) EscapeBool(w io.Writer, v bool) {
	if v {
		writeRune(w, '1')
	} else {
		writeRune(w, '0')
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
	writeRune(w, '\'')
	for _, b := range bytes {
		switch b {
		case '\'':
			writeString(w, `\'`)
		case '"':
			writeString(w, `\"`)
		case '\\':
			writeString(w, `\\`)
		case '\n':
			writeString(w, `\n`)
		case '\r':
			writeString(w, `\r`)
		case 0:
			writeString(w, `\x00`)
		case 0x1A:
			writeString(w, `\x1a`)
		default:
			writeRune(w, rune(b))
		}
	}
	writeRune(w, '\'')
}

const mysqlTimeFormat = "2006-01-02 15:04:05"

func (MySQL) EscapeTime(w io.Writer, t time.Time) {
	writeRune(w, '\'')
	writeString(w, t.Format(mysqlTimeFormat))
	writeRune(w, '\'')
}
