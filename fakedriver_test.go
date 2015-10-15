package dali

import (
	"fmt"
	"io"
)

type FakeDriver struct{}

func (FakeDriver) EscapeIdent(w io.Writer, ident string) { fmt.Fprintf(w, "{%s}", ident) }
func (FakeDriver) EscapeBool(w io.Writer, v bool)        { fmt.Fprint(w, v) }
func (FakeDriver) EscapeString(w io.Writer, s string)    { fmt.Fprintf(w, "'%s'", s) }
