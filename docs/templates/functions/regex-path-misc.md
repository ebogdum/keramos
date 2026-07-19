---
title: "Regex, Path, and Misc functions"
parent: "Functions"
grand_parent: "Templates"
---
{% raw %}
# Regex, Path, and Misc functions

> **Pipeline note.** `${value | f x}` = `f(value, x)`. Regex helpers compile `args[0]` as an RE2 pattern and match against the stringified `value`; a bad pattern or missing pattern errors.

## Regex functions

### `regexMatch`
`regexMatch(value, pattern)` → bool — does `pattern` match anywhere in `value`?
```
${"abc123" | regexMatch "[0-9]+"}   → true
${"abc" | regexMatch "^x"}          → false
```

### `regexFind`
`regexFind(value, pattern)` → string — leftmost match, or `""`.
```
${"a1b2c3" | regexFind "[0-9]"}     → "1"
```

### `regexFindAll`
`regexFindAll(value, pattern, n?)` → []string — all matches. Cap **1024** (an explicit `n` applies only when `0 < n < 1024`).
```
${"a1b2" | regexFindAll "[0-9]"}       → ["1","2"]
${"a1b2c3" | regexFindAll "[0-9]" 2}   → ["1","2"]
```

### `regexReplaceAll`
`regexReplaceAll(value, pattern, replacement)` → string — supports `$1`/`${name}` submatch expansion.
```
${"a1b2" | regexReplaceAll "[0-9]" "#"}   → "a#b#"
```

### `regexSplit`
`regexSplit(value, pattern, n?)` → []string — split on `pattern`. Same 1024 cap as `regexFindAll`.
```
${"a1b2c" | regexSplit "[0-9]"}   → ["a","b","c"]
```

## Path functions

`base`/`dir`/`clean`/`ext`/`isAbs` use slash-only paths (good for URLs/config); the `os*` variants use OS-aware `path/filepath`.

### `base`
`base(value)` → string — last path element.
```
${"/a/b/c.yaml" | base}   → "c.yaml"
${"" | base}              → "."
```

### `dir`
`dir(value)` → string — everything but the last element, cleaned.
```
${"/a/b/c.yaml" | dir}    → "/a/b"
```

### `clean`
`clean(value)` → string — shortest equivalent path (collapses `.`/`..`).
```
${"/a/b/../c" | clean}    → "/a/c"
```

### `ext`
`ext(value)` → string — extension including the dot, or `""`.
```
${"/a/b/c.yaml" | ext}    → ".yaml"
```

### `isAbs`
`isAbs(value)` → bool — starts with `/`? (literal check).
```
${"/etc/hosts" | isAbs}   → true
```

### `osBase` / `osDir` / `osClean` / `osExt` / `osIsAbs`
Same as above but via `path/filepath` (OS-aware separators / volume names).
```
${"/a/b/c.yaml" | osBase}  → "c.yaml"
${"/etc/hosts" | osIsAbs}  → true
```

### `urlParse`
`urlParse(value)` → map — parses a URL into `scheme`, `host`, `hostname`, `port`, `path`, `query`, `fragment`, `userinfo`, `opaque`.
```
${"https://u:p@ex.com:8080/a?x=1#f" | urlParse | get "hostname"}  → "ex.com"
${"https://ex.com:8080/a?x=1" | urlParse | get "port"}           → "8080"
```

### `urlJoin`
`urlJoin(value)` → string — inverse of `urlParse` (reads `scheme`/`host`/`path`/`query`/`fragment`).
```
${dict "scheme" "https" "host" "ex.com" "path" "/a" | urlJoin}  → "https://ex.com/a"
```

## Misc functions

### `printf` / `sprintf`
`printf(format, ...args)` → string — `fmt.Sprintf` with the value as format string.
```
${"hi %s" | printf "there"}   → "hi there"
${"%.2f" | sprintf 3.14159}   → "3.14"
```

### `dict`
`dict(value, ...args)` → map — build a map from alternating key/value pairs; values keep their native type. Odd count errors.
```
${dict "a" 1 "b" 2}   → {"a":1,"b":2}
```

### `set`
`set(value, key, val)` → map — copy of the map with `key`=`val` (copy-on-write).
```
${dict "a" 1 | set "b" 2}   → {"a":1,"b":2}
```

### `unset`
`unset(value, ...keys)` → map — copy with keys removed.
```
${dict "a" 1 "b" 2 | unset "a"}   → {"b":2}
```

### `get`
`get(value, key)` → any — value at `key`, or `nil`.
```
${dict "a" 1 | get "a"}   → 1
${dict "a" 1 | get "z"}   → null
```

### `hasKey`
`hasKey(value, key)` → bool — map contains `key`? (never errors; non-map → false).
```
${dict "a" 1 | hasKey "a"}   → true
```

### `merge`
`merge(value, ...jsonSources)` → map — deep-merge JSON-string sources; **destination wins** on a conflict unless the destination's value is a zero value.
```
${dict "a" 1 | merge '{"a":9}'}   → {"a":1}   (dest non-zero kept)
${dict "a" 0 | merge '{"a":9}'}   → {"a":9}   (dest zero filled)
```

### `mergeOverwrite`
`mergeOverwrite(value, ...jsonSources)` → map — like `merge` but the **source always wins**.
```
${dict "a" 1 | mergeOverwrite '{"a":9}'}   → {"a":9}
```

### `pick`
`pick(value, ...keys)` → map — only the named keys.
```
${dict "a" 1 "b" 2 "c" 3 | pick "a" "c"}   → {"a":1,"c":3}
```

### `omit`
`omit(value, ...keys)` → map — every key except the named ones.
```
${dict "a" 1 "b" 2 "c" 3 | omit "b"}   → {"a":1,"c":3}
```

### `fail`
`fail(value, ...args)` → error — aborts rendering with the joined message.
```
${"config invalid" | fail}   → error: fail: config invalid
```

### `kindOf` / `typeOf`
`kindOf(value)` → string — `"invalid"`, `"bool"`, `"int"`, `"float64"`, `"string"`, `"map"`, `"slice"`.
```
${5 | kindOf}          → "int"
${dict "a" 1 | typeOf} → "map"
```

### `kindIs` / `typeIs`
`kindIs(value, kind)` → bool — is `kindOf(value)` == `kind`?
```
${5 | kindIs "int"}    → true
${"x" | typeIs "int"}  → false
```

### `semver`
`semver(value)` → string — normalized semantic version (leading `v` stripped). Errors on unparseable.
```
${"v1.2.3" | semver}   → "1.2.3"
```

### `semverCompare`
`semverCompare(value, constraint)` → bool — does the version satisfy the constraint?
```
${"1.2.3" | semverCompare ">=1.0.0"}   → true
${"1.2.3" | semverCompare "^2.0.0"}    → false
```

### `coalesce`
`coalesce(value, ...args)` → any — first non-empty candidate (value first), else `nil`.
```
${"" | coalesce "fallback"}   → "fallback"
${0 | coalesce "" "z"}        → "z"
```

### `toJson` / `toYAML`
`toJson(value)` → string (compact JSON) · `toYAML(value)` → string (YAML, trailing newline trimmed).
```
${dict "a" 1 | toJson}   → {"a":1}
${dict "a" 1 | toYAML}   → a: 1
```

### `len`
`len(value)` → int — byte length (string) / element count (list) / key count (map); other → 0.
```
${"hello" | len}   → 5
```

### `repeat`
`repeat(value, count)` → string — repeat the string. Caps: `count` ≤ 65536 AND result ≤ 32 MB.
```
${"ab" | repeat 3}      → "ababab"
${"x" | repeat 70000}   → error (count exceeds 65536)
```

### `contains` / `hasPrefix` / `hasSuffix`
`contains(value, substr)` / `hasPrefix(value, p)` / `hasSuffix(value, s)` → bool.
```
${"hello" | contains "ell"}    → true
${"hello" | hasPrefix "he"}    → true
${"hello" | hasSuffix "lo"}    → true
```

### `split`
`split(value, sep)` → []string — split on a literal separator (no cap).
```
${"a,b,c" | split ","}   → ["a","b","c"]
```

### `title` / `untitle`
`title(value)` → Title Case Each Word · `untitle(value)` → lowercase first letter of each word.
```
${"hello world" | title}     → "Hello World"
${"Hello World" | untitle}   → "hello world"
```

### `substr`
`substr(value, start, end)` → string — rune-indexed `[start, end)` slice (clamped).
```
${"hello" | substr 0 3}   → "hel"
${"hello" | substr 3 1}   → ""
```

### `cat`
`cat(value, ...args)` → string — join tokens with single spaces.
```
${"a" | cat "b" "c"}   → "a b c"
```
{% endraw %}
