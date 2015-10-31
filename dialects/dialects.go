package dialects

import (
	"io"
	"time"
)

type Dialect interface {
	EscapeIdent(w io.Writer, ident string)
	EscapeBool(w io.Writer, v bool)
	EscapeString(w io.Writer, s string)
	EscapeBytes(w io.Writer, b []byte)
	EscapeTime(w io.Writer, t time.Time)
	// Prints nth placeholder sign starting from 1.
	PrintPlaceholderSign(w io.Writer, n int)
}

func writeByte(w io.Writer, b byte) (n int, err error) {
	return w.Write([]byte{b})
}

func writeString(w io.Writer, s string) (n int, err error) {
	return w.Write([]byte(s))
}
