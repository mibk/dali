package dalivet

import "unicode/utf8"

// Placeholder represents a parsed dali placeholder from a query string.
type Placeholder struct {
	Type   string // e.g. "", "ident", "values", "set", "sql"
	Expand bool   // true when followed by "..."
}

// parsePlaceholders extracts all ?-placeholders from a dali query string.
// It mirrors the parsing loop in translate.go.
func parsePlaceholders(query string) []Placeholder {
	var phs []Placeholder
	pos := 0
	for pos < len(query) {
		r, w := utf8.DecodeRuneInString(query[pos:])
		pos += w

		switch r {
		case '[':
			// Skip identifier escape.
			w := indexRune(query[pos:], ']')
			if w == -1 {
				return phs
			}
			pos += w + 1
		case '?':
			start, end := pos, pos
			var expand bool
			for pos < len(query) {
				r, w := utf8.DecodeRuneInString(query[pos:])
				if r < 'a' || r > 'z' {
					if len(query[pos:]) >= 3 && query[pos:pos+3] == "..." {
						pos += 3
						expand = true
					}
					break
				}
				pos += w
				end = pos
			}
			phs = append(phs, Placeholder{
				Type:   query[start:end],
				Expand: expand,
			})
		}
	}
	return phs
}

func indexRune(s string, r rune) int {
	for i := 0; i < len(s); {
		c, w := utf8.DecodeRuneInString(s[i:])
		if c == r {
			return i
		}
		i += w
	}
	return -1
}
