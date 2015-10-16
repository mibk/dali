package dali

import (
	"database/sql"
	"testing"
)

var (
	dvr  *FakeDriver
	conn *Connection
)

func init() {
	dvr = NewFakeDriver()
	sql.Register("dali", dvr)
	db, err := sql.Open("dali", "")
	if err != nil {
		panic(err)
	}
	conn = NewConnection(db, dvr)
}

func TestScanRow(t *testing.T) {
	var (
		id   int64
		name string
	)
	dvr.SetColumns("ID").SetResult(U{1, "John"})
	conn.Query("").ScanRow(&id)
	if id != 1 {
		t.Errorf("id: got %v, want %v", id, 1)
	}
	dvr.SetColumns("Name").SetResult(U{1, "John"})
	conn.Query("").ScanRow(&name)
	if name != "John" {
		t.Errorf("name: got %v, want %v", name, "John")
	}
}

type U struct {
	ID   int64
	Name string
}
