# Template function reference

keramos's expression engine ships ~200 built-in functions. Every function is
documented with its signature and an **input → output** example, split into
category pages so you can find what you need fast.

**Pipeline convention.** In `${value | f x y}` the value to the left of the pipe
is the function's **first** argument, so it calls `f(value, x, y)`. For example
`${"hello" | upper}` calls `upper("hello")` → `"HELLO"`, and
`${values.text | replace "old" "new"}` calls `replace(values.text, "old", "new")`.

## Categories

| Page | Covers |
|---|---|
| [String](functions/string.md) | `upper`, `lower`, `trim`, `replace`, `quote`, `indent`, `trunc`, `camelcase`, `kebabcase`, `snakecase`, `initials`, … |
| [Math & logic](functions/math-logic.md) | `add`, `sub`, `mul`, `div`, `mod`, `max`, `min`, `round`, `default`, `required`, `ternary`, `omitempty`, … |
| [Collection & type](functions/collection-type.md) | `keys`, `values`, `first`, `last`, `join`, `uniq`, `compact`, `has`, `toYaml`, `toJson`, `toInt`, `toBool`, … |
| [Date & encoding](functions/date-encoding.md) | `now`, `date`, `dateInZone`, `toDate`, `ago`, `b64encode`, `b64decode`, `sha256`, … |
| [Crypto & secrets](functions/crypto-secrets.md) | `sha256sum`, `bcrypt`, `genPrivateKey`, `genCA`, `randAlphaNum`, `uuidv4`, `sops`, `externalSecret`, `sealedSecret`, … |
| [Regex, path & misc](functions/regex-path-misc.md) | `regexMatch`, `regexReplaceAll`, `base`, `dir`, `ext`, `urlParse`, `dict`, `merge`, `pick`, `semver`, `coalesce`, … |
| [Sequence, Sprig & external](functions/sprig-external.md) | `until`, `seq`, `slice`, `dig`, `chunk`, `fromJson`, `http`, `httpJSON`, `vault`, `env`, the `must*` variants, … |

## How to read an entry

```
### `funcName`
`funcName(value, arg) → returnType`

One-line description of what it does.

${"input" | funcName "arg"}   → "output"
```

The expression is on the left of the `→`, the result on the right. When a result
varies between runs (timestamps, random keys, network responses), the example
shows its *shape* and is marked `(shape; value varies)`.

## See also

- [Expressions](expressions.md) — the `${...}` syntax, operators, and pipelines
- [CLI reference](../cli/README.md) — the commands that render these templates
