package dali

import (
	"database/sql"
	"fmt"
	"reflect"
)

// One executes the query that should return rows and loads the
// resulting data from the first row into dest which must be a struct.
// Only fields that match the column names (after filtering through
// the mapperFunc) are filled. One returns sql.ErrNoRows if there are
// no rows.
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

// All executes the query that should return rows, and loads the
// resulting data into dest which must be a slice of structs.
// Only fields that match the column names (after filtering through
// the mapperFunc) are filled.
func (q *Query) All(dest interface{}) error {
	const errMsg = "dali: dest must be a pointer to a slice of structs or pointers to structs"
	destv := reflect.ValueOf(dest)
	if destv.Kind() != reflect.Ptr {
		panic(errMsg)
	}
	slicev := reflect.Indirect(destv)
	if slicev.Kind() != reflect.Slice {
		panic(errMsg)
	}

	elemt := slicev.Type().Elem()
	isPtr := false
	if isPtr = elemt.Kind() == reflect.Ptr; isPtr {
		elemt = elemt.Elem()
	}
	switch elemt.Kind() {
	case reflect.Ptr:
		panic("dali: a pointer to a pointer is not allowed as an element of dest")
	case reflect.Struct:
		return q.loadStructs(slicev, elemt, isPtr)
	}
	panic(errMsg)
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
	cols, indexes := colNamesAndFieldIndexes(elemt, false)
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

		noMatch := true
		for i, index := range fieldIndexes {
			if index == nil {
				fields[i] = new(interface{})
				continue
			}
			noMatch = false
			fields[i] = elemv.FieldByIndex(index).Addr().Interface()
		}
		if noMatch {
			return fmt.Errorf("dali: no match between columns and struct fields")
		}
		if err := rows.Scan(fields...); err != nil {
			return err
		}
		if loadJustOne {
			// v is a struct.
			v.Set(elemv)
			err = nil
			break
		}

		// Otherwise, v must be a slice.
		if isPtr {
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

// ScanAllRows executes the query that is expected to return rows.
// It copies the columns from the matched rows into the slices
// pointed at by dests.
func (q *Query) ScanAllRows(dests ...interface{}) error {
	slicevals := make([]reflect.Value, len(dests))
	elemtypes := make([]reflect.Type, len(dests))
	for i, dests := range dests {
		destv := reflect.ValueOf(dests)
		if destv.Kind() != reflect.Ptr {
			panic("dali: dests must be a pointer to a slice")
		}
		slicevals[i] = reflect.Indirect(destv)
		if slicevals[i].Kind() != reflect.Slice {
			panic("dali: dests must be a pointer to a slice")
		}
		elemtypes[i] = slicevals[i].Type().Elem()
	}
	rows, err := q.Rows()
	if err != nil {
		return err
	}
	defer rows.Close()
	elemvptrs := make([]reflect.Value, len(dests))
	args := make([]interface{}, len(dests))
	for rows.Next() {
		for i := range args {
			elemvptrs[i] = reflect.New(elemtypes[i])
			args[i] = elemvptrs[i].Interface()
		}
		if err := rows.Scan(args...); err != nil {
			return err
		}
		for i := range args {
			slicevals[i].Set(reflect.Append(slicevals[i], reflect.Indirect(elemvptrs[i])))
		}
	}
	return rows.Err()
}
