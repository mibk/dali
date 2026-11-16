package dialect

import (
	"io"
	"time"
)

// Escaper is the interface that describes a dialect of a particular SQL driver.
type Escaper interface {
	// EscapeIdent safely escapes identifiers (such as column or table
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

	// WritePlaceholder writes the nth placeholder starting from 1.
	WritePlaceholder(w io.Writer, n int)
}

// Deprecated: Use Escaper.
type Dialect = Escaper

// writeByte is a helper func for Escaper implementors.
func writeByte(w io.Writer, b byte) (n int, err error) {
	return w.Write([]byte{b})
}
