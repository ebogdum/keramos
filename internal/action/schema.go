package action

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"time"

	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
)

const maxRefDepth = 32

// ValidateValuesAgainstSchema validates a merged values map against the
// package's values.schema.json (if present). The validator implements a
// pragmatic JSON-Schema Draft 2020-12 subset:
//
//   - Type: object/array/string/integer/number/boolean/null + union types
//   - Object: required, properties, additionalProperties, patternProperties,
//     minProperties, maxProperties, dependentRequired
//   - Array: items, minItems, maxItems, uniqueItems
//   - String: minLength, maxLength, pattern, format
//   - Numeric: minimum, maximum, exclusiveMinimum, exclusiveMaximum, multipleOf
//   - Combinators: allOf, anyOf, oneOf, not
//   - References: $ref, $defs, definitions (single-document only)
//   - Misc: enum, const
//
// Unknown keywords are silently ignored.
func ValidateValuesAgainstSchema(packagePath string, vals map[string]any) error {
	schemaPath := filepath.Join(packagePath, "values.schema.json")
	data, err := os.ReadFile(schemaPath)
	if nil != err {
		if os.IsNotExist(err) {
			return nil
		}
		return keramoserr.WrapError(keramoserr.ErrCLIValidation, "failed to read values.schema.json", err)
	}

	var root map[string]any
	if jsonErr := json.Unmarshal(data, &root); nil != jsonErr {
		return keramoserr.WrapError(keramoserr.ErrCLIValidation, "values.schema.json is not valid JSON", jsonErr)
	}

	v := &validator{root: root}
	v.validate("$", vals, root, 0)
	if 0 < len(v.errs) {
		return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"values failed schema validation:\n  - %s", strings.Join(v.errs, "\n  - "))
	}
	return nil
}

// maxSchemaPatternLen caps the length of `pattern` and `patternProperties`
// keys to prevent DoS via gargantuan attacker-supplied regexes shipped in
// values.schema.json. Go's regexp is RE2 (linear time), but compilation cost
// is still O(n) and we re-compile on every validateString / validateObject
// call without the cache below.
const maxSchemaPatternLen = 512

type validator struct {
	root        map[string]any
	errs        []string
	regexCache  map[string]*regexp.Regexp
}

func (v *validator) compilePattern(pattern string) (*regexp.Regexp, error) {
	if maxSchemaPatternLen < len(pattern) {
		return nil, fmt.Errorf("pattern length %d exceeds %d-byte safety cap", len(pattern), maxSchemaPatternLen)
	}
	if nil == v.regexCache {
		v.regexCache = make(map[string]*regexp.Regexp, 4)
	}
	if cached, ok := v.regexCache[pattern]; ok {
		return cached, nil
	}
	re, err := regexp.Compile(pattern)
	if nil != err {
		return nil, err
	}
	v.regexCache[pattern] = re
	return re, nil
}

func (v *validator) addf(format string, args ...any) {
	v.errs = append(v.errs, fmt.Sprintf(format, args...))
}

func (v *validator) validate(path string, value any, schema map[string]any, depth int) {
	if nil == schema {
		return
	}
	if maxRefDepth <= depth {
		v.addf("%s: $ref recursion exceeds %d levels", path, maxRefDepth)
		return
	}

	// $ref dereferencing first (replaces the active schema).
	if ref, ok := schema["$ref"].(string); ok {
		resolved := v.resolveRef(ref)
		if nil == resolved {
			v.addf("%s: cannot resolve $ref %q", path, ref)
			return
		}
		v.validate(path, value, resolved, depth+1)
		return
	}

	if c, ok := schema["const"]; ok {
		if !equalAny(c, value) {
			v.addf("%s: value does not equal const", path)
		}
	}
	if enum, ok := schema["enum"].([]any); ok {
		if !enumContains(enum, value) {
			v.addf("%s: value not in enum", path)
		}
	}

	// Combinators
	if subs, ok := schema["allOf"].([]any); ok {
		for i, sub := range subs {
			if sm, ok := sub.(map[string]any); ok {
				v.validate(fmt.Sprintf("%s/allOf[%d]", path, i), value, sm, depth+1)
			}
		}
	}
	if subs, ok := schema["anyOf"].([]any); ok {
		passed := false
		for _, sub := range subs {
			if sm, ok := sub.(map[string]any); ok {
				probe := &validator{root: v.root, regexCache: v.regexCache}
				probe.validate(path, value, sm, depth+1)
				if 0 == len(probe.errs) {
					passed = true
					break
				}
			}
		}
		if !passed {
			v.addf("%s: value does not match any of anyOf", path)
		}
	}
	if subs, ok := schema["oneOf"].([]any); ok {
		matches := 0
		for _, sub := range subs {
			if sm, ok := sub.(map[string]any); ok {
				probe := &validator{root: v.root, regexCache: v.regexCache}
				probe.validate(path, value, sm, depth+1)
				if 0 == len(probe.errs) {
					matches++
				}
			}
		}
		if 1 != matches {
			v.addf("%s: value matches %d of oneOf (expected exactly 1)", path, matches)
		}
	}
	if sub, ok := schema["not"].(map[string]any); ok {
		probe := &validator{root: v.root, regexCache: v.regexCache}
		probe.validate(path, value, sub, depth+1)
		if 0 == len(probe.errs) {
			v.addf("%s: value matched 'not' schema", path)
		}
	}

	switch t := schema["type"].(type) {
	case string:
		v.validateType(path, t, value, schema, depth)
	case []any:
		matched := false
		for _, ty := range t {
			if s, ok := ty.(string); ok && typeMatches(s, value) {
				matched = true
				v.validateType(path, s, value, schema, depth)
				break
			}
		}
		if !matched {
			v.addf("%s: type does not match any of %v", path, t)
		}
	default:
		// No explicit type. Infer from schema shape so common patterns work.
		v.validateUntyped(path, value, schema, depth)
	}
}

// validateUntyped applies object/array/string/numeric keywords without a type
// declaration. Real-world schemas often omit `"type": "object"` at the root.
func (v *validator) validateUntyped(path string, value any, schema map[string]any, depth int) {
	if obj, ok := value.(map[string]any); ok {
		if hasObjectKeywords(schema) {
			v.validateObject(path, obj, schema, depth)
		}
		return
	}
	if arr, ok := value.([]any); ok {
		if hasArrayKeywords(schema) {
			v.validateArray(path, arr, schema, depth)
		}
		return
	}
	if s, ok := value.(string); ok {
		if hasStringKeywords(schema) {
			v.validateString(path, s, schema)
		}
		return
	}
	if f, ok := numericValue(value); ok {
		if hasNumericKeywords(schema) {
			v.validateNumeric(path, f, schema)
		}
		return
	}
}

func hasObjectKeywords(s map[string]any) bool {
	for _, k := range []string{"properties", "required", "patternProperties", "additionalProperties", "dependentRequired", "minProperties", "maxProperties"} {
		if _, ok := s[k]; ok {
			return true
		}
	}
	return false
}

func hasArrayKeywords(s map[string]any) bool {
	for _, k := range []string{"items", "minItems", "maxItems", "uniqueItems"} {
		if _, ok := s[k]; ok {
			return true
		}
	}
	return false
}

func hasStringKeywords(s map[string]any) bool {
	for _, k := range []string{"minLength", "maxLength", "pattern", "format"} {
		if _, ok := s[k]; ok {
			return true
		}
	}
	return false
}

func hasNumericKeywords(s map[string]any) bool {
	for _, k := range []string{"minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf"} {
		if _, ok := s[k]; ok {
			return true
		}
	}
	return false
}

func (v *validator) resolveRef(ref string) map[string]any {
	if !strings.HasPrefix(ref, "#/") {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
	var cur any = v.root
	for _, p := range parts {
		decoded := strings.ReplaceAll(strings.ReplaceAll(p, "~1", "/"), "~0", "~")
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		next, exists := m[decoded]
		if !exists {
			return nil
		}
		cur = next
	}
	resolved, ok := cur.(map[string]any)
	if !ok {
		return nil
	}
	return resolved
}

func (v *validator) validateType(path, typ string, value any, schema map[string]any, depth int) {
	switch typ {
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			if nil != value {
				v.addf("%s: expected object, got %T", path, value)
			}
			return
		}
		v.validateObject(path, obj, schema, depth)
	case "array":
		arr, ok := value.([]any)
		if !ok {
			if nil != value {
				v.addf("%s: expected array, got %T", path, value)
			}
			return
		}
		v.validateArray(path, arr, schema, depth)
	case "string":
		s, ok := value.(string)
		if !ok {
			if nil != value {
				v.addf("%s: expected string, got %T", path, value)
			}
			return
		}
		v.validateString(path, s, schema)
	case "integer":
		f, ok := numericValue(value)
		if !ok || f != math.Trunc(f) {
			v.addf("%s: expected integer, got %T", path, value)
			return
		}
		v.validateNumeric(path, f, schema)
	case "number":
		f, ok := numericValue(value)
		if !ok {
			v.addf("%s: expected number, got %T", path, value)
			return
		}
		v.validateNumeric(path, f, schema)
	case "boolean":
		if _, ok := value.(bool); !ok {
			v.addf("%s: expected boolean, got %T", path, value)
		}
	case "null":
		if nil != value {
			v.addf("%s: expected null", path)
		}
	}
}

func (v *validator) validateObject(path string, obj map[string]any, schema map[string]any, depth int) {
	if req, ok := schema["required"].([]any); ok {
		for _, r := range req {
			if name, ok := r.(string); ok {
				if _, present := obj[name]; !present {
					v.addf("%s.%s: required property missing", path, name)
				}
			}
		}
	}
	if minP, ok := numFromAny(schema["minProperties"]); ok && float64(len(obj)) < minP {
		v.addf("%s: %d properties is less than minProperties %d", path, len(obj), int(minP))
	}
	if maxP, ok := numFromAny(schema["maxProperties"]); ok && float64(len(obj)) > maxP {
		v.addf("%s: %d properties exceeds maxProperties %d", path, len(obj), int(maxP))
	}
	if dr, ok := schema["dependentRequired"].(map[string]any); ok {
		for trigger, deps := range dr {
			if _, present := obj[trigger]; !present {
				continue
			}
			depList, ok := deps.([]any)
			if !ok {
				continue
			}
			for _, d := range depList {
				if name, ok := d.(string); ok {
					if _, has := obj[name]; !has {
						v.addf("%s: property %q requires %q to be set", path, trigger, name)
					}
				}
			}
		}
	}

	props, _ := schema["properties"].(map[string]any)
	patternProps, _ := schema["patternProperties"].(map[string]any)
	additional, hasAdditional := schema["additionalProperties"]

	patternMatchers := make([]struct {
		re *regexp.Regexp
		s  map[string]any
	}, 0, len(patternProps))
	for k, sv := range patternProps {
		re, err := v.compilePattern(k)
		if nil != err {
			// Surface invalid patternProperties keys as a real validation
			// error rather than silently bypassing them.
			v.addf("%s: invalid patternProperties key %q: %v", path, k, err)
			continue
		}
		if sm, ok := sv.(map[string]any); ok {
			patternMatchers = append(patternMatchers, struct {
				re *regexp.Regexp
				s  map[string]any
			}{re, sm})
		}
	}

	for k, val := range obj {
		matched := false
		if subSchema, knownProp := props[k].(map[string]any); knownProp {
			v.validate(path+"."+k, val, subSchema, depth+1)
			matched = true
		}
		for _, pm := range patternMatchers {
			if pm.re.MatchString(k) {
				v.validate(path+"."+k, val, pm.s, depth+1)
				matched = true
			}
		}
		if matched {
			continue
		}
		if hasAdditional {
			if allow, ok := additional.(bool); ok && !allow {
				v.addf("%s.%s: additional property not allowed", path, k)
				continue
			}
			if addSchema, ok := additional.(map[string]any); ok {
				v.validate(path+"."+k, val, addSchema, depth+1)
			}
		}
	}
}

func (v *validator) validateArray(path string, arr []any, schema map[string]any, depth int) {
	if minI, ok := numFromAny(schema["minItems"]); ok && float64(len(arr)) < minI {
		v.addf("%s: %d items is less than minItems %d", path, len(arr), int(minI))
	}
	if maxI, ok := numFromAny(schema["maxItems"]); ok && float64(len(arr)) > maxI {
		v.addf("%s: %d items exceeds maxItems %d", path, len(arr), int(maxI))
	}
	if u, ok := schema["uniqueItems"].(bool); ok && u {
		seen := make([]any, 0, len(arr))
		for _, item := range arr {
			for _, prev := range seen {
				if reflect.DeepEqual(prev, item) {
					v.addf("%s: duplicate item in array (uniqueItems)", path)
				}
			}
			seen = append(seen, item)
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		for i, el := range arr {
			v.validate(fmt.Sprintf("%s[%d]", path, i), el, items, depth+1)
		}
	}
}

func (v *validator) validateString(path, s string, schema map[string]any) {
	if minLen, ok := numFromAny(schema["minLength"]); ok && len(s) < int(minLen) {
		v.addf("%s: string shorter than minLength %d", path, int(minLen))
	}
	if maxLen, ok := numFromAny(schema["maxLength"]); ok && len(s) > int(maxLen) {
		v.addf("%s: string longer than maxLength %d", path, int(maxLen))
	}
	if pattern, ok := schema["pattern"].(string); ok {
		re, err := v.compilePattern(pattern)
		if nil != err {
			v.addf("%s: invalid pattern in schema: %v", path, err)
		} else if !re.MatchString(s) {
			v.addf("%s: string does not match pattern %q", path, pattern)
		}
	}
	if format, ok := schema["format"].(string); ok {
		if formatErr := validateFormat(format, s); nil != formatErr {
			v.addf("%s: %v", path, formatErr)
		}
	}
}

func (v *validator) validateNumeric(path string, f float64, schema map[string]any) {
	if min, ok := numFromAny(schema["minimum"]); ok && f < min {
		v.addf("%s: %v less than minimum %v", path, f, min)
	}
	if max, ok := numFromAny(schema["maximum"]); ok && f > max {
		v.addf("%s: %v greater than maximum %v", path, f, max)
	}
	if eMin, ok := numFromAny(schema["exclusiveMinimum"]); ok && f <= eMin {
		v.addf("%s: %v not greater than exclusiveMinimum %v", path, f, eMin)
	}
	if eMax, ok := numFromAny(schema["exclusiveMaximum"]); ok && f >= eMax {
		v.addf("%s: %v not less than exclusiveMaximum %v", path, f, eMax)
	}
	if mul, ok := numFromAny(schema["multipleOf"]); ok && 0 != mul {
		ratio := f / mul
		if math.Abs(ratio-math.Round(ratio)) > 1e-9 {
			v.addf("%s: %v is not a multiple of %v", path, f, mul)
		}
	}
}

// validateFormat implements a subset of JSON Schema's format keyword.
func validateFormat(format, s string) error {
	switch format {
	case "email":
		if !strings.Contains(s, "@") {
			return fmt.Errorf("not a valid email")
		}
		at := strings.LastIndex(s, "@")
		if 0 == at || at == len(s)-1 {
			return fmt.Errorf("not a valid email")
		}
	case "uri", "uri-reference":
		if _, err := url.Parse(s); nil != err {
			return fmt.Errorf("not a valid URI")
		}
	case "uuid":
		uuidRe := regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
		if !uuidRe.MatchString(s) {
			return fmt.Errorf("not a valid UUID")
		}
	case "ipv4":
		ip := net.ParseIP(s)
		if nil == ip || nil == ip.To4() {
			return fmt.Errorf("not a valid IPv4 address")
		}
	case "ipv6":
		ip := net.ParseIP(s)
		if nil == ip || nil != ip.To4() {
			return fmt.Errorf("not a valid IPv6 address")
		}
	case "hostname":
		// RFC 1123: labels of 1-63 chars, alphanumerics and hyphens, not starting/ending with hyphen.
		if 253 < len(s) || "" == s {
			return fmt.Errorf("not a valid hostname")
		}
		labelRe := regexp.MustCompile(`^[a-zA-Z0-9]([-a-zA-Z0-9]{0,61}[a-zA-Z0-9])?$`)
		for _, label := range strings.Split(s, ".") {
			if !labelRe.MatchString(label) {
				return fmt.Errorf("not a valid hostname")
			}
		}
	case "date":
		if _, err := time.Parse("2006-01-02", s); nil != err {
			return fmt.Errorf("not a valid date")
		}
	case "date-time":
		if _, err := time.Parse(time.RFC3339, s); nil != err {
			return fmt.Errorf("not a valid date-time")
		}
	case "time":
		if _, err := time.Parse("15:04:05", s); nil != err {
			return fmt.Errorf("not a valid time")
		}
	}
	return nil
}

func typeMatches(typ string, value any) bool {
	switch typ {
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "null":
		return nil == value
	case "integer":
		f, ok := numericValue(value)
		return ok && f == math.Trunc(f)
	case "number":
		_, ok := numericValue(value)
		return ok
	}
	return false
}

func numericValue(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case float32:
		return float64(n), true
	}
	return 0, false
}

func numFromAny(v any) (float64, bool) {
	return numericValue(v)
}

func enumContains(enum []any, value any) bool {
	for _, e := range enum {
		if equalAny(e, value) {
			return true
		}
	}
	return false
}

func equalAny(a, b any) bool {
	if reflect.DeepEqual(a, b) {
		return true
	}
	af, aok := numericValue(a)
	bf, bok := numericValue(b)
	if aok && bok {
		return af == bf
	}
	return false
}
