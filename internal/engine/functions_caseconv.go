package engine

import (
	"fmt"
	"strings"
	"unicode"
)

func registerCaseConvFuncs(r *FuncRegistry) {
	r.Register("camelcase", fnCamelCase)
	r.Register("kebabcase", fnKebabCase)
	r.Register("snakecase", fnSnakeCase)
	r.Register("swapcase", fnSwapCase)
	r.Register("initials", fnInitials)
}

func splitWords(s string) []string {
	out := make([]string, 0, 4)
	var cur strings.Builder
	prevIsLower := false
	for _, r := range s {
		switch {
		case unicode.IsSpace(r), '_' == r, '-' == r, '.' == r:
			if 0 < cur.Len() {
				out = append(out, cur.String())
				cur.Reset()
			}
			prevIsLower = false
		case unicode.IsUpper(r):
			if prevIsLower && 0 < cur.Len() {
				out = append(out, cur.String())
				cur.Reset()
			}
			cur.WriteRune(r)
			prevIsLower = false
		default:
			cur.WriteRune(r)
			prevIsLower = unicode.IsLower(r)
		}
	}
	if 0 < cur.Len() {
		out = append(out, cur.String())
	}
	return out
}

func fnCamelCase(value any, args ...any) (any, error) {
	words := splitWords(fmt.Sprintf("%v", value))
	for i, w := range words {
		if 0 == len(w) {
			continue
		}
		if 0 == i {
			words[i] = strings.ToLower(w)
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
	}
	return strings.Join(words, ""), nil
}

func fnKebabCase(value any, args ...any) (any, error) {
	words := splitWords(fmt.Sprintf("%v", value))
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	return strings.Join(words, "-"), nil
}

func fnSnakeCase(value any, args ...any) (any, error) {
	words := splitWords(fmt.Sprintf("%v", value))
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	return strings.Join(words, "_"), nil
}

func fnSwapCase(value any, args ...any) (any, error) {
	s := fmt.Sprintf("%v", value)
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case unicode.IsUpper(r):
			out = append(out, unicode.ToLower(r))
		case unicode.IsLower(r):
			out = append(out, unicode.ToUpper(r))
		default:
			out = append(out, r)
		}
	}
	return string(out), nil
}

func fnInitials(value any, args ...any) (any, error) {
	s := fmt.Sprintf("%v", value)
	out := make([]rune, 0, 4)
	for _, w := range strings.Fields(s) {
		if 0 < len(w) {
			out = append(out, []rune(w)[0])
		}
	}
	return string(out), nil
}
