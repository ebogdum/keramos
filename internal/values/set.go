package values

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"

	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
)

// maxSetIndex bounds --set array indexes so a single flag cannot grow a slice
// to an arbitrary length and OOM the process. 65536 is far beyond any real
// values array while capping the worst-case allocation.
const maxSetIndex = 1 << 16

// splitDotPath splits a dotted key path, honoring `\.` as an escape for a
// literal dot inside a path segment. Bracket-style array indexing is folded
// into dotted form AFTER the empty-segment check: `images[0].repo` becomes
// ["images","0","repo"]. Real empty segments (leading/trailing dot,
// consecutive dots in the original key) still error.
func splitDotPath(key string) ([]string, error) {
	parts := make([]string, 0, 4)
	var cur strings.Builder
	keyLen := len(key)
	for i := 0; i < keyLen; i++ {
		c := key[i]
		if '\\' == c && i+1 < keyLen && '.' == key[i+1] {
			cur.WriteByte('.')
			i++
			continue
		}
		if '.' == c {
			parts = append(parts, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	parts = append(parts, cur.String())
	for _, part := range parts {
		if "" == part {
			return nil, keramoserr.NewError(keramoserr.ErrCLIFlag, "invalid --set key: empty segment in path: "+key)
		}
	}
	// Now fold bracket form `images[0]` → `images`, `0` (two segments).
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		expanded := strings.NewReplacer("[", ".", "]", "").Replace(p)
		for _, sub := range strings.Split(expanded, ".") {
			if "" == sub {
				continue // strictly from bracket close
			}
			out = append(out, sub)
		}
	}
	if 0 == len(out) {
		return nil, keramoserr.NewError(keramoserr.ErrCLIFlag, "invalid --set key: empty path: "+key)
	}
	return out, nil
}

// ParseSet parses a --set argument "key=value" and applies it to a values map.
// Supports dotted paths: "image.tag=v1.0" sets values["image"]["tag"] = "v1.0".
// Use `\.` to embed a literal dot in a key segment.
// Supports type inference: numbers, booleans, null are auto-detected.
func ParseSet(values map[string]any, raw string) error {
	key, val, found := strings.Cut(raw, "=")
	if !found {
		return keramoserr.NewErrorf(keramoserr.ErrCLIFlag, "invalid --set format, expected key=value: %s", raw)
	}
	if "" == key {
		return keramoserr.NewError(keramoserr.ErrCLIFlag, "empty key in --set argument")
	}
	parts, err := splitDotPath(key)
	if nil != err {
		return err
	}
	return setNestedValue(values, parts, inferType(val))
}

// ParseSetString parses a --set-string argument, forcing string interpretation
// (no type inference). Used for values where `--set` would otherwise parse
// the input as an int, float, or bool.
func ParseSetString(values map[string]any, raw string) error {
	key, val, found := strings.Cut(raw, "=")
	if !found {
		return keramoserr.NewErrorf(keramoserr.ErrCLIFlag, "invalid --set-string format, expected key=value: %s", raw)
	}
	if "" == key {
		return keramoserr.NewError(keramoserr.ErrCLIFlag, "empty key in --set-string argument")
	}
	parts, err := splitDotPath(key)
	if nil != err {
		return err
	}
	return setNestedValue(values, parts, val)
}

// ParseSetFile parses --set-file: reads the value from a file and assigns
// the file contents (as string) to the key.
func ParseSetFile(values map[string]any, raw string) error {
	key, path, found := strings.Cut(raw, "=")
	if !found {
		return keramoserr.NewErrorf(keramoserr.ErrCLIFlag, "invalid --set-file format, expected key=path: %s", raw)
	}
	if "" == key {
		return keramoserr.NewError(keramoserr.ErrCLIFlag, "empty key in --set-file argument")
	}
	data, readErr := os.ReadFile(path)
	if nil != readErr {
		return keramoserr.WrapErrorf(keramoserr.ErrCLIFlag, readErr, "failed to read --set-file source: %s", path)
	}
	parts, err := splitDotPath(key)
	if nil != err {
		return err
	}
	return setNestedValue(values, parts, string(data))
}

// ParseSetJSON parses --set-json: the value is parsed as a JSON literal
// (object/array/scalar), allowing complex structures inline.
func ParseSetJSON(values map[string]any, raw string) error {
	key, val, found := strings.Cut(raw, "=")
	if !found {
		return keramoserr.NewErrorf(keramoserr.ErrCLIFlag, "invalid --set-json format, expected key=value: %s", raw)
	}
	if "" == key {
		return keramoserr.NewError(keramoserr.ErrCLIFlag, "empty key in --set-json argument")
	}
	var parsed any
	if jsonErr := json.Unmarshal([]byte(val), &parsed); nil != jsonErr {
		return keramoserr.WrapErrorf(keramoserr.ErrCLIFlag, jsonErr, "invalid JSON in --set-json value: %s", val)
	}
	parts, err := splitDotPath(key)
	if nil != err {
		return err
	}
	return setNestedValue(values, parts, parsed)
}

// setNestedValue writes `value` at the dotted path `parts` inside `m`,
// creating intermediate maps and slices as required. A numeric segment that
// directly follows a slice container indexes into the slice; a numeric
// segment that follows a map container creates a slice big enough to hold
// the index. Out-of-bounds slice writes grow the slice with nil holes.
func setNestedValue(m map[string]any, parts []string, value any) error {
	if 0 == len(parts) {
		return keramoserr.NewError(keramoserr.ErrCLIFlag, "empty path")
	}
	return setRecursive(m, parts, value)
}

func setRecursive(container any, parts []string, value any) error {
	if 0 == len(parts) {
		return nil
	}
	head := parts[0]
	last := 1 == len(parts)

	switch host := container.(type) {
	case map[string]any:
		if last {
			host[head] = value
			return nil
		}
		// Decide whether the next segment is a numeric index (=> slice
		// container) or a string key (=> nested map). Look at parts[1].
		next := parts[1]
		if isInt(next) {
			existing, ok := host[head].([]any)
			if !ok {
				existing = []any{}
			}
			updated, err := setSlice(existing, parts[1:], value)
			if nil != err {
				return err
			}
			host[head] = updated
			return nil
		}
		nextMap, ok := host[head].(map[string]any)
		if !ok {
			nextMap = map[string]any{}
			host[head] = nextMap
		}
		return setRecursive(nextMap, parts[1:], value)
	case []any:
		// Numeric index into the slice
		idx, err := strconv.Atoi(head)
		if nil != err {
			return keramoserr.NewErrorf(keramoserr.ErrCLIFlag,
				"slice container expects numeric index, got %q", head)
		}
		_ = idx
		// Should not be reached: callers route through setSlice for slice
		// containers because we need to mutate the slice header.
		return keramoserr.NewError(keramoserr.ErrCLIFlag, "internal: slice container hit setRecursive directly")
	}
	return keramoserr.NewErrorf(keramoserr.ErrCLIFlag,
		"cannot index into %T at %q", container, head)
}

// setSlice grows `s` to fit the leading numeric index in `parts`, recurses
// into the indexed element if more parts remain, and returns the (possibly
// reallocated) slice. The map at `path` in the caller must be reassigned.
func setSlice(s []any, parts []string, value any) ([]any, error) {
	if 0 == len(parts) {
		return s, nil
	}
	idx, err := strconv.Atoi(parts[0])
	if nil != err {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIFlag,
			"expected numeric index, got %q", parts[0])
	}
	// Bound the index: negative indexes are invalid (a bare strconv.Atoi
	// accepts "-1", which would panic on the assignment below), and an
	// unbounded positive index would grow the slice to idx+1 entries and
	// OOM the process from a single --set flag.
	if idx < 0 {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIFlag,
			"array index %d is negative", idx)
	}
	if idx > maxSetIndex {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIFlag,
			"array index %d exceeds the maximum of %d", idx, maxSetIndex)
	}
	for len(s) <= idx {
		s = append(s, nil)
	}
	if 1 == len(parts) {
		s[idx] = value
		return s, nil
	}
	// Recurse into the element. If next segment is numeric, we have a
	// slice-of-slice; otherwise we need a map at this index.
	next := parts[1]
	if isInt(next) {
		inner, _ := s[idx].([]any)
		updated, err := setSlice(inner, parts[1:], value)
		if nil != err {
			return nil, err
		}
		s[idx] = updated
		return s, nil
	}
	innerMap, _ := s[idx].(map[string]any)
	if nil == innerMap {
		innerMap = map[string]any{}
	}
	if err := setRecursive(innerMap, parts[1:], value); nil != err {
		return nil, err
	}
	s[idx] = innerMap
	return s, nil
}

func isInt(s string) bool {
	if "" == s {
		return false
	}
	_, err := strconv.Atoi(s)
	return nil == err
}

func inferType(s string) any {
	if "null" == s || "nil" == s {
		return nil
	}
	if "true" == s {
		return true
	}
	if "false" == s {
		return false
	}

	if i, err := strconv.ParseInt(s, 10, 64); nil == err {
		return int(i)
	}
	if f, err := strconv.ParseFloat(s, 64); nil == err {
		return f
	}

	return s
}
