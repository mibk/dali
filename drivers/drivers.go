package drivers

import (
	"io"
	"unicode/utf8"
)

type Driver interface {
	EscapeIdent(w io.Writer, ident string)
	EscapeBool(w io.Writer, v bool)
	EscapeString(w io.Writer, s string)
}

func writeRune(w io.Writer, r rune) (n int, err error) {
	if r < utf8.RuneSelf {
		return w.Write([]byte{byte(r)})
	}
	runeBuf := make([]byte, 4)
	n = utf8.EncodeRune(runeBuf, r)
	return w.Write(runeBuf[:n])
}

func writeString(w io.Writer, s string) (n int, err error) {
	return w.Write([]byte(s))
}
