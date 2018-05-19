package dialect

import (
	"bytes"
	"testing"
	"time"
)

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
		MySQL.EscapeIdent(b, tt.v)
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
		MySQL.EscapeBool(b, tt.v)
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
		{"\x00'\"\b\n\r", `'\0\'\"\b\n\r'`},
		{"\t\x1A\\", `'\t\Z\\'`},
		{"příliš žluťoučký kůň úpěl ďábelské ódy", "'příliš žluťoučký kůň úpěl ďábelské ódy'"},
	}
	for _, tt := range tests {
		b := new(bytes.Buffer)
		MySQL.EscapeString(b, tt.v)
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
		MySQL.EscapeBytes(b, tt.v)
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
		MySQL.EscapeTime(b, tt.v)
		if got := b.String(); got != tt.exp {
			t.Errorf("got %v, want %v", got, tt.exp)
		}
	}
}
