package dali

import (
	"bytes"
	"database/sql/driver"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mibk/dali/drivers"
)

var (
	ErrArgumentMismatch = errors.New("mismatch between placeholders and arguments")
	ErrInvalidSyntax    = errors.New("SQL syntax error")
	ErrInvalidValue     = errors.New("trying to interpolate invalid value")
	ErrNotUTF8          = errors.New("invalid UTF-8")
)

type Preprocessor struct {
	driver drivers.Driver
}

func NewPreprocessor(driver drivers.Driver) *Preprocessor {
	return &Preprocessor{driver}
}

// Preprocess processes the sql using the driver.
func (p *Preprocessor) Process(sql string, args []interface{}) (string, error) {
	buf := new(bytes.Buffer)
	pos := 0
	argIndex := 0
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
			p.driver.EscapeIdent(buf, col)
			pos += w + 1 // size of ']'
		case '?':
			if argIndex >= len(args) {
				return "", ErrArgumentMismatch
			}
			if err := p.interpolate(buf, args[argIndex]); err != nil {
				return "", err
			}
			argIndex++
		default:
			buf.WriteRune(r)
		}
	}

	return buf.String(), nil
}

func (p *Preprocessor) interpolate(buf *bytes.Buffer, v interface{}) error {
	if valuer, ok := v.(driver.Valuer); ok {
		var err error
		if v, err = valuer.Value(); err != nil {
			return err
		}
	}

	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.Bool:
		p.driver.EscapeBool(buf, val.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		buf.WriteString(strconv.FormatInt(val.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		buf.WriteString(strconv.FormatUint(val.Uint(), 10))
	case reflect.Float32, reflect.Float64:
		buf.WriteString(strconv.FormatFloat(val.Float(), 'f', -1, 64))
	case reflect.String:
		s := val.String()
		if !utf8.ValidString(s) {
			return ErrNotUTF8
		}
		p.driver.EscapeString(buf, s)
	default:
		return ErrInvalidValue
	}
	return nil
}
