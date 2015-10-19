package drivers

import (
	"bytes"
	"testing"
	"time"
)

var d = MySQL{}

func TestEscapeIdent(t *testing.T) {
	tests := []struct {
		v   string
		exp string
	}{
		{"basic*name", "`basic*name`"},
		{"some`name", "`some``name`"},
	}
	for _, tt := range tests {
		b := new(bytes.Buffer)
		d.EscapeIdent(b, tt.v)
		if got := b.String(); got != tt.exp {
			t.Errorf("got %v, want %v", got, tt.exp)
		}
	}
}

func TestEscapeBool(t *testing.T) {
	tests := []struct {
		v   bool
		exp string
	}{
		{false, "0"},
		{true, "1"},
	}
	for _, tt := range tests {
		b := new(bytes.Buffer)
		d.EscapeBool(b, tt.v)
		if got := b.String(); got != tt.exp {
			t.Errorf("got %v, want %v", got, tt.exp)
		}
	}
}

func TestEscapeString(t *testing.T) {
	tests := []struct {
		v   string
		exp string
	}{
		{"simple", "'simple'"},
		{`simplers's "world"`, `'simplers\'s \"world\"'`},
		{"\n\r\x00\x1A", `'\n\r\x00\x1a'`},
	}
	for _, tt := range tests {
		b := new(bytes.Buffer)
		d.EscapeString(b, tt.v)
		if got := b.String(); got != tt.exp {
			t.Errorf("got %v, want %v", got, tt.exp)
		}
	}
}

func TestEscapeBytes(t *testing.T) {
	tests := []struct {
		v   []byte
		exp string
	}{
		{[]byte("a slice"), "_binary'a slice'"},
	}
	for _, tt := range tests {
		b := new(bytes.Buffer)
		d.EscapeBytes(b, tt.v)
		if got := b.String(); got != tt.exp {
			t.Errorf("got %v, want %v", got, tt.exp)
		}
	}
}

func TestEscapeTime(t *testing.T) {
	tim, _ := time.Parse(time.Kitchen, "3:40AM")
	tests := []struct {
		v   time.Time
		exp string
	}{
		{tim, "'" + tim.Format(mysqlTimeFormat) + "'"},
	}
	for _, tt := range tests {
		b := new(bytes.Buffer)
		d.EscapeTime(b, tt.v)
		if got := b.String(); got != tt.exp {
			t.Errorf("got %v, want %v", got, tt.exp)
		}
	}
}
