# Sequence, Sprig-parity, and External functions

> **Pipeline note.** `${value | f x}` = `f(value, x)`. Ranges are capped at 65536 items.

## Sequence functions

### `until`
`until(n)` → list — integers `[0, n)` (counts down if `n` is negative).
```
${5 | until}    → [0,1,2,3,4]
${-3 | until}   → [0,-1,-2]
```

### `untilStep`
`untilStep(start, stop, step)` → list — from `start` toward `stop` (exclusive) by `step`.
```
${1 | untilStep 10 2}   → [1,3,5,7,9]
${10 | untilStep 0 -3}  → [10,7,4,1]
```

### `seq`
`seq([start,] [step,] end)` → list — Unix `seq` (endpoint inclusive).
```
${5 | seq}       → [1,2,3,4,5]
${1 | seq 2 9}   → [1,3,5,7,9]
```

## List functions

### `slice`
`slice(list, start?, end?)` → list — sub-slice `[start, end)`.
```
${[10,20,30,40] | slice 1 3}   → [20,30]
```

### `append` / `prepend`
`append(list, ...items)` / `prepend(list, ...items)` → list.
```
${[1,2] | append 3 4}    → [1,2,3,4]
${[2,3] | prepend 0 1}   → [0,1,2,3]
```

### `concat`
`concat(list, ...lists)` → list — flatten one level.
```
${[1,2] | concat [3,4] 5}   → [1,2,3,4,5]
```

### `reverse`
`reverse(list)` → list.
```
${[1,2,3] | reverse}   → [3,2,1]
```

### `first` / `last` / `initial` / `rest`
Head / tail / all-but-last / all-but-first.
```
${[1,2,3] | initial}   → [1,2]
${[1,2,3] | rest}      → [2,3]
```

### `without`
`without(list, ...items)` → list — remove matching elements.
```
${[1,2,3,2] | without 2}   → [1,3]
```

### `pluck`
`pluck(list, key)` → list — collect `key` from each map.
```
${[{"name":"a"},{"name":"b"}] | pluck "name"}   → ["a","b"]
```

### `chunk`
`chunk(list, size)` → list of lists.
```
${[1,2,3,4,5] | chunk 2}   → [[1,2],[3,4],[5]]
```

### `tuple`
`tuple(value, ...items)` → list — ad-hoc list (value prepended).
```
${1 | tuple 2 3}   → [1,2,3]
```

## Map functions

### `dig`
`dig(map, ...path, default)` → any — walk a nested map; last arg is the default.
```
${{"a":{"b":{"c":7}}} | dig "a" "b" "c" 0}   → 7
${{"a":{}} | dig "a" "x" "y" "none"}         → "none"
```

### `deepCopy`
`deepCopy(value)` → any — independent recursive copy.

### `deepEqual`
`deepEqual(value, other)` → bool — deep structural equality.
```
${[1,2] | deepEqual [1,2]}   → true
```

## Encoding / conversion

### `b32enc` / `b32dec`
Base32 encode / decode.
```
${"hi" | b32enc}       → "NBUQ===="
${"NBUQ====" | b32dec} → "hi"
```

### `fromJson` / `fromYaml` / `fromYamlArray`
Parse a JSON string, one YAML doc, or a multi-doc YAML stream into a value.
```
${"{\"a\":1}" | fromJson}   → {"a":1}
${"a: 1" | fromYaml}        → {"a":1}
```

### `toRawJson`
`toRawJson(value)` → string — JSON without HTML-escaping.
```
${{"u":"a&b"} | toRawJson}   → {"u":"a&b"}
```

### `int` / `int64` / `float64`
Numeric coercion.
```
${"42" | int}      → 42
${"3.14" | float64} → 3.14
```

### `urlquery` / `urlqueryescape`
URL query-escape a string.
```
${"a b&c" | urlquery}   → "a+b%26c"
```

### `splitn`
`splitn(value, sep, n)` → list — split into at most `n` parts.
```
${"a,b,c,d" | splitn "," 2}   → ["a","b,c,d"]
```

### `regexQuoteMeta`
`regexQuoteMeta(value)` → string — escape regex metacharacters.
```
${"a.b*c" | regexQuoteMeta}   → "a\.b\*c"
```

## Number & date extras

### `addf` / `subf` / `mulf` / `divf`
Float arithmetic (always `float64`).
```
${1 | addf 2 3}    → 6.0
${100 | divf 2 5}  → 10.0
```

### `randInt`
`randInt(min, max)` → int — random in `[min, max)`.
```
${5 | randInt 10}   → 7 (varies, in [5,10))
```

### `dateModify`
`dateModify(value, duration)` → time — shift by a Go duration.
```
${$t | dateModify "24h"}   → time 24h later
```

### `htmlDate` / `htmlDateInZone`
Format a time as `YYYY-MM-DD` (optionally in an IANA zone).

### `derivePassword` / `genSignedCert`
Deterministic Master-Password derivation, and a CA-signed leaf certificate (returns `{Cert, Key}`).

## String extras

### `wrap` / `wrapWith`
Word-wrap at a column width (optionally with a custom break string).

### `nospace`
`nospace(value)` → string — remove all whitespace.
```
${"a b\tc" | nospace}   → "abc"
```

## Text helpers

### `biggest`
Alias of `max`.

### `pickv`
Alias of `pick`.

## `must*` variants

Sprig ships `must*` twins (`mustFirst`, `mustUniq`, `mustMerge`, `mustRegexFind`, `mustChunk`, `mustToJson`, …) that error instead of returning a zero value on bad input. In keramos the base functions **already** return errors on failure, so each `must*` behaves identically to its non-`must` counterpart — use whichever name reads better. The full set: `mustAppend`, `mustPrepend`, `mustSlice`, `mustReverse`, `mustWithout`, `mustInitial`, `mustRest`, `mustHas`, `mustFirst`, `mustLast`, `mustUniq`, `mustCompact`, `mustConcat`, `mustPick`, `mustOmit`, `mustMerge`, `mustMergeOverwrite`, `mustChunk`, `mustDeepCopy`, `mustToJson`, `mustToRawJson`, `mustFromJson`, `mustFromYaml`, `mustFromYamlArray`, `mustDate`, `mustToDate`, `mustDateModify`, `mustRegexMatch`, `mustRegexFind`, `mustRegexFindAll`, `mustRegexReplaceAll`, `mustRegexSplit`.

## External functions (network — opt-in)

These make render-time network calls. They are **disabled by default** and require `KERAMOS_RENDER_NETWORK=1`; internal/metadata IPs are blocked (SSRF guard). Use them only with charts you trust.

### `http`
`http(url, headers?)` → string — HTTP GET body.
```
${"https://api.example.com/health" | http}   → "ok" (requires KERAMOS_RENDER_NETWORK=1)
```

### `httpJSON`
`httpJSON(url, headers?)` → any — GET and parse JSON.
```
${"https://api.example.com/user" | httpJSON}   → {"id":1,"name":"…"}
```

### `vault`
`vault(path, field?)` → any — read a Vault KV-v2 secret (uses `VAULT_ADDR`/`VAULT_TOKEN`).
```
${"secret/data/db" | vault "password"}   → "s3cr3t"
```

### `env` / `expandenv`
Read a process env var / expand `$VAR` references. Gated behind `KERAMOS_RENDER_ENV=1` (off by default — reading the operator's environment from an untrusted chart is a secret-exfiltration risk).
```
${"PATH" | env}   → "/usr/bin:…" (requires KERAMOS_RENDER_ENV=1)
```

### `getHostByName`
`getHostByName(name)` → string — resolve a hostname to its first IP (gated behind `KERAMOS_RENDER_NETWORK=1`).
