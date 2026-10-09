---
title: "Math and Logic functions"
nav_order: 3
parent: "Functions"
grand_parent: "Templates"
---
{% raw %}
# Math and Logic functions

> **Pipeline convention:** `${value | f x}` = `f(value, x)`.
>
> **Numeric result type:** math results normalize through `numericResult` — a whole number returns as `int64`, otherwise `float64`. So `add 2 3` → `5`, `div 10 4` → `2.5`.
>
> **Operand coercion:** the piped value is parsed with `toFloat` (accepts int/int64/float32/float64 and numeric **strings**, but **not** booleans). Trailing args use `coerceFloat` (also accepts int32 and `bool`: `true`→1, `false`→0). So `${1 | add true}` → `2`, but `${add true 1}` errors. Non-numeric input errors.

## Math functions

### `add`
`add(a, ...n)` → number — sum (identity `0`).

**Examples**
```
${add 2 3}       → 5
${1.5 | add 2}   → 3.5
${add 10 -4 1}   → 7
```

### `sub`
`sub(a, ...n)` → number — subtracts each arg from `a`; a single operand negates it.

**Examples**
```
${sub 10 3}      → 7
${10 | sub 3 2}  → 5
${sub 5}         → -5
```

### `mul`
`mul(a, ...n)` → number — product (identity `1`).

**Examples**
```
${mul 2 3}       → 6
${2 | mul 3 4}   → 24
```

### `div`
`div(a, ...n)` → number — floating-point division by each divisor. Errors `div requires at least one divisor` or `div: division by zero`.

**Examples**
```
${div 10 2}      → 5
${10 | div 3}    → 3.3333333333333335
${div 12 2 3}    → 2
${div 10 0}      → error: div: division by zero
```

### `mod`
`mod(a, b)` → number — `math.Mod` remainder of exactly two operands (keeps dividend sign). Errors `mod requires exactly two operands` / `mod: division by zero`.

**Examples**
```
${mod 10 3}      → 1
${mod 10.5 3}    → 1.5
${mod -10 3}     → -1
```

### `max`
`max(a, ...n)` → number — largest operand.

**Examples**
```
${max 3 7 2}     → 7
${5 | max 9}     → 9
```

### `min`
`min(a, ...n)` → number — smallest operand.

**Examples**
```
${min 3 7 2}     → 2
```

### `floor`
`floor(a)` → int — round down (always `int64`).

**Examples**
```
${floor 3.7}     → 3
${-3.2 | floor}  → -4
```

### `ceil`
`ceil(a)` → int — round up (always `int64`).

**Examples**
```
${ceil 3.2}      → 4
${-3.7 | ceil}   → -3
```

### `round`
`round(a, digits?)` → number — round half away from zero; to `digits` decimals if given. Errors `round: invalid digit count` on a bad `digits`.

**Examples**
```
${round 3.5}         → 4
${round 3.14159 2}   → 3.14
${round -2.5}        → -3
```

### `abs`
`abs(a)` → number — absolute value.

**Examples**
```
${abs -5}        → 5
${-4.5 | abs}    → 4.5
```

## Logic functions

> **Two notions of "blank":**
> - **Emptiness** (`isEmpty`, used by `default`/`required`/`empty`/`omitempty`): `nil`, `""`, `false`, numeric `0`, empty list/map. Any other value (including `"false"`, `"0"`) is **not** empty.
> - **Truthiness** (`isTruthy`, used only by `ternary`): `nil` false; bools themselves; numbers true when non-zero; lists/maps true when non-empty; the strings `""`, `"false"`/`"False"`/`"FALSE"`, `"0"`, `"no"`/`"No"`/`"NO"` are false, every other string true.

### `default`
`default(value, fallback)` → any — returns `fallback` when `value` is empty. Errors without a fallback.

**Examples**
```
${"" | default "n/a"}   → n/a
${0 | default 5}        → 5
${false | default "y"}  → y
${"hi" | default "n/a"} → hi
```

### `required`
`required(value, message?)` → any — passes `value` through, or errors (with optional custom message) if empty.

**Examples**
```
${"prod" | required}                → prod
${"" | required}                    → error: value is required
${"" | required "name is required"} → error: name is required
```

### `empty`
`empty(value)` → bool — reports emptiness.

**Examples**
```
${empty ""}       → true
${"false" | empty} → false
${0 | empty}      → true
```

### `ternary`
`ternary(cond, trueVal, falseVal)` → any — `trueVal` when `cond` is truthy, else `falseVal`. Errors without both values.

**Examples**
```
${true | ternary "yes" "no"}   → yes
${"false" | ternary "a" "b"}   → b
${"hi" | ternary "a" "b"}      → a
```

### `eq`
`eq(a, b, …)` → bool — `true` when every argument equals `a`. Numbers compare by value across int/float; `nil` only equals `nil`.

**Examples**
```
${values.env | eq "prod"}     → true   (values.env = prod)
${eq 1 1.0}                   → true
${eq "a" "a" "b"}             → false
```

### `ne`
`ne(a, b)` → bool — the negation of `eq`.

**Examples**
```
${values.env | ne "prod"}     → false  (values.env = prod)
${ne 1 2}                     → true
```

### `lt`, `le`, `gt`, `ge`
`lt(a, b)` → bool — `a < b`; `le` is `<=`, `gt` is `>`, `ge` is `>=`. Numbers compare numerically, anything else compares as strings. Exactly one argument.

**Examples**
```
${values.replicas | gt 1}     → true   (replicas = 3)
${lt 2 10}                    → true
${ge "b" "a"}                 → true
${le 3 3}                     → true
```

### `and`
`and(a, b, …)` → any — the first falsy operand, or the last operand when all are truthy (Go-template semantics, not a strict bool).

**Examples**
```
${and true "x"}               → x
${and values.enabled 0}       → 0      (enabled = true)
${and "" "x"}                 → ""
```

### `or`
`or(a, b, …)` → any — the first truthy operand, or the last operand when none is.

**Examples**
```
${or "" "fallback"}           → fallback
${or values.tag "latest"}     → latest (tag unset)
${or false 0}                 → 0
```

### `not`
`not(value)` → bool — `true` when `value` is falsy.

**Examples**
```
${not false}                  → true
${values.enabled | not}       → false  (enabled = true)
${not ""}                     → true
```

### `omitempty`
`omitempty(value)` → any | (omitted) — returns `value`, or drops its containing map key / slice element when empty (emits the omit sentinel, so no `key: null`).

**Examples**
```
# field: ${values.optional | omitempty}
#   present → renders `field: <value>`
#   empty   → the `field:` key disappears
```
{% endraw %}
