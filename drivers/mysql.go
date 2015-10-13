package drivers

import (
	"io"
	"strings"
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
	writeRune(w, '\'')
	for _, r := range []rune(s) {
		switch r {
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
		case 0x1a:
			writeString(w, `\x1a`)
		default:
			writeRune(w, r)
		}
	}
	writeRune(w, '\'')
}
