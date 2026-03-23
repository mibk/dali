package dali

import (
	"bytes"
	"database/sql/driver"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mibk/dali/dialect"
)

// Marshaler is the interface implemented by types that can marshal
// themselves into valid SQL. Any type that implements Marshaler can
// be used as an argument to the ?sql placeholder.
//
// Marshaler is intentionally not accepted by the plain ? placeholder.
// First, ?sql signals to the reader that raw SQL is being injected,
// whereas ? guarantees an escaped value. Second, in prepared
// statements ? emits a driver bind parameter without inspecting the
// argument, so a Marshaler passed to ? would be silently ignored.
type Marshaler interface {
	MarshalSQL(t Translator) (string, error)
}

// A Translator translates SQL queries using a dialect.
type Translator struct {
	dialect      dialect.Dialect
	preparedStmt bool

	err  error
	args []any

	index int // of current arg
	param int // placeholder index
}

func translate(d dialect.Dialect, sql string, args []any) (string, error) {
	t := Translator{
		dialect: d,
	}
	return t.Translate(sql, args)
}

func translatePreparedStmt(d dialect.Dialect, sql string, args []any) (string, error) {
	t := Translator{
		dialect:      d,
		preparedStmt: true,
	}
	return t.Translate(sql, args)
}

// Translate processes sql and args using the dialect specified in t.
// It returns the resulting SQL query and an error, if there is one.
// The value receiver is intentional: each call operates on its own
// copy so that a single Translator can be reused concurrently.
func (t Translator) Translate(sql string, args []any) (string, error) {
	t.args = args
	s, err := t.translate(sql)
	if err != nil {
		return "", fmt.Errorf("dali: %v", err)
	}
	return s, nil
}

func (t Translator) clone() Translator {
	return Translator{
		dialect:      t.dialect,
		preparedStmt: t.preparedStmt,
	}
}

func (t *Translator) checkInterpolationOf(placeholder string) error {
	if t.preparedStmt {
		return fmt.Errorf("%s cannot be used in prepared statements", placeholder)
	}
	return nil
}

func (t *Translator) translate(sql string) (string, error) {
	b := new(bytes.Buffer)
	pos := 0
	for pos < len(sql) {
		r, w := utf8.DecodeRuneInString(sql[pos:])
		pos += w

		switch r {
		case '[':
			w := strings.IndexRune(sql[pos:], ']')
			if w == -1 {
				return "", fmt.Errorf("identifier not terminated")
			}
			col := sql[pos : pos+w]
			t.dialect.EscapeIdent(b, col)
			pos += w + 1 // size of ']'
		case '?':
			start, end := pos, pos
			var expand bool
			for {
				r, w := utf8.DecodeRuneInString(sql[pos:])
				if r < 'a' || r > 'z' {
					if strings.HasPrefix(sql[pos:], "...") {
						pos += 3
						expand = true
					}
					break
				}
				pos += w
				end = pos
			}
			if err := t.interpolate(b, sql[start:end], expand); err != nil {
				return "", err
			}
		default:
			b.WriteRune(r)
		}
	}
	if t.index < len(t.args) {
		return "", fmt.Errorf("only %d args are expected", t.index)
	}
	return b.String(), nil
}

func (t *Translator) nextArg() any {
	if t.index >= len(t.args) {
		t.try(fmt.Errorf("there is not enough args for placeholders"))
		return nil
	}
	v := t.args[t.index]
	t.index++
	return v
}

func (t *Translator) nextParamNumber() int {
	t.param++
	return t.param
}

func (t *Translator) interpolate(b *bytes.Buffer, typ string, expand bool) error {
	if expand {
		switch typ {
		case "":
			t.try(t.checkInterpolationOf("?..."))
			t.try(t.escapeMultipleValues(b, t.nextArg()))
		case "ident":
			idents, ok := t.nextArg().([]string)
			if !ok {
				return fmt.Errorf("?ident... expects the argument to be a []string")
			} else if len(idents) == 0 {
				return fmt.Errorf("empty slice passed to ?ident...")
			}
			for i, ident := range idents {
				if i > 0 {
					b.WriteString(", ")
				}
				t.dialect.EscapeIdent(b, ident)
			}
		case "values":
			t.try(t.checkInterpolationOf("?values..."))
			t.try(t.printMultiValuesClause(b, t.nextArg()))
		default:
			return fmt.Errorf("?%s cannot be expanded (...) or doesn't exist", typ)
		}
	} else {
		switch typ {
		case "":
			if t.preparedStmt {
				t.dialect.PrintPlaceholderSign(b, t.nextParamNumber())
				return nil
			}
			t.try(t.escapeValue(b, t.nextArg()))
		case "ident":
			ident, ok := t.nextArg().(string)
			if !ok {
				return t.try(
					fmt.Errorf("?ident expects the argument to be a string"))
			}
			t.dialect.EscapeIdent(b, ident)
		case "values":
			t.try(t.checkInterpolationOf("?values"))
			t.try(t.printValuesClause(b, t.nextArg()))
		case "set":
			t.try(t.checkInterpolationOf("?set"))
			t.try(t.printSetClause(b, t.nextArg()))
		case "sql":
			switch arg := t.nextArg().(type) {
			case Marshaler:
				sql, err := arg.MarshalSQL(t.clone())
				if err != nil {
					return fmt.Errorf("marshal SQL: %v", err)
				}
				b.WriteString(sql)
			case string:
				b.WriteString(arg)
			default:
				return fmt.Errorf("?sql expects the argument to be a string or Marshaler")
			}
		default:
			return fmt.Errorf("unknown placeholder ?%s", typ)
		}
	}
	return t.err
}

func (t *Translator) try(err error) error {
	if t.err == nil {
		t.err = err
	}
	return t.err
}

var timeType = reflect.TypeFor[time.Time]()

func (t *Translator) escapeValue(b *bytes.Buffer, v any) error {
	vv := reflect.ValueOf(v)
	if valuer, ok := v.(driver.Valuer); ok {
		if vv.Kind() == reflect.Pointer && vv.IsNil() {
			b.WriteString("NULL")
			return nil
		}
		var err error
		if v, err = valuer.Value(); err != nil {
			return err
		}
		vv = reflect.ValueOf(v)
	}
	if v == nil {
		b.WriteString("NULL")
		return nil
	}

	switch vv.Kind() {
	case reflect.Pointer:
		if vv.IsNil() {
			b.WriteString("NULL")
			return nil
		}
		return t.escapeValue(b, vv.Elem().Interface())
	case reflect.Bool:
		t.dialect.EscapeBool(b, vv.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		formatInt(b, vv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		formatUint(b, vv.Uint())
	case reflect.Float32, reflect.Float64:
		formatFloat(b, vv.Float())
	case reflect.String:
		t.dialect.EscapeString(b, vv.String())
	case reflect.Slice:
		if vv.Type().Elem().Kind() == reflect.Uint8 {
			t.dialect.EscapeBytes(b, vv.Bytes())
			break
		}
		return fmt.Errorf("only a slice of bytes supported; got: %T", v)
	case reflect.Struct:
		if vv.Type().ConvertibleTo(timeType) {
			t.dialect.EscapeTime(b, vv.Convert(timeType).Interface().(time.Time))
			break
		}
		fallthrough
	default:
		return fmt.Errorf("invalid argument type: %T", v)
	}
	return nil
}

func formatInt(b *bytes.Buffer, i int64)     { b.WriteString(strconv.FormatInt(i, 10)) }
func formatUint(b *bytes.Buffer, u uint64)   { b.WriteString(strconv.FormatUint(u, 10)) }
func formatFloat(b *bytes.Buffer, f float64) { b.WriteString(strconv.FormatFloat(f, 'f', -1, 64)) }

func (t *Translator) escapeMultipleValues(b *bytes.Buffer, v any) error {
	vv := reflect.ValueOf(v)
	if vv.Kind() != reflect.Slice {
		return fmt.Errorf("?... expects the argument to be a slice")
	}
	length := vv.Len()
	if length == 0 {
		b.WriteString("NULL")
		return nil
	}
	for i := range length {
		if i > 0 {
			b.WriteString(", ")
		}
		if err := t.escapeValue(b, vv.Index(i).Interface()); err != nil {
			return err
		}
	}
	return nil
}

func (t *Translator) printValuesClause(b *bytes.Buffer, v any) error {
	cols, vals, err := t.deriveColsAndVals(v)
	if err != nil {
		return err
	}
	b.WriteRune('(')
	for i, c := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		t.dialect.EscapeIdent(b, c)
	}
	b.WriteString(") VALUES (")
	for i, v := range vals {
		if i > 0 {
			b.WriteString(", ")
		}
		t.try(t.escapeValue(b, v))
	}
	b.WriteRune(')')
	return nil
}

func (t *Translator) printSetClause(b *bytes.Buffer, v any) error {
	cols, vals, err := t.deriveColsAndVals(v)
	if err != nil {
		return err
	}
	b.WriteString("SET ")
	for i, c := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		v := vals[i]
		t.dialect.EscapeIdent(b, c)
		b.WriteString(" = ")
		t.try(t.escapeValue(b, v))
	}
	return nil
}

// deriveColsAndVals derives column names from an underlying type of v and returns
// them together with the corresponding values.
func (t *Translator) deriveColsAndVals(v any) (cols []string, vals []any, err error) {
	switch v := v.(type) {
	case Map:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		cols = make([]string, 0, len(keys))
		vals = make([]any, 0, len(keys))
		for _, col := range keys {
			cols = append(cols, col)
			vals = append(vals, v[col])
		}
	default:
		vv := reflect.ValueOf(v)
		if vv.Kind() == reflect.Pointer {
			vv = reflect.Indirect(vv)
		}
		if vv.Kind() != reflect.Struct {
			return nil, nil, fmt.Errorf("argument must be a pointer to a struct")
		}
		var indexes [][]int
		cols, indexes = colNamesAndFieldIndexes(vv.Type(), true)
		vals = valuesByFieldIndexes(vv, indexes)
	}
	if len(cols) == 0 {
		err = errNoCols(v)
	}
	return cols, vals, err
}

func (t *Translator) printMultiValuesClause(b *bytes.Buffer, v any) error {
	errInvalidArg := fmt.Errorf("?values... expects the argument to be a slice of structs")
	vv := reflect.ValueOf(v)
	if vv.Kind() != reflect.Slice {
		return errInvalidArg
	}
	el := vv.Type().Elem()
	isPtr := false
	if el.Kind() == reflect.Pointer {
		el = el.Elem()
		isPtr = true
	}
	if el.Kind() != reflect.Struct {
		return errInvalidArg
	}
	if vv.Len() == 0 {
		return fmt.Errorf("empty slice passed to ?values...")
	}
	cols, indexes := colNamesAndFieldIndexes(el, true)
	if len(cols) == 0 {
		return errNoCols(v)
	}
	b.WriteRune('(')
	for i, c := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		t.dialect.EscapeIdent(b, c)
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
			if i > 0 {
				b.WriteString(", ")
			}
			t.try(t.escapeValue(b, v))
		}
		b.WriteRune(')')
		if i != length-1 {
			b.WriteRune(',')
		}
	}
	return nil
}

func errNoCols(v any) error {
	return fmt.Errorf("no columns derived from %T", v)
}

func valuesByFieldIndexes(v reflect.Value, indexes [][]int) []any {
	vals := make([]any, 0, len(indexes))
	for _, index := range indexes {
		vals = append(vals, v.FieldByIndex(index).Interface())
	}
	return vals
}
