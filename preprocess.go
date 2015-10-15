package dali

import (
	"bytes"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mibk/dali/drivers"
)

var (
	ErrArgumentMismatch   = errors.New("mismatch between placeholders and arguments")
	ErrInvalidSyntax      = errors.New("SQL syntax error")
	ErrInvalidValue       = errors.New("trying to interpolate invalid value")
	ErrInvalidPlaceholder = errors.New("invalid placeholder")
	ErrNotUTF8            = errors.New("invalid UTF-8")
)

type Preprocessor struct {
	driver drivers.Driver
}

func NewPreprocessor(driver drivers.Driver) *Preprocessor {
	return &Preprocessor{driver}
}

// Preprocess processes the sql using the driver.
func (p *Preprocessor) Process(sql string, args []interface{}) (string, error) {
	b := new(bytes.Buffer)
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
			p.driver.EscapeIdent(b, col)
			pos += w + 1 // size of ']'
		case '?':
			start := pos
			for {
				r, w := utf8.DecodeRuneInString(sql[pos:])
				if (r < 'a' || r > 'z') && r != '.' {
					break
				}
				pos += w
			}
			if argIndex >= len(args) {
				return "", ErrArgumentMismatch
			}
			if err := p.interpolate(b, sql[start:pos], args[argIndex]); err != nil {
				return "", err
			}
			argIndex++
		default:
			b.WriteRune(r)
		}
	}

	return b.String(), nil
}

func (p *Preprocessor) interpolate(b *bytes.Buffer, typ string, v interface{}) error {
	switch typ {
	case "":
		return p.escapeValue(b, v)
	case "...":
		return p.escapeMultipleValues(b, v)
	case "ident":
		col, ok := v.(string)
		if !ok {
			return ErrInvalidValue
		}
		p.driver.EscapeIdent(b, col)
	case "values":
		return p.printValuesClause(b, v)
	case "values...":
		return p.printMultiValuesClause(b, v)
	case "set":
		return p.printSetClause(b, v)
	default:
		return fmt.Errorf("invalid placeholder: %s", typ)
	}
	return nil
}

func (p *Preprocessor) escapeValue(b *bytes.Buffer, v interface{}) error {
	if valuer, ok := v.(driver.Valuer); ok {
		var err error
		if v, err = valuer.Value(); err != nil {
			return err
		}
	}
	vv := reflect.ValueOf(v)
	switch vv.Kind() {
	case reflect.Bool:
		p.driver.EscapeBool(b, vv.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		b.WriteString(strconv.FormatInt(vv.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		b.WriteString(strconv.FormatUint(vv.Uint(), 10))
	case reflect.Float32, reflect.Float64:
		b.WriteString(strconv.FormatFloat(vv.Float(), 'f', -1, 64))
	case reflect.String:
		s := vv.String()
		if !utf8.ValidString(s) {
			return ErrNotUTF8
		}
		p.driver.EscapeString(b, s)
	default:
		return ErrInvalidValue
	}
	return nil
}

func (p *Preprocessor) escapeMultipleValues(b *bytes.Buffer, v interface{}) error {
	vv := reflect.ValueOf(v)
	if vv.Kind() != reflect.Slice {
		return ErrInvalidValue
	}
	length := vv.Len()
	for i := 0; i < length; i++ {
		if err := p.escapeValue(b, vv.Index(i).Interface()); err != nil {
			return err
		}
		if i != length-1 {
			b.WriteString(", ")
		}
	}
	return nil
}

// Map is just an alias for map[string]interface{}. It's shorter.
type Map map[string]interface{}

func (p *Preprocessor) printValuesClause(b *bytes.Buffer, v interface{}) error {
	cols, vals, err := deriveColsAndVals(v)
	if err != nil {
		return err
	}
	b.WriteRune('(')
	for i, c := range cols {
		p.driver.EscapeIdent(b, c)
		if i != len(vals)-1 {
			b.WriteString(", ")
		}
	}
	b.WriteString(") VALUES (")
	for i, v := range vals {
		p.escapeValue(b, v)
		if i != len(vals)-1 {
			b.WriteString(", ")
		}
	}
	b.WriteRune(')')
	return nil
}

func (p *Preprocessor) printSetClause(b *bytes.Buffer, v interface{}) error {
	cols, vals, err := deriveColsAndVals(v)
	if err != nil {
		return err
	}
	b.WriteString("SET ")
	for i, c := range cols {
		v := vals[i]
		p.driver.EscapeIdent(b, c)
		b.WriteString(" = ")
		p.escapeValue(b, v)
		if i != len(vals)-1 {
			b.WriteString(", ")
		}
	}
	return nil
}

// deriveColsAndVals derives column names from an underlying type of v and returns
// them together with the corresponding values.
func deriveColsAndVals(v interface{}) (cols []string, vals []interface{}, err error) {
	switch v := v.(type) {
	case Map:
		for k, v := range v {
			cols = append(cols, k)
			vals = append(vals, v)
		}
	default:
		vv := reflect.ValueOf(v)
		if vv.Kind() == reflect.Ptr {
			vv = reflect.Indirect(vv)
		}
		if vv.Kind() != reflect.Struct {
			return nil, nil, ErrInvalidValue
		}
		var indexes []int
		cols, indexes = colNamesAndFieldIndexes(vv.Type())
		vals = valuesByFieldIndexes(vv, indexes)

	}
	return
}

func (p *Preprocessor) printMultiValuesClause(b *bytes.Buffer, v interface{}) error {
	vv := reflect.ValueOf(v)
	if vv.Kind() != reflect.Slice {
		return ErrInvalidValue
	}
	el := vv.Type().Elem()
	isPtr := false
	if el.Kind() == reflect.Ptr {
		el = el.Elem()
		isPtr = true
	}
	if el.Kind() != reflect.Struct {
		return ErrInvalidValue
	}
	cols, indexes := colNamesAndFieldIndexes(el)
	b.WriteRune('(')
	for i, c := range cols {
		p.driver.EscapeIdent(b, c)
		if i != len(cols)-1 {
			b.WriteString(", ")
		}
	}
	b.WriteString(") VALUES")
	for i, length := 0, vv.Len(); i < length; i++ {
		b.WriteString(" (")
		el := vv.Index(i)
		if isPtr {
			el = reflect.Indirect(el)
		}
		vals := valuesByFieldIndexes(el, indexes)
		for i, v := range vals {
			p.escapeValue(b, v)
			if i != len(vals)-1 {
				b.WriteString(", ")
			}
		}
		b.WriteRune(')')
		if i != length-1 {
			b.WriteRune(',')
		}
	}
	return nil
}

// colNamesAndFieldIndexes derives column names from a struct type and returns
// them together with the indexes of used fields. typ must by a struct type.
func colNamesAndFieldIndexes(typ reflect.Type) (cols []string, indexes []int) {
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.PkgPath != "" { // Is exported?
			continue
		}
		name := f.Tag.Get("db")
		if name == "" {
			name = ToUnderscore(f.Name)
		} else if name == "-" {
			continue
		}
		cols = append(cols, name)
		indexes = append(indexes, i)
	}
	return
}

func valuesByFieldIndexes(v reflect.Value, indexes []int) (vals []interface{}) {
	for _, f := range indexes {
		vals = append(vals, v.Field(f).Interface())
	}
	return
}
