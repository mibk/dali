package dalivet

import (
	"reflect"
	"testing"
)

func TestParsePlaceholders(t *testing.T) {
	tests := []struct {
		query string
		want  []Placeholder
	}{
		{"SELECT 1", nil},
		{"SELECT ?", []Placeholder{{Type: ""}}},
		{"SELECT ?, ?", []Placeholder{{Type: ""}, {Type: ""}}},
		{"SELECT ?...", []Placeholder{{Type: "", Expand: true}}},
		{"INSERT INTO t ?values", []Placeholder{{Type: "values"}}},
		{"INSERT INTO t ?values...", []Placeholder{{Type: "values", Expand: true}}},
		{"UPDATE t ?set WHERE id = ?", []Placeholder{{Type: "set"}, {Type: ""}}},
		{"SELECT [col] FROM t WHERE id = ?", []Placeholder{{Type: ""}}},
		{"SELECT ?ident FROM t", []Placeholder{{Type: "ident"}}},
		{"SELECT ?ident... FROM t", []Placeholder{{Type: "ident", Expand: true}}},
		{"SELECT ?sql FROM t", []Placeholder{{Type: "sql"}}},
		// Unknown placeholder types are still parsed.
		{"SELECT ?foo FROM t", []Placeholder{{Type: "foo"}}},
		// Unterminated bracket — stops parsing.
		{"SELECT [broken", nil},
		// Placeholder at end of string.
		{"SELECT ?", []Placeholder{{Type: ""}}},
		// Mixed. Note: [?ident] is an identifier escape, not a placeholder.
		{
			"INSERT INTO [?ident] (?ident...) VALUES (?...) ON DUPLICATE KEY UPDATE ?set",
			[]Placeholder{
				{Type: "ident", Expand: true},
				{Type: "", Expand: true},
				{Type: "set"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got := parsePlaceholders(tt.query)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parsePlaceholders(%q):\n got %v\nwant %v", tt.query, got, tt.want)
			}
		})
	}
}
