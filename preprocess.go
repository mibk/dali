package dali

import (
	"bytes"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/mibk/dali/drivers"
)

var (
	ErrInvalidSyntax = errors.New("SQL syntax error")
)

// Preprocess processes the sql using the driver.
func Preprocess(driver drivers.Driver, sql string, args []interface{}) (string, error) {
	buf := new(bytes.Buffer)
	pos := 0

	for pos < len(sql) {
		r, w := utf8.DecodeRuneInString(sql[pos:])
		pos += w

		switch r {
		case '[':
			w := strings.IndexRune(sql[pos:], ']')
			if w == -1 {
				return "", ErrInvalidSyntax
			}
			col := sql[pos : pos+w]
			driver.EscapeIdent(buf, col)
			pos += w + 1 // size of ']'
		default:
			buf.WriteRune(r)
		}
	}

	return buf.String(), nil
}
