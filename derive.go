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
	return colNamesAndFieldIndexesOfEmbedded(typ, []int{}, insert)
}

func colNamesAndFieldIndexesOfEmbedded(typ reflect.Type, index []int, insert bool) (cols []string, indexes [][]int) {
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
				emCols, emIndexes := colNamesAndFieldIndexesOfEmbedded(f.Type,
					append(index, i), insert)
				cols = append(cols, emCols...)
				indexes = append(indexes, emIndexes...)
				continue
			}
		}
		prop := parseFieldProp(f.Tag.Get("db"))
		if prop.Ignore || insert && prop.SelectOnly {
			continue
		}
		if prop.Col == "" {
			prop.Col = f.Name
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
	SelectOnly bool
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
		case "selectonly":
			p.SelectOnly = true
		}
	}
	return p
}
