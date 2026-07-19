---
title: "Collection and Type functions"
nav_order: 2
parent: "Functions"
grand_parent: "Templates"
---
{% raw %}
# Collection and Type functions

> Pipeline convention: `${value | f x}` is evaluated as `f(value, x)`.

Collection functions operate on the engine's native containers: a list is `[]any`, a map is `map[string]any`. Passing the wrong container type raises a function error. The inline list constructor is `tuple` (e.g. `${tuple 10 2 1}` is the list `[10,2,1]`); note that `tuple` coerces numeric-looking strings to numbers.

## Collection functions

### `keys`
`keys(coll)` → list

Returns the map's keys as a list, sorted alphabetically. Errors `keys: expected map, got <T>` for non-maps.

**Examples**
```
${dict "b" 2 "a" 1 | keys}       → ["a","b"]
${dict "x" 1 | omit "x" | keys}  → []
```

### `values`
`values(coll)` → list

Returns the map's values ordered by their keys' alphabetical sort. Errors for non-maps.

**Examples**
```
${dict "b" 2 "a" 1 | values}   → [1,2]
```

### `first`
`first(coll)` → element | null

First element of a list; `null` on an empty list. Errors for non-lists.

**Examples**
```
${tuple 1 2 3 | first}   → 1
${until 0 | first}       → null
```

### `last`
`last(coll)` → element | null

Last element of a list; `null` on an empty list. Errors for non-lists.

**Examples**
```
${tuple 1 2 3 | last}   → 3
```

### `join`
`join(coll, sep?)` → string

Joins list elements (each stringified with `%v`); `sep` defaults to `,`. Errors for non-lists.

| Param | Type | Description |
|---|---|---|
| `coll` | list | elements to join |
| `sep` | string | optional separator (default `,`) |

**Examples**
```
${tuple "a" "b" "c" | join}   → "a,b,c"
${tuple 1 2 3 | join "-"}     → "1-2-3"
```

### `sortAlpha`
`sortAlpha(coll)` → list

Stringifies every element and sorts alphabetically (lexicographic — `"10"` before `"2"`); output elements are strings. Errors for non-lists. For numeric order (10 after 2), use [`sortNumeric`](#sortnumeric).

**Examples**
```
${tuple "banana" "apple" | sortAlpha}   → ["apple","banana"]
${tuple 10 2 1 | sortAlpha}             → ["1","10","2"]
```

### `sortNumeric`
`sortNumeric(value)` → list

Sorts a list by numeric value (ascending), preserving each element and its type. Every element must be numeric or a numeric string, otherwise it errors (`sortNumeric: element N (…) is not numeric`). Errors for non-lists. Contrast with [`sortAlpha`](#sortalpha), which sorts lexically after stringifying (so `"10"` sorts before `"2"`).

**Examples**
```
${tuple 10 2 1 | sortNumeric}              → [1,2,10]
${"10,2,1" | split "," | sortNumeric}      → ["1","2","10"]
```

### `uniq`
`uniq(coll)` → list

Removes duplicates (dedup key is the `%v` string form), preserving first-seen order and original element types. Errors for non-lists.

**Examples**
```
${tuple 1 2 2 3 1 | uniq}   → [1,2,3]
```

### `compact`
`compact(coll)` → list

Drops "empty" elements (`nil`, `""`, `false`, numeric `0`, empty list/map), preserving order. Errors for non-lists.

**Examples**
```
${tuple 1 0 2 "" 3 | compact}   → [1,2,3]
```

### `has`
`has(coll, item)` → bool

Reports whether a map contains the key, or a list contains the item (compared via `%v`). Errors if `item` is missing or `coll` is neither map nor list.

**Examples**
```
${dict "a" 1 | has "a"}   → true
${tuple 1 2 3 | has 2}    → true
${tuple 1 2 3 | has 9}    → false
```

## Type functions

### `toYaml`
`toYaml(value)` → string

Marshals any value to YAML (trailing newline stripped).

**Examples**
```
${dict "a" 1 "b" 2 | toYaml}
# →
a: 1
b: 2
```
```
${tuple 1 2 3 | toYaml}
# →
- 1
- 2
- 3
```

### `toJson`
`toJson(value)` → string

Marshals any value to compact JSON.

**Examples**
```
${dict "a" 1 | toJson}   → {"a":1}
${nil | toJson}          → "null"
```

### `toString`
`toString(value)` → string

Converts any value to `%v` form. Never errors. `nil` → `<nil>`; a list renders space-separated in brackets.

**Examples**
```
${42 | toString}        → "42"
${tuple 1 2 | toString} → "[1 2]"
```

### `toInt`
`toInt(value)` → int

Converts numbers/strings/bools to int. `int64`/`float64` cast (floats truncate toward zero); strings parsed with `Atoi` (integers only); `true`/`false` → `1`/`0`. Errors on an unparseable string or unsupported type (incl. `nil`).

**Examples**
```
${"42" | toInt}    → 42
${3.9 | toInt}     → 3
${true | toInt}    → 1
```

### `toBool`
`toBool(value)` → bool

Converts to boolean. Strings via `ParseBool` (`1/t/true/…`, `0/f/false/…`); numbers `false` only when zero; `nil` → `false`. Errors on an unparseable string or unsupported type.

**Examples**
```
${"true" | toBool}   → true
${0 | toBool}        → false
${nil | toBool}      → false
```
{% endraw %}
