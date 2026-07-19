# String functions

Pipeline convention: in `${value | f x y}` the left side is the FIRST argument, so this calls `f(value, x, y)`. For example `${"hi" | upper}` calls `upper("hi")`.

### `upper`
`upper(value)` → string

Converts the value's string form to upper case.

| Param | Type | Description |
|---|---|---|
| `value` | string | The string to upper-case (non-strings are formatted with `%v` first). |

**Examples**
```
${"Hello" | upper}   → "HELLO"
${"aBc" | upper}     → "ABC"
```

### `lower`
`lower(value)` → string

Converts the value's string form to lower case.

| Param | Type | Description |
|---|---|---|
| `value` | string | The string to lower-case (non-strings are formatted with `%v` first). |

**Examples**
```
${"Hello" | lower}   → "hello"
${"aBc" | lower}     → "abc"
```

### `trim`
`trim(value)` → string

Removes leading and trailing whitespace from the value's string form.

| Param | Type | Description |
|---|---|---|
| `value` | string | The string to trim. |

**Examples**
```
${"  hi  " | trim}   → "hi"
```

### `trimPrefix`
`trimPrefix(value, prefix)` → string

Removes `prefix` from the start of the string if present; otherwise returns the string unchanged. Errors if the prefix argument is missing.

| Param | Type | Description |
|---|---|---|
| `value` | string | The string to operate on. |
| `prefix` | string | The prefix to remove. |

**Examples**
```
${"foobar" | trimPrefix "foo"}   → "bar"
${"foobar" | trimPrefix "xyz"}   → "foobar"
```

### `trimSuffix`
`trimSuffix(value, suffix)` → string

Removes `suffix` from the end of the string if present; otherwise returns the string unchanged. Errors if the suffix argument is missing.

| Param | Type | Description |
|---|---|---|
| `value` | string | The string to operate on. |
| `suffix` | string | The suffix to remove. |

**Examples**
```
${"foobar" | trimSuffix "bar"}   → "foo"
${"foobar" | trimSuffix "xyz"}   → "foobar"
```

### `replace`
`replace(value, old, new)` → string

Replaces every non-overlapping occurrence of `old` with `new`. Errors if fewer than two arguments are supplied.

| Param | Type | Description |
|---|---|---|
| `value` | string | The string to operate on. |
| `old` | string | Substring to search for. |
| `new` | string | Replacement substring. |

**Examples**
```
${"a.b.c" | replace "." "-"}   → "a-b-c"
${"aaa" | replace "a" "bb"}    → "bbbbbb"
```

### `quote`
`quote(value)` → string

Wraps the value's string form in double quotes using Go's `%q` formatting (escaping special characters).

| Param | Type | Description |
|---|---|---|
| `value` | string | The string to quote. |

**Examples**
```
${"Hello" | quote}   → "Hello" (with surrounding double quotes)
```

### `squote`
`squote(value)` → string

Wraps the value's string form in single quotes without any escaping.

| Param | Type | Description |
|---|---|---|
| `value` | string | The string to single-quote. |

**Examples**
```
${"Hello" | squote}   → 'Hello'
```

### `indent`
`indent(value, width)` → string

Prepends `width` spaces to the beginning of every line. Errors if `width` is missing, non-numeric, negative, exceeds 65536, or if the result exceeds the internal output-size cap.

| Param | Type | Description |
|---|---|---|
| `value` | string | The (possibly multi-line) string to indent. |
| `width` | int | Number of leading spaces per line; `0..65536`. |

**Examples**
```
${"a\nb" | indent 2}   → "  a\n  b"
${"x" | indent 0}      → "x"
```

### `nindent`
`nindent(value, width)` → string

Like `indent`, but also prepends a leading newline before the first indented line. Same error conditions as `indent`.

| Param | Type | Description |
|---|---|---|
| `value` | string | The (possibly multi-line) string to indent. |
| `width` | int | Number of leading spaces per line; `0..65536`. |

**Examples**
```
${"a\nb" | nindent 2}   → "\n  a\n  b"
${"x" | nindent 4}      → "\n    x"
```

### `trunc`
`trunc(value, length)` → string

Truncates the string to at most `length` runes; a negative length yields an empty string; a length at or beyond the rune count returns it unchanged. Errors if `length` is missing or non-numeric.

| Param | Type | Description |
|---|---|---|
| `value` | string | The string to truncate. |
| `length` | int | Maximum number of runes to keep. |

**Examples**
```
${"Hello" | trunc 3}    → "Hel"
${"Hello" | trunc 10}   → "Hello"
${"Hello" | trunc -1}   → ""
```

### `camelcase`
`camelcase(value)` → string

Splits into words (on spaces, `_`, `-`, `.`, and lower→upper case boundaries) and joins them camelCase: first word lower-cased, each subsequent word capitalized.

| Param | Type | Description |
|---|---|---|
| `value` | string | The string to convert. |

**Examples**
```
${"hello world" | camelcase}   → "helloWorld"
${"foo_bar-baz" | camelcase}   → "fooBarBaz"
${"HelloWorld" | camelcase}    → "helloWorld"
```

### `kebabcase`
`kebabcase(value)` → string

Splits into words and joins them lower-cased with hyphens.

| Param | Type | Description |
|---|---|---|
| `value` | string | The string to convert. |

**Examples**
```
${"Hello World" | kebabcase}   → "hello-world"
${"fooBar" | kebabcase}        → "foo-bar"
```

### `snakecase`
`snakecase(value)` → string

Splits into words and joins them lower-cased with underscores.

| Param | Type | Description |
|---|---|---|
| `value` | string | The string to convert. |

**Examples**
```
${"Hello World" | snakecase}   → "hello_world"
${"fooBar" | snakecase}        → "foo_bar"
```

### `swapcase`
`swapcase(value)` → string

Swaps the case of each letter; other characters are unchanged.

| Param | Type | Description |
|---|---|---|
| `value` | string | The string to convert. |

**Examples**
```
${"Hello World" | swapcase}   → "hELLO wORLD"
${"aBc123" | swapcase}        → "AbC123"
```

### `initials`
`initials(value)` → string

Returns the first character of each whitespace-separated field, concatenated.

| Param | Type | Description |
|---|---|---|
| `value` | string | The string whose word initials are collected. |

**Examples**
```
${"Hello World Foo" | initials}     → "HWF"
${"john ronald tolkien" | initials} → "jrt"
```
