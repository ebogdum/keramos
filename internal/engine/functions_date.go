package engine

import (
	"fmt"
	"strings"
	"time"

	keramoserrors "github.com/ebogdum/keramos/internal/errors"
)

func registerDateFuncs(r *FuncRegistry) {
	r.Register("now", fnNow)
	r.Register("date", fnDate)
	r.Register("dateInZone", fnDateInZone)
	r.Register("toDate", fnToDate)
	r.Register("ago", fnAgo)
}

// strftimeMap converts strftime tokens to Go reference-layout fragments.
// Subset of glibc strftime; uncommon tokens pass through verbatim.
var strftimeMap = map[byte]string{
	'Y': "2006", 'y': "06", 'C': "20",
	'm': "01", 'B': "January", 'b': "Jan", 'h': "Jan",
	'd': "02", 'e': "_2", 'j': "002",
	'a': "Mon", 'A': "Monday",
	'H': "15", 'I': "03",
	'M': "04", 'S': "05",
	'p': "PM", 'P': "pm",
	'Z': "MST", 'z': "-0700",
	'n': "\n", 't': "\t",
}

// goLayout converts a strftime-style format string to Go's reference layout.
// Strings without `%` pass through unchanged so callers can keep using Go
// reference layouts directly (e.g. "2006-01-02").
func goLayout(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if '%' != c || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}
		tok := s[i+1]
		if '%' == tok {
			b.WriteByte('%')
			i++
			continue
		}
		if mapped, ok := strftimeMap[tok]; ok {
			b.WriteString(mapped)
			i++
			continue
		}
		b.WriteByte('%')
		b.WriteByte(tok)
		i++
	}
	return b.String()
}

func fnNow(value any, args ...any) (any, error) {
	return time.Now(), nil
}

func fnDate(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "date requires a format argument")
	}
	t, err := coerceTime(value)
	if nil != err {
		return nil, err
	}
	return t.Format(goLayout(coerceString(args[0]))), nil
}

func fnDateInZone(value any, args ...any) (any, error) {
	if 2 > len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "dateInZone requires format and zone arguments")
	}
	t, err := coerceTime(value)
	if nil != err {
		return nil, err
	}
	loc, locErr := time.LoadLocation(coerceString(args[1]))
	if nil != locErr {
		return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, locErr, "dateInZone: invalid zone %q", args[1])
	}
	return t.In(loc).Format(goLayout(coerceString(args[0]))), nil
}

func fnToDate(value any, args ...any) (any, error) {
	if 0 == len(args) {
		return nil, keramoserrors.NewError(keramoserrors.ErrFunction, "toDate requires a format argument")
	}
	s, ok := value.(string)
	if !ok {
		s = fmt.Sprintf("%v", value)
	}
	t, parseErr := time.Parse(goLayout(coerceString(args[0])), s)
	if nil != parseErr {
		return nil, keramoserrors.WrapErrorf(keramoserrors.ErrFunction, parseErr, "toDate: failed to parse %q", s)
	}
	return t, nil
}

func fnAgo(value any, args ...any) (any, error) {
	t, err := coerceTime(value)
	if nil != err {
		return nil, err
	}
	return time.Since(t).String(), nil
}

func coerceTime(value any) (time.Time, error) {
	switch v := value.(type) {
	case time.Time:
		return v, nil
	case string:
		// Try common formats.
		for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02T15:04:05Z", "2006-01-02"} {
			if t, err := time.Parse(layout, v); nil == err {
				return t, nil
			}
		}
	}
	return time.Time{}, keramoserrors.NewErrorf(keramoserrors.ErrFunction, "expected time.Time or RFC3339 string, got %T", value)
}
