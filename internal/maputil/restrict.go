package maputil

import "strings"

// RestrictToPaths produces a new map equal to `prev` except at every dotted
// path in `paths`, where the value from `next` is used. Missing-in-next paths
// are deleted from the result.
//
// This implements `keramos upgrade --only key1,key2`: only the listed keys are
// promoted to their new values; all other keys remain at the previous
// revision's value.
func RestrictToPaths(prev, next map[string]any, paths []string) map[string]any {
	result := CopyMap(prev)
	for _, p := range paths {
		applyRestriction(result, next, splitPath(p))
	}
	return result
}

func splitPath(p string) []string {
	parts := strings.Split(p, ".")
	out := make([]string, 0, len(parts))
	for _, x := range parts {
		x = strings.TrimSpace(x)
		if "" != x {
			out = append(out, x)
		}
	}
	return out
}

// applyRestriction copies the value at the dotted path from `src` into `dst`,
// creating intermediate maps as needed. If the value is missing from src, the
// path is removed from dst.
//
// When `dst` holds a scalar at an intermediate path component (a previous
// revision had `db: "uri"` and the new path is `db.password`), we adopt the
// new shape from `src` directly: replace dst[head] with the corresponding
// src subtree. Falling back to a synthesised empty map silently dropped the
// previous scalar and could delete the leaf entirely if `src` also had a
// scalar there. Treating the path as an explicit type-changing rewrite is
// closer to user intent.
func applyRestriction(dst, src map[string]any, parts []string) {
	if 0 == len(parts) {
		return
	}
	srcVal, srcOk := lookupPath(src, parts)
	if 1 == len(parts) {
		if !srcOk {
			delete(dst, parts[0])
			return
		}
		dst[parts[0]] = DeepCopyValue(srcVal)
		return
	}
	head, rest := parts[0], parts[1:]
	child, ok := dst[head].(map[string]any)
	if !ok {
		// Previous value at `head` was a scalar (or absent). Hoist whatever
		// `src` has at this point — if src also has a scalar/missing here,
		// the recursive call deletes the leaf, which is the correct outcome.
		if srcChild, srcChildOk := src[head].(map[string]any); srcChildOk {
			child = DeepCopyValue(srcChild).(map[string]any)
		} else {
			child = map[string]any{}
		}
		dst[head] = child
	}
	applyRestriction(child, mapAt(src, head), rest)
}

func mapAt(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return map[string]any{}
}

func lookupPath(m map[string]any, parts []string) (any, bool) {
	var cur any = m
	for _, p := range parts {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, exists := mm[p]
		if !exists {
			return nil, false
		}
		cur = v
	}
	return cur, true
}
