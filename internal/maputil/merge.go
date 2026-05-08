package maputil

// DeepMerge merges src into dst recursively.
// Maps are merged recursively, all other types are replaced.
// src values take precedence over dst values.
func DeepMerge(dst, src map[string]any) map[string]any {
	if nil == dst {
		return CopyMap(src)
	}
	if nil == src {
		return CopyMap(dst)
	}

	result := CopyMap(dst)
	for key, srcVal := range src {
		dstVal, exists := result[key]
		if !exists {
			result[key] = DeepCopyValue(srcVal)
			continue
		}

		srcMap := ToMap(srcVal)
		dstMap := ToMap(dstVal)
		if nil != srcMap && nil != dstMap {
			result[key] = DeepMerge(dstMap, srcMap)
			continue
		}

		result[key] = DeepCopyValue(srcVal)
	}

	return result
}

// ToMap attempts to convert a value to map[string]any.
func ToMap(v any) map[string]any {
	if nil == v {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

// CopyMap creates a shallow-deep copy of a map.
func CopyMap(m map[string]any) map[string]any {
	if nil == m {
		return nil
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = DeepCopyValue(v)
	}
	return result
}

// DeepCopyValue deep-copies a value (maps and slices are copied, scalars are shared).
func DeepCopyValue(v any) any {
	if m := ToMap(v); nil != m {
		return CopyMap(m)
	}
	switch val := v.(type) {
	case []any:
		cp := make([]any, len(val))
		for i, item := range val {
			cp[i] = DeepCopyValue(item)
		}
		return cp
	default:
		return v
	}
}
