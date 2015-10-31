package dali

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mibk/dali/dialects"
)

// ErrNotUTF8 is returned when a string argument is not a valid UTF-8 string.
var ErrNotUTF8 = errors.New("dali: argument is not a valid UTF-8 string")

// Preprocessor processes SQL queries using a dialect.
type Preprocessor struct {
	dialect    dialects.Dialect
	mapperFunc func(string) string
}

// NewPreprocessor creates a new Preprocessor.
func NewPreprocessor(dialect dialects.Dialect) *Preprocessor {
	return &Preprocessor{dialect, ToUnderscore}
}

// Process processes the sql and the args. It returns the resulting SQL query and
// an error if there is one.
func (p *Preprocessor) Process(sql string, args []interface{}) (string, error) {
	pp := &preprocessor{Preprocessor: p, args: args}
	return pp.process(sql)
}

func (p *Preprocessor) ProcessPreparedStmt(sql string, args []interface{}) (string, error) {
	pp := &preprocessor{Preprocessor: p, args: args, isPreparedStmt: true}
	return pp.process(sql)
}

// preprocessor represents a single preprocessing.
type preprocessor struct {
	*Preprocessor
	isPreparedStmt bool
	err            error
	args           []interface{}
	// Index of the current arg.
	index int
	// Holds the placeholder index.
	param int
}

func (p *preprocessor) checkInterpolationOf(placeholder string) error {
	if p.isPreparedStmt {
		return fmt.Errorf("dali: %s cannot be used in prepared statements", placeholder)
	}
	return nil
}

func (p *preprocessor) process(sql string) (string, error) {
	b := new(bytes.Buffer)
	pos := 0
	for pos < len(sql) {
		r, w := utf8.DecodeRuneInString(sql[pos:])
		pos += w

		switch r {
		case '[':
			w := strings.IndexRune(sql[pos:], ']')
			if w == -1 {
				return "", fmt.Errorf("dali: identifier not terminated")
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
		return "", fmt.Errorf("dali: only %d args are expected", p.index)
	}
	return b.String(), nil
}

func (p *preprocessor) nextArg() interface{} {
	if p.index >= len(p.args) {
		p.try(fmt.Errorf("dali: there is not enough args for placeholders"))
		return nil
	}
	v := p.args[p.index]
	p.index++
	return v
}

func (p *preprocessor) nextParamNumber() int {
	p.param++
	return p.param
}

func (p *preprocessor) interpolate(b *bytes.Buffer, typ string, expand bool) error {
	if expand {
		switch typ {
		case "":
			p.try(p.checkInterpolationOf("?..."))
			p.try(p.escapeMultipleValues(b, p.nextArg()))
		case "ident":
			idents, ok := p.nextArg().([]string)
			if !ok {
				return fmt.Errorf("dali: ?ident... expects the argument to be a []string")
			}
			for i, ident := range idents {
				p.dialect.EscapeIdent(b, ident)
				if i != len(idents)-1 {
					b.WriteString(", ")
				}
			}
		case "values":
			p.try(p.checkInterpolationOf("?values..."))
			p.try(p.printMultiValuesClause(b, p.nextArg()))
		default:
			return fmt.Errorf("dali: ?%s cannot be expanded (...) or doesn't exist", typ)
		}
	} else {
		switch typ {
		case "":
			if p.isPreparedStmt {
				p.dialect.PrintPlaceholderSign(b, p.nextParamNumber())
				return nil
			}
			p.try(p.escapeValue(b, p.nextArg()))
		case "ident":
			ident, ok := p.nextArg().(string)
			if !ok {
				return p.try(
					fmt.Errorf("dali: ?ident expects the argument to be a string"))
			}
			p.dialect.EscapeIdent(b, ident)
		case "values":
			p.try(p.checkInterpolationOf("?values"))
			p.try(p.printValuesClause(b, p.nextArg()))
		case "set":
			p.try(p.checkInterpolationOf("?set"))
			p.try(p.printSetClause(b, p.nextArg()))
		case "raw":
			raw, ok := p.nextArg().(string)
			if !ok {
				return p.try(fmt.Errorf("dali: ?raw expects the argument to be a string"))
			}
			b.WriteString(raw)
		default:
			return fmt.Errorf("dali: unknown placeholder ?%s", typ)
		}
	}
	return p.err
}

func (p *preprocessor) try(err error) error {
	if p.err == nil {
		p.err = err
	}
	return p.err
}

func (p *Preprocessor) escapeValue(b *bytes.Buffer, v interface{}) error {
	if valuer, ok := v.(driver.Valuer); ok {
		var err error
		if v, err = valuer.Value(); err != nil {
			return err
		}
	}
	if v == nil {
		b.WriteString("NULL")
		return nil
	}
	switch v := v.(type) {
	case bool:
		p.dialect.EscapeBool(b, v)

	// signed integers
	case int:
		formatInt(b, int64(v))
	case int8:
		formatInt(b, int64(v))
	case int16:
		formatInt(b, int64(v))
	case int32:
		formatInt(b, int64(v))
	case int64:
		formatInt(b, v)

	// unsigned integers
	case uint:
		formatUint(b, uint64(v))
	case uint8:
		formatUint(b, uint64(v))
	case uint16:
		formatUint(b, uint64(v))
	case uint32:
		formatUint(b, uint64(v))
	case uint64:
		formatUint(b, v)

	// floats
	case float32:
		formatFloat(b, float64(v))
	case float64:
		formatFloat(b, v)

	case string:
		if !utf8.ValidString(v) {
			return ErrNotUTF8
		}
		p.dialect.EscapeString(b, v)

	case []byte:
		p.dialect.EscapeBytes(b, v)

	case time.Time:
		p.dialect.EscapeTime(b, v)
	default:
		return fmt.Errorf("dali: invalid argument type: %T", v)
	}
	return nil
}

func formatInt(b *bytes.Buffer, i int64)     { b.WriteString(strconv.FormatInt(i, 10)) }
func formatUint(b *bytes.Buffer, u uint64)   { b.WriteString(strconv.FormatUint(u, 10)) }
func formatFloat(b *bytes.Buffer, f float64) { b.WriteString(strconv.FormatFloat(f, 'f', -1, 64)) }

func (p *Preprocessor) escapeMultipleValues(b *bytes.Buffer, v interface{}) error {
	vv := reflect.ValueOf(v)
	if vv.Kind() != reflect.Slice {
		return fmt.Errorf("dali: ?... expects the argument to be a slice")
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

// Map is just an alias for map[string]interface{}.
type Map map[string]interface{}

func (p *Preprocessor) printValuesClause(b *bytes.Buffer, v interface{}) error {
	cols, vals, err := p.deriveColsAndVals(v)
	if err != nil {
		return err
	}
	b.WriteRune('(')
	for i, c := range cols {
		p.dialect.EscapeIdent(b, c)
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
	cols, vals, err := p.deriveColsAndVals(v)
	if err != nil {
		return err
	}
	b.WriteString("SET ")
	for i, c := range cols {
		v := vals[i]
		p.dialect.EscapeIdent(b, c)
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
func (p *Preprocessor) deriveColsAndVals(v interface{}) (cols []string, vals []interface{}, err error) {
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
			return nil, nil, fmt.Errorf("dali: argument must be a pointer to a struct")
		}
		var indexes [][]int
		cols, indexes = p.colNamesAndFieldIndexes(vv.Type(), true)
		vals = valuesByFieldIndexes(vv, indexes)

	}
	return
}

func (p *Preprocessor) printMultiValuesClause(b *bytes.Buffer, v interface{}) error {
	errInvalidArg := fmt.Errorf("dali: ?values... expects the argument to be a slice of structs")
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
	cols, indexes := p.colNamesAndFieldIndexes(el, true)
	b.WriteRune('(')
	for i, c := range cols {
		p.dialect.EscapeIdent(b, c)
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
// them together with the indexes of used fields. typ must be a struct type.
// If the tag name equals "-", the field is ignored. If insert is true,
// fields having the omitinsert property are ignored as well.
func (p *Preprocessor) colNamesAndFieldIndexes(typ reflect.Type, insert bool) (
	cols []string, indexes [][]int) {
	return p.colNamesAndFieldIndexesOfNested(typ, []int{}, insert)
}

func (p *Preprocessor) colNamesAndFieldIndexesOfNested(typ reflect.Type, index []int, insert bool) (
	cols []string, indexes [][]int) {

	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.PkgPath != "" { // Is unexported?
			continue
		}
		if f.Type.Kind() == reflect.Struct {
			switch {
			case f.Type.ConvertibleTo(reflect.TypeOf(time.Time{})):
				break
			case insert && f.Type.Implements(valuerInterface):
				break
			case !insert && (f.Type.Implements(scannerInterface) ||
				reflect.PtrTo(f.Type).Implements(scannerInterface)):
				break
			default:
				emCols, emIndexes := p.colNamesAndFieldIndexesOfNested(f.Type,
					append(index, i), insert)
				cols = append(cols, emCols...)
				indexes = append(indexes, emIndexes...)
				continue
			}
		}
		prop := parseFieldProp(f.Tag.Get("db"))
		if prop.Ignore || insert && prop.OmitInsert {
			continue
		}
		if prop.Col == "" {
			prop.Col = p.mapperFunc(f.Name)
		}
		cols = append(cols, prop.Col)
		indexes = append(indexes, append(index, i))
	}
	return
}

var (
	valuerInterface  = reflect.TypeOf((*driver.Valuer)(nil)).Elem()
	scannerInterface = reflect.TypeOf((*sql.Scanner)(nil)).Elem()
)

type fieldProp struct {
	Col        string
	OmitInsert bool
	Ignore     bool
}

func parseFieldProp(s string) fieldProp {
	props := strings.Split(s, ",")
	if props[0] == "-" {
		return fieldProp{Ignore: true}
	}
	p := fieldProp{Col: props[0]}
	for _, prop := range props[1:] {
		switch prop {
		case "omitinsert":
			p.OmitInsert = true
		}
	}
	return p
}

func valuesByFieldIndexes(v reflect.Value, indexes [][]int) (vals []interface{}) {
	for _, index := range indexes {
		vals = append(vals, v.FieldByIndex(index).Interface())
	}
	return
}

func (p *Preprocessor) setMapperFunc(f func(string) string) {
	p.mapperFunc = f
}
