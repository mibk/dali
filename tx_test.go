package dali

import "testing"

var transactionTests = []struct {
	sql      string
	args     []any
	wantSQL  string
	prepared bool
}{
	{"SELECT name WHERE id = ?", Args{13}, "SELECT name WHERE id = 13", false},
	{"SELECT ?ident WHERE [id] = ?", Args{"name"}, "SELECT {name} WHERE {id} = &1", true},
}

func TestTransactions(t *testing.T) {
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range transactionTests {
		t.Run(tt.sql, func(t *testing.T) {
			var q *Query
			var gotErr error
			if tt.prepared {
				stmt, err := tx.Prepare(tt.sql, tt.args...)
				if err != nil {
					gotErr = err
				} else {
					q = stmt.Bind()
				}
			} else {
				q = tx.Query(tt.sql, tt.args...)
				if q.err != nil {
					gotErr = q.err
				}
			}
			if gotErr != nil {
				t.Fatalf("unexpected err: %v", gotErr)
			}
			if q.query != tt.wantSQL {
				t.Errorf("\n got: %v\nwant: %v", q.query, tt.wantSQL)
			}
		})
	}
}
