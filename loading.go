package dali

import (
	"database/sql"
	"reflect"
)

// One executes a query that returns rows and loads the resulting
// data from the first row into dest which must be a struct.
// Only fields that match the column names (after filtering
// through the mapperFunc) are filled. One returns sql.ErrNoRows
// if there are no rows.
func (q *Query) One(dest interface{}) error {
	destv := reflect.ValueOf(dest)
	if destv.Kind() != reflect.Ptr {
		panic("dali: dest must be a pointer to a struct")
	}
	v := reflect.Indirect(destv)
	if v.Kind() != reflect.Struct {
		panic("dali: dest must be a pointer to a struct")
	}
	return q.loadStruct(v)
}

// All executes a query that returns rows and loads the resulting
// data into dest which could be either a slice of structs, or
// a slice of primitive types.
//
// If the slice element is a primitive type, query is expected
// to return only 1 column for each row. dest is the filled
// with these values.
//
// If it is a struct, only fields that match the column names
// (after filtering through the mapperFunc) are filled.
func (q *Query) All(dest interface{}) error {
	destv := reflect.ValueOf(dest)
	if destv.Kind() != reflect.Ptr {
		panic("dali: dest must be a pointer to a slice")
	}
	slicev := reflect.Indirect(destv)
	if slicev.Kind() != reflect.Slice {
		panic("dali: dest must be a pointer to a slice")
	}

	elemt := slicev.Type().Elem()
	origint := elemt
	isPtr := false
	if isPtr = elemt.Kind() == reflect.Ptr; isPtr {
		elemt = elemt.Elem()
	}
	switch elemt.Kind() {
	case reflect.Ptr:
		panic("dali: a pointer to a pointer is not allowed as an element of a slice")
	case reflect.Struct:
		return q.loadStructs(slicev, elemt, isPtr)
	default:
		return q.loadValues(slicev, origint)
	}
}

func (q *Query) loadStruct(v reflect.Value) error { return q.load(v, v.Type(), true, false) }

func (q *Query) loadStructs(slicev reflect.Value, elemt reflect.Type, isPtr bool) error {
	return q.load(slicev, elemt, false, isPtr)
}

func (q *Query) load(v reflect.Value, elemt reflect.Type, loadJustOne, isPtr bool) error {
	rows, err := q.Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	rowCols, err := rows.Columns()
	if err != nil {
		return err
	}
	cols, indexes := q.preproc.colNamesAndFieldIndexes(elemt, false)
	fieldIndexes := make([][]int, len(rowCols))
	for coln, rowCol := range rowCols {
		var index []int
		for i, col := range cols {
			if rowCol == col {
				index = indexes[i]
				break
			}
		}
		fieldIndexes[coln] = index
	}
	fields := make([]interface{}, len(fieldIndexes))

	err = nil
	if loadJustOne {
		err = sql.ErrNoRows
	}
	for rows.Next() {
		elemvptr := reflect.New(elemt)
		elemv := reflect.Indirect(elemvptr)

		for i, index := range fieldIndexes {
			if index == nil {
				fields[i] = &ignoreField
				continue
			}
			fields[i] = elemv.FieldByIndex(index).Addr().Interface()
		}
		if err := rows.Scan(fields...); err != nil {
			return err
		}
		if loadJustOne {
			// v must is a struct.
			v.Set(elemv)
			err = nil
			break
			// Otherwise, v must be a slice.
		} else if isPtr {
			v.Set(reflect.Append(v, elemvptr))
		} else {
			v.Set(reflect.Append(v, elemv))
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return err
}

var ignoreField interface{}

func (q *Query) loadValues(slicev reflect.Value, elemt reflect.Type) error {
	rows, err := q.Rows()
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		elemvptr := reflect.New(elemt)
		if err := rows.Scan(elemvptr.Interface()); err != nil {
			return err
		}
		slicev.Set(reflect.Append(slicev, reflect.Indirect(elemvptr)))
	}
	return rows.Err()
}
