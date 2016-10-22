package dialects

import (
	"io"
	"time"
)

// Dialect is the interface that describes a dialect of a particular SQL driver.
type Dialect interface {
	// EscapeIdent safely escapes identificatiors (such as column or table
	// names, etc.)
	EscapeIdent(w io.Writer, ident string)

	// EscapeBool safely escapes boolean variables.
	EscapeBool(w io.Writer, v bool)

	// EscapeString safely escapes strings.
	EscapeString(w io.Writer, s string)

	// EscapeBytes safely escapes byte slices.
	EscapeBytes(w io.Writer, b []byte)

	// EscapeTime safely escapes time.Time variables.
	EscapeTime(w io.Writer, t time.Time)

	// PrintPlaceholderSign prints nth placeholder sign starting from 1.
	PrintPlaceholderSign(w io.Writer, n int)
}

// writeByte is a helper func for Dialect implementators.
func writeByte(w io.Writer, b byte) (n int, err error) {
	return w.Write([]byte{b})
}
