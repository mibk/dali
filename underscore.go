package dali

import "unicode"

// ToUnderscore transforms CamelCASEString to camel_case_string.
func ToUnderscore(name string) string {
	var underscore []rune
	prevIsUpper := true
	runes := []rune(name)
	for i, r := range runes {
		switch {
		case unicode.IsUpper(r):
			if !prevIsUpper {
				underscore = append(underscore, '_')
			}
			r = unicode.ToLower(r)
			fallthrough
		case r == '_':
			prevIsUpper = true
		default:
			if prevIsUpper && i >= 2 && unicode.IsUpper(runes[i-2]) {
				l := len(underscore)
				underscore = append(underscore[:l-1], '_', underscore[l-1])
			}
			prevIsUpper = false
		}
		underscore = append(underscore, r)
	}
	return string(underscore)
}
