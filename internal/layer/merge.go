package layer

import "github.com/ebogdum/keramos/v2/internal/maputil"

// DeepMerge merges src into dst recursively.
// Maps are merged recursively, all other types are replaced.
// src values take precedence over dst values.
func DeepMerge(dst, src map[string]any) map[string]any {
	return maputil.DeepMerge(dst, src)
}
