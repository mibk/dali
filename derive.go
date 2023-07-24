package dali

import (
	"database/sql"
	"database/sql/driver"
	"reflect"
	"strings"
	"time"
)

// colNamesAndFieldIndexes derives column names from a struct type and returns
// them together with the indexes of used fields. typ must be a struct type.
// If the tag name equals "-", the field is ignored. If insert is true,
// fields having the selectonly property are ignored as well.
func colNamesAndFieldIndexes(typ reflect.Type, insert bool) (cols []string, indexes [][]int) {
	return colNamesAndFieldIndexesBase(nil, typ, insert)
}

func colNamesAndFieldIndexesBase(baseIndex []int, typ reflect.Type, insert bool) (cols []string, indexes [][]int) {
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)

		if f.Type.Kind() == reflect.Struct {
			switch {
			case f.Type == reflect.TypeOf(time.Time{}):
			case insert && f.Type.Implements(valuerInterface):
			case !insert && (f.Type.Implements(scannerInterface) || reflect.PtrTo(f.Type).Implements(scannerInterface)):
				// Known struct.

			case f.Anonymous, f.IsExported():
				emCols, emIndexes := colNamesAndFieldIndexesBase(append(baseIndex, i), f.Type, insert)
				cols = append(cols, emCols...)
				indexes = append(indexes, emIndexes...)
				continue
			}
		}

		if !f.IsExported() {
			continue
		}

		prop := parseFieldProp(f.Tag.Get("db"))
		if prop.Ignore || insert && prop.SelectOnly {
			continue
		}
		if prop.ColName == "" {
			prop.ColName = f.Name
		}
		cols = append(cols, prop.ColName)
		indexes = append(indexes, append(baseIndex, i))
	}
	return
}

var (
	valuerInterface  = reflect.TypeOf((*driver.Valuer)(nil)).Elem()
	scannerInterface = reflect.TypeOf((*sql.Scanner)(nil)).Elem()
)

type fieldProps struct {
	ColName    string
	SelectOnly bool
	Ignore     bool
}

func parseFieldProp(s string) fieldProps {
	props := strings.Split(s, ",")
	if props[0] == "-" {
		return fieldProps{Ignore: true}
	}
	p := fieldProps{ColName: props[0]}
	for _, prop := range props[1:] {
		switch prop {
		case "selectonly":
			p.SelectOnly = true
		}
	}
	return p
}
