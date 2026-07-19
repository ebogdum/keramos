---
title: "Date and Encoding functions"
nav_order: 4
parent: "Functions"
grand_parent: "Templates"
---
{% raw %}
# Date and Encoding functions

> **Pipeline note.** `${value | f x}` desugars to `f(value, x)` — the piped subject becomes the first argument (`value`), and tokens after the function name become trailing arguments.

## Date functions

Date formatting accepts either a **Go reference layout** (e.g. `2006-01-02`) or a **strftime-style** string containing `%` tokens:

- If the format contains **no** `%`, it is used unchanged (Go reference layout).
- Otherwise each `%X` token is translated to its Go fragment. `%%` emits a literal `%`; unrecognized tokens pass through.

**strftime → Go layout**

| Token | Go | Meaning | Token | Go | Meaning |
|---|---|---|---|---|---|
| `%Y` | `2006` | 4-digit year | `%H` | `15` | hour (24h) |
| `%y` | `06` | 2-digit year | `%I` | `03` | hour (12h) |
| `%m` | `01` | month | `%M` | `04` | minute |
| `%B` | `January` | month name | `%S` | `05` | second |
| `%b`/`%h` | `Jan` | abbrev month | `%p` | `PM` | AM/PM |
| `%d` | `02` | day (0-pad) | `%P` | `pm` | am/pm |
| `%e` | `_2` | day (space-pad) | `%Z` | `MST` | tz abbrev |
| `%a` | `Mon` | abbrev weekday | `%z` | `-0700` | tz offset |
| `%A` | `Monday` | weekday | `%j` | `002` | day of year |

**Time coercion.** A `time.Time` is used as-is; a `string` is parsed trying `RFC3339`, `RFC3339Nano`, `2006-01-02T15:04:05Z`, then `2006-01-02`; anything else errors `expected time.Time or RFC3339 string, got <type>`.

### `now`
`now(value)` → time.Time

Returns the current local time. Ignores `value`, but must be *called* with an argument (a bare `now` identifier evaluates to null); pass any value, e.g. `now ""`.

**Examples**
```
${now "" | date "2006-01-02"}   → 2026-07-19 (shape; value is the current date)
```

### `date`
`date(value, fmt)` → string

Formats time `value` using `fmt` (Go layout or strftime). Errors `date requires a format argument` without `fmt`.

| Param | Type | Description |
|---|---|---|
| `value` | time.Time / RFC3339 / `2006-01-02` string | time to format |
| `fmt` | string | Go layout or strftime |

**Examples**
```
${now "" | date "%Y-%m-%d %H:%M"}             → 2026-07-19 03:29 (shape; varies)
${"2026-07-18T09:30:00Z" | date "Mon Jan 2"} → Sat Jul 18
${"2026-07-18" | date "%A"}                   → Saturday
```

### `dateInZone`
`dateInZone(value, fmt, zone)` → string

Like `date` but converts to IANA `zone` first. Errors without both args, or `dateInZone: invalid zone "<zone>"` on an unknown zone.

| Param | Type | Description |
|---|---|---|
| `value` | time | time to format |
| `fmt` | string | layout |
| `zone` | string | IANA zone (e.g. `America/New_York`, `UTC`) |

**Examples**
```
${"2026-07-18T12:00:00Z" | dateInZone "2006-01-02 15:04 MST" "America/New_York"}
    → 2026-07-18 08:00 EDT
```

### `toDate`
`toDate(value, fmt)` → time.Time

Parses string `value` into a time using layout `fmt`. Errors `toDate: failed to parse "<value>"` on failure.

| Param | Type | Description |
|---|---|---|
| `value` | string | text to parse |
| `fmt` | string | layout describing `value` |

**Examples**
```
${"18/07/2026" | toDate "%d/%m/%Y" | date "%A"}   → Saturday
```

### `ago`
`ago(value)` → string

Elapsed duration since `value` (`time.Since` in Go duration form, e.g. `1h30m0s`).

**Examples**
```
${"2026-07-18T12:00:00Z" | ago}   → 13h29m59.8s (shape; varies)
```

## Encoding functions

All encoding functions stringify `value` with `%v` first and take no extra arguments.

### `b64encode`
`b64encode(value)` → string

Base64-encodes with standard encoding (`=` padding).

**Examples**
```
${"hello" | b64encode}               → aGVsbG8=
${"hello" | b64encode | b64decode}   → hello
```

### `b64decode`
`b64decode(value)` → string

Base64-decodes standard encoding. Errors `b64decode: invalid base64` on bad input.

**Examples**
```
${"aGVsbG8=" | b64decode}   → hello
${"not base64!" | b64decode} → error: b64decode: invalid base64: illegal base64 data at input byte 3
```

### `sha256`
`sha256(value)` → string

SHA-256 digest as a lowercase 64-char hex string.

**Examples**
```
${"hello" | sha256}
    → 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
```
{% endraw %}
