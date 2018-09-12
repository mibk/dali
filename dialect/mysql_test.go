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
	tests := []struct {
		v   time.Time
		exp string
	}{
		{
			time.Date(2018, 9, 12, 8, 24, 37, 0, time.UTC),
			"'2018-09-12 08:24:37'",
		},
		{
			time.Date(2018, 9, 12, 9, 1, 2, 304000000, time.UTC),
			"'2018-09-12 09:01:02.304'",
		},
		{
			time.Date(2018, 9, 12, 9, 1, 2, 999999000, time.UTC),
			"'2018-09-12 09:01:02.999999'",
		},
		{
			time.Date(2018, 9, 12, 9, 1, 2, 1000, time.UTC),
			"'2018-09-12 09:01:02.000001'",
		},
		{
			time.Date(2018, 9, 12, 9, 1, 3, 900, time.UTC),
			"'2018-09-12 09:01:03'",
		},
	}
	for _, tt := range tests {
		b := new(bytes.Buffer)
		MySQL.EscapeTime(b, tt.v)
		if got := b.String(); got != tt.exp {
			t.Errorf("%v: got %v, want %v", tt.v, got, tt.exp)
		}
	}
}
