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
type Marshaler interface {
	MarshalSQL(t Translator) (string, error)
}

// A Translator translates SQL queries using a dialect.
type Translator struct {
	dialect      dialect.Dialect
	preparedStmt bool

	err  error
	args []interface{}

	index int // of current arg
	param int // placeholder index
}

func translate(d dialect.Dialect, sql string, args []interface{}) (string, error) {
	t := Translator{
		dialect: d,
	}
	return t.Translate(sql, args)
}
func translatePreparedStmt(d dialect.Dialect, sql string, args []interface{}) (string, error) {
	t := Translator{
		dialect:      d,
		preparedStmt: true,
	}
	return t.Translate(sql, args)
}

// Translate processes sql and args using the dialect specified in t.
// It returns the resulting SQL query and an error, if there is one.
func (t Translator) Translate(sql string, args []interface{}) (string, error) {
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

func (p *Translator) checkInterpolationOf(placeholder string) error {
	if p.preparedStmt {
		return fmt.Errorf("%s cannot be used in prepared statements", placeholder)
	}
	return nil
}

func (p *Translator) translate(sql string) (string, error) {
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
			p.dialect.EscapeIdent(b, col)
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
			if err := p.interpolate(b, sql[start:end], expand); err != nil {
				return "", err
			}
		default:
			b.WriteRune(r)
		}
	}
	if p.index < len(p.args) {
		return "", fmt.Errorf("only %d args are expected", p.index)
	}
	return b.String(), nil
}

func (p *Translator) nextArg() interface{} {
	if p.index >= len(p.args) {
		p.try(fmt.Errorf("there is not enough args for placeholders"))
		return nil
	}
	v := p.args[p.index]
	p.index++
	return v
}

func (p *Translator) nextParamNumber() int {
	p.param++
	return p.param
}

func (p *Translator) interpolate(b *bytes.Buffer, typ string, expand bool) error {
	if expand {
		switch typ {
		case "":
			p.try(p.checkInterpolationOf("?..."))
			p.try(p.escapeMultipleValues(b, p.nextArg()))
		case "ident":
			idents, ok := p.nextArg().([]string)
			if !ok {
				return fmt.Errorf("?ident... expects the argument to be a []string")
			} else if len(idents) == 0 {
				return fmt.Errorf("empty slice passed to ?ident...")
			}
			for i, ident := range idents {
				if i > 0 {
					b.WriteString(", ")
				}
				p.dialect.EscapeIdent(b, ident)
			}
		case "values":
			p.try(p.checkInterpolationOf("?values..."))
			p.try(p.printMultiValuesClause(b, p.nextArg()))
		default:
			return fmt.Errorf("?%s cannot be expanded (...) or doesn't exist", typ)
		}
	} else {
		switch typ {
		case "":
			if p.preparedStmt {
				p.dialect.PrintPlaceholderSign(b, p.nextParamNumber())
				return nil
			}
			p.try(p.escapeValue(b, p.nextArg()))
		case "ident":
			ident, ok := p.nextArg().(string)
			if !ok {
				return p.try(
					fmt.Errorf("?ident expects the argument to be a string"))
			}
			p.dialect.EscapeIdent(b, ident)
		case "values":
			p.try(p.checkInterpolationOf("?values"))
			p.try(p.printValuesClause(b, p.nextArg()))
		case "set":
			p.try(p.checkInterpolationOf("?set"))
			p.try(p.printSetClause(b, p.nextArg()))
		case "sql":
			switch arg := p.nextArg().(type) {
			case Marshaler:
				sql, err := arg.MarshalSQL(p.clone())
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
	return p.err
}

func (p *Translator) try(err error) error {
	if p.err == nil {
		p.err = err
	}
	return p.err
}

var timeType = reflect.TypeOf(time.Time{})

func (p *Translator) escapeValue(b *bytes.Buffer, v interface{}) error {
	vv := reflect.ValueOf(v)
	if valuer, ok := v.(driver.Valuer); ok {
		if vv.Kind() == reflect.Ptr && vv.IsNil() {
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
	case reflect.Ptr:
		if vv.IsNil() {
			b.WriteString("NULL")
			return nil
		}
		return p.escapeValue(b, vv.Elem().Interface())
	case reflect.Bool:
		p.dialect.EscapeBool(b, vv.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		formatInt(b, vv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		formatUint(b, vv.Uint())
	case reflect.Float32, reflect.Float64:
		formatFloat(b, vv.Float())
	case reflect.String:
		p.dialect.EscapeString(b, vv.String())
	case reflect.Slice:
		if vv.Type().Elem().Kind() == reflect.Uint8 {
			p.dialect.EscapeBytes(b, vv.Bytes())
			break
		}
		return fmt.Errorf("only a slice of bytes supported; got: %T", v)
	case reflect.Struct:
		if vv.Type() == timeType {
			p.dialect.EscapeTime(b, vv.Interface().(time.Time))
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

func (p *Translator) escapeMultipleValues(b *bytes.Buffer, v interface{}) error {
	vv := reflect.ValueOf(v)
	if vv.Kind() != reflect.Slice {
		return fmt.Errorf("?... expects the argument to be a slice")
	}
	length := vv.Len()
	if length == 0 {
		b.WriteString("NULL")
		return nil
	}
	for i := 0; i < length; i++ {
		if i > 0 {
			b.WriteString(", ")
		}
		if err := p.escapeValue(b, vv.Index(i).Interface()); err != nil {
			return err
		}
	}
	return nil
}

func (p *Translator) printValuesClause(b *bytes.Buffer, v interface{}) error {
	cols, vals, err := p.deriveColsAndVals(v)
	if err != nil {
		return err
	}
	b.WriteRune('(')
	for i, c := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		p.dialect.EscapeIdent(b, c)
	}
	b.WriteString(") VALUES (")
	for i, v := range vals {
		if i > 0 {
			b.WriteString(", ")
		}
		p.try(p.escapeValue(b, v))
	}
	b.WriteRune(')')
	return nil
}

func (p *Translator) printSetClause(b *bytes.Buffer, v interface{}) error {
	cols, vals, err := p.deriveColsAndVals(v)
	if err != nil {
		return err
	}
	b.WriteString("SET ")
	for i, c := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		v := vals[i]
		p.dialect.EscapeIdent(b, c)
		b.WriteString(" = ")
		p.try(p.escapeValue(b, v))
	}
	return nil
}

// deriveColsAndVals derives column names from an underlying type of v and returns
// them together with the corresponding values.
func (p *Translator) deriveColsAndVals(v interface{}) (cols []string, vals []interface{}, err error) {
	switch v := v.(type) {
	case Map:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, col := range keys {
			cols = append(cols, col)
			vals = append(vals, v[col])
		}
	default:
		vv := reflect.ValueOf(v)
		if vv.Kind() == reflect.Ptr {
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
	return
}

func (p *Translator) printMultiValuesClause(b *bytes.Buffer, v interface{}) error {
	errInvalidArg := fmt.Errorf("?values... expects the argument to be a slice of structs")
	vv := reflect.ValueOf(v)
	if vv.Kind() != reflect.Slice {
		return errInvalidArg
	}
	el := vv.Type().Elem()
	isPtr := false
	if el.Kind() == reflect.Ptr {
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
		p.dialect.EscapeIdent(b, c)
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
			p.try(p.escapeValue(b, v))
		}
		b.WriteRune(')')
		if i != length-1 {
			b.WriteRune(',')
		}
	}
	return nil
}

func errNoCols(v interface{}) error {
	return fmt.Errorf("no columns derived from %T", v)
}

func valuesByFieldIndexes(v reflect.Value, indexes [][]int) (vals []interface{}) {
	for _, index := range indexes {
		vals = append(vals, v.FieldByIndex(index).Interface())
	}
	return
}
