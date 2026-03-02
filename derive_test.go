package dali

import (
	"reflect"
	"testing"
)

type BenchStruct struct {
	ID      int64  `db:"id,selectonly"`
	Name    string `db:"name"`
	Email   string `db:"email"`
	GroupID int64  `db:"group_id"`
	Ignore  int    `db:"-"`
}

type BenchEmbedded struct {
	ID int64 `db:"id"`
	Name
	Email string `db:"email"`
}

func BenchmarkColNamesAndFieldIndexes(b *testing.B) {
	typ := reflect.TypeFor[BenchStruct]()
	for b.Loop() {
		colNamesAndFieldIndexes(typ, true)
	}
}

func BenchmarkColNamesAndFieldIndexes_select(b *testing.B) {
	typ := reflect.TypeFor[BenchStruct]()
	for b.Loop() {
		colNamesAndFieldIndexes(typ, false)
	}
}

func BenchmarkColNamesAndFieldIndexes_embedded(b *testing.B) {
	typ := reflect.TypeFor[BenchEmbedded]()
	for b.Loop() {
		colNamesAndFieldIndexes(typ, true)
	}
}
