# Function reference

Keramos's expression engine ships ~180 functions covering strings, math, regex, dates, crypto, collections, type conversion, and external integrations. This page is a cookbook: every function has its signature, a one-sentence description, and an input/output snippet showing what goes in and what comes out.

Functions are grouped by category. Within a category, functions that share semantics are documented together with one or more representative examples.

> **Pipeline note.** In the space-call form (`a | f x y`), the value to the *left* of the pipe becomes the **first** argument; trailing tokens are extras. The signatures below name the first argument as `value`. So `${"hello" | upper}` calls `upper("hello")`; `${values.text | replace "old" "new"}` calls `replace(values.text, "old", "new")`.

---

## String

| Function | Signature | Description |
|---|---|---|
| `upper` | `upper(value)` | Uppercase. |
| `lower` | `lower(value)` | Lowercase. |
| `title` | `title(value)` | Title Case Each Word. |
| `untitle` | `untitle(value)` | Lowercase first letter of each word. |
| `swapCase` | `swapCase(value)` | Swap upper↔lower. |
| `trim` | `trim(value)` | Trim leading and trailing whitespace. |
| `trimPrefix` | `trimPrefix(value, prefix)` | Remove prefix if present. |
| `trimSuffix` | `trimSuffix(value, suffix)` | Remove suffix if present. |
| `replace` | `replace(value, old, new)` | Substring replace. |
| `quote` | `quote(value)` | Wrap in double quotes. |
| `squote` | `squote(value)` | Wrap in single quotes. |
| `indent` | `indent(value, n)` | Prefix every line with N spaces. |
| `nindent` | `nindent(value, n)` | Like indent, with a leading newline. |
| `trunc` | `trunc(value, n)` | Truncate to N runes. |
| `repeat` | `repeat(value, n)` | Repeat the string N times. |
| `contains` | `contains(value, substr)` | Substring test → bool. |
| `hasPrefix` | `hasPrefix(value, prefix)` | Prefix test → bool. |
| `hasSuffix` | `hasSuffix(value, suffix)` | Suffix test → bool. |
| `split` | `split(value, sep)` | Split into a list. |
| `splitN` | `splitN(value, sep, n)` | Split with max-N parts. |
| `substr` | `substr(value, start, end)` | Substring by index. |
| `cat` | `cat(arg1, arg2, ...)` | Concatenate with single spaces. |
| `wrap` | `wrap(value, n)` | Soft-wrap at column N. |
| `wrapWith` | `wrapWith(value, n, separator)` | Wrap using a custom separator. |
| `nospace` | `nospace(value)` | Remove all whitespace. |

### `upper` / `lower` / `title`

**Input:**

```yaml
a: ${"hello world" | upper}
b: ${"HELLO WORLD" | lower}
c: ${"hello world" | title}
```

**Output:**

```yaml
a: HELLO WORLD
b: hello world
c: Hello World
```

### `replace`

**Input:**

```yaml
out: ${replace "_" "-" values.name}
```

**Values: `{ name: my_app_v2 }`**

**Output:**

```yaml
out: my-app-v2
```

### `indent` / `nindent`

**Input:**

```yaml
data:
  config: |
    ${values.config | toYaml | nindent 4}
```

**Values:**

```yaml
config:
  log: stdout
  port: 8080
```

**Output:**

```yaml
data:
  config: |
    
    log: stdout
    port: 8080
```

(`nindent 4` adds a newline + 4-space indent on subsequent lines.)

### `trunc`, `quote`, `cat`

**Input:**

```yaml
a: ${trunc "abcdefghij" 5}
b: ${quote values.password}
c: ${cat values.firstName values.lastName}
```

**Values: `{ password: "p@ss", firstName: "Ada", lastName: "Lovelace" }`**

**Output:**

```yaml
a: abcde
b: "p@ss"
c: Ada Lovelace
```

### `contains` / `hasPrefix` / `hasSuffix`

**Input:**

```yaml
a: ${contains "image:1.0" ":"}
b: ${hasPrefix "kube-system" "kube-"}
c: ${hasSuffix "config.yaml" ".yaml"}
```

**Output:**

```yaml
a: true
b: true
c: true
```

### `split`

**Input:**

```yaml
out: ${split "," values.csvList}
```

**Values: `{ csvList: "a,b,c" }`**

**Output:**

```yaml
out:
  - a
  - b
  - c
```

---

## Case conversion

| Function | Signature | Example |
|---|---|---|
| `camelCase` | `camelCase(value)` | `my_var` → `myVar` |
| `kebabCase` | `kebabCase(value)` | `myVar` → `my-var` |
| `snakeCase` | `snakeCase(value)` | `myVar` → `my_var` |
| `swapCase` | `swapCase(value)` | `Hello` → `hELLO` |
| `initials` | `initials(value)` | `Ada Lovelace` → `AL` |

**Input:**

```yaml
a: ${camelCase "my_app_name"}
b: ${kebabCase "myAppName"}
c: ${snakeCase "myAppName"}
d: ${initials "Hello World Foo"}
```

**Output:**

```yaml
a: myAppName
b: my-app-name
c: my_app_name
d: HWF
```

---

## Math

| Function | Signature |
|---|---|
| `add` | `add(a, b)` |
| `sub` | `sub(a, b)` |
| `mul` | `mul(a, b)` |
| `div` | `div(a, b)` |
| `mod` | `mod(a, b)` |
| `min` | `min(a, b)` |
| `max` | `max(a, b)` |
| `floor` | `floor(value)` |
| `ceil` | `ceil(value)` |
| `round` | `round(value)` |
| `abs` | `abs(value)` |
| `addf` | `addf(a, b)` (forces float arithmetic) |
| `subf` | `subf(a, b)` |
| `mulf` | `mulf(a, b)` |
| `divf` | `divf(a, b)` |
| `randInt` | `randInt(min, max)` (inclusive) |
| `until` | `until(n)` → `[0, 1, ..., n-1]` |
| `untilStep` | `untilStep(start, end, step)` → list |

Math functions follow the same constraint as every other function: arguments are literals; only the value to the **left** of `|` is a path lookup. So `${add 2 3}` works (both literals), and `${values.x | mul 2}` works (path on left, literal on right). `${add values.a values.b}` does **not** work — both arguments would be parsed as the literal strings `"values.a"` and `"values.b"`.

**Input:**

```yaml
sum-literal: ${add 2 3}
double:      ${values.x | mul 2}
half:        ${values.x | div 2}
mod3:        ${values.x | mod 3}
floor:       ${3.7 | floor}
range:       ${until 5}
step:        ${untilStep 0 10 2}
```

**Values:** `{ x: 100 }`

**Output:**

```yaml
sum-literal: 5
double:      200
half:        50
mod3:        1
floor:       3
range:       [0, 1, 2, 3, 4]
step:        [0, 2, 4, 6, 8]
```

For two-path arithmetic (e.g. `replicas * shards`), pre-compute the result at value-authoring time:

```yaml
# values.yaml
replicas: 3
shards: 4
totalWorkers: 12     # = replicas * shards, computed by hand or by a script

# template
spec:
  parallelism: ${values.totalWorkers}
```

---

## Logic

Keramos's runtime exposes five logic functions: `default`, `required`, `empty`, `ternary`, `omitempty`. There is no `eq`, `ne`, `lt`, `gt`, `and`, `or`, `not`, or `coalesce` at the expression level — those compositions belong in the YAML control-flow directives (`$if`, `$switch`) and in the values themselves (a discriminator field rather than a runtime comparison). See [Expression syntax](expressions.md#truthy-if-evaluation) for the rationale.

| Function | Signature | Description |
|---|---|---|
| `default` | `value | default fallback` | return value if truthy/non-empty, else `fallback` |
| `required` | `value | required message` | error with message if value is nil/empty |
| `empty` | `value | empty` | true if nil/false/0/empty-string/empty-list/empty-map |
| `ternary` | `condition | ternary ifTrue ifFalse` | return `ifTrue` if condition is truthy, else `ifFalse` |
| `omitempty` | `value | omitempty` | drop the surrounding map key / slice element when value is empty (instead of rendering `null`) |

**Input:**

```yaml
a: ${values.tag | default 'latest'}
b: ${values.maybe | empty}
c: ${values.flag | ternary 'y' 'n'}
```

**Values:** `{ tag: '', maybe: '', flag: true }`

**Output:**

```yaml
a: latest
b: true
c: y
```

`required` aborts the render rather than returning a value:

```yaml
sa: ${values.serviceAccountName | required 'serviceAccountName must be set'}
```

For string-equality conditionals, use `$switch` in YAML control flow instead:

```yaml
$switch: ${values.env}
$cases:
  prod:    { resources: { requests: { cpu: 500m } } }
  staging: { resources: { requests: { cpu: 200m } } }
$default:  { resources: { requests: { cpu: 100m } } }
```

For "feature X is enabled", use a flag in values and `$if`:

```yaml
$if: ${values.metrics.enabled}
ports:
  - { containerPort: 9100, name: metrics }
```

For "between A and B" range checks, restructure as a single boolean in values, or use `$if` chains.

---

## Collections

### List inspection

| Function | Description |
|---|---|
| `len` | length of string/list/map |
| `first` | first element |
| `last` | last element |
| `initial` | all but the last |
| `rest` | all but the first |
| `reverse` | reversed list |
| `uniq` | deduplicate (preserves order) |
| `sortAlpha` | alphabetical sort |
| `compact` | drop empty/nil entries |
| `has` | `has(item, list)` → bool |
| `without` | `without(list, item, ...)` — drop named items |
| `pluck` | `pluck(key, listOfMaps)` — gather one field from each |

**Input:**

```yaml
n:        ${len values.items}
first:    ${first values.items}
last:     ${last values.items}
init:     ${initial values.items}
rest:     ${rest values.items}
rev:      ${reverse values.items}
uniq:     ${uniq values.dups}
sorted:   ${sortAlpha values.unsorted}
hasC:     ${has "c" values.items}
without:  ${without values.items "b"}
names:    ${pluck "name" values.users | toYaml | nindent 2}
```

**Values:**

```yaml
items: [a, b, c]
dups:  [a, b, a, c, b]
unsorted: [c, a, b]
users:
  - { name: ada,  age: 36 }
  - { name: alan, age: 41 }
```

**Output:**

```yaml
n:        3
first:    a
last:     c
init:     [a, b]
rest:     [b, c]
rev:      [c, b, a]
uniq:     [a, b, c]
sorted:   [a, b, c]
hasC:     true
without:  [a, c]
names:
  - ada
  - alan
```

### List construction

| Function | Description |
|---|---|
| `tuple` / `list` | `tuple(a, b, c)` → `[a, b, c]` |
| `append` / `mustAppend` | append item |
| `prepend` / `mustPrepend` | prepend item |
| `concat` | join multiple lists |
| `slice` / `mustSlice` | sub-slice |
| `chunk` | split into N-sized chunks |
| `seq` | numeric range |

Like all keramos functions, list-builder calls take literal arguments; to use a path's value, put it on the **left** of the pipe.

**Input:**

```yaml
t:      ${tuple 'a' 'b' 'c'}
app:    ${values.items | append 'z'}
pre:    ${values.items | prepend '0'}
chunks: ${values.items | chunk 2 | toYaml | nindent 2}
seq:    ${seq 1 5}
```

**Values:**

```yaml
items: [a, b, c]
```

**Output:**

```yaml
t:      [a, b, c]
app:    [a, b, c, z]
pre:    [0, a, b, c]
chunks:
  - [a, b]
  - [c]
seq:    [1, 2, 3, 4, 5]
```

`concat` joins multiple lists, but every list-arg must be a literal — useful for combining a path-resolved list with a small ad-hoc one via `append`/`prepend` rather than for two-path concatenation.

### Map operations

| Function | Description |
|---|---|
| `keys` | sorted list of keys |
| `values` | list of values |
| `dict` | `dict(k1, v1, k2, v2, ...)` |
| `hasKey` | `hasKey(map, key)` |
| `pick` | `pick(map, k1, k2, ...)` — subset |
| `omit` | `omit(map, k1, k2, ...)` — complement |
| `merge` | merge maps; **first** map's values win on collision |
| `mergeOverwrite` | merge maps; **last** map's values win |
| `set` | `set(map, key, value)` — return new map with key set |
| `unset` | `unset(map, key)` — return new map with key removed |
| `get` | `get(map, key, default)` — safe lookup |
| `dig` | `dig(map, k1, k2, ..., default)` — deep lookup |
| `deepCopy` / `mustDeepCopy` | full clone |
| `deepEqual` | structural equality |

**Input:**

```yaml
keys:    ${values.config | keys}
hasA:    ${values.config | hasKey 'a'}
sub:     ${values.config | pick 'a' 'c'}
nob:     ${values.config | omit 'b'}
deep:    ${values.nested | dig 'a' 'b' 'c' 'default'}
```

**Values:**

```yaml
config: { a: 1, b: 2, c: 3 }
nested:
  a:
    b:
      c: deep-found
```

**Output:**

```yaml
keys:    [a, b, c]
hasA:    true
sub:     { a: 1, c: 3 }
nob:     { a: 1, c: 3 }
deep:    deep-found
```

For two-map merging, keramos's expression engine cannot directly compose two paths. The merge semantics live at the **values layer** instead: keramos merges `values.yaml` files from layers and the parent automatically (see [Values guide](../guides/values.md)), and the parent's `layers.<name>` block lets the parent override individual keys of a layer's contributions. `merge` and `mergeOverwrite` exist as runtime functions but only over a single resolved path piped in:

```yaml
${values.specificMap | merge values.fallback}    # ✗ values.fallback is literal
${values.specificMap | toYaml | nindent 2}       # ✓ for simple emission
```

---

## Type conversion

| Function | Description |
|---|---|
| `toString` | any → string |
| `toInt` | any → int |
| `int` / `int64` | parse to int / int64 |
| `float64` | parse to float |
| `toBool` | "true"/"false"/1/0 → bool |
| `toJson` / `toRawJson` / `mustToJson` / `mustToRawJson` | encode as JSON |
| `toYaml` | encode as YAML |
| `fromJson` / `mustFromJson` | parse JSON |
| `fromYaml` / `mustFromYaml` | parse YAML |
| `fromYamlArray` / `mustFromYamlArray` | parse YAML array form |
| `kindOf` | string name of the type |
| `kindIs` | `kindIs("string", value)` — type check |

**Input:**

```yaml
toStr:  ${toString values.replicas}
toInt:  ${toInt "42"}
fjson:  ${fromJson values.json | toYaml | nindent 2}
kind:   ${kindOf values.thing}
isStr:  ${kindIs "string" values.thing}
```

**Values:**

```yaml
replicas: 3
json: '{"a": 1, "b": [2, 3]}'
thing: hello
```

**Output:**

```yaml
toStr:  "3"
toInt:  42
fjson:
  a: 1
  b: [2, 3]
kind:   string
isStr:  true
```

---

## Date and time

| Function | Description |
|---|---|
| `now` | current time as Time |
| `date` | `date(format, time)` — strftime-style |
| `dateInZone` | `dateInZone(format, time, "UTC")` |
| `dateModify` / `mustDateModify` | `dateModify("+24h", time)` |
| `htmlDate` / `htmlDateInZone` | RFC3339-shaped |
| `toDate` / `mustToDate` | parse string → Time |
| `ago` | duration since |

**Input:**

```yaml
ts:   ${now | date "%Y-%m-%d"}
utc:  ${dateInZone "%Y-%m-%dT%H:%M:%SZ" now "UTC"}
soon: ${now | dateModify "+24h" | date "%Y-%m-%d"}
```

**Output (at 2026-05-08 14:32 local):**

```yaml
ts:   "2026-05-08"
utc:  "2026-05-08T12:32:00Z"
soon: "2026-05-09"
```

Format tokens are strftime-compatible: `%Y` (4-digit year), `%m` (month), `%d` (day), `%H` (24h hour), `%M` (minute), `%S` (second), `%j` (day of year), `%a` / `%A` (weekday short/long), `%b` / `%B` (month short/long), `%Z` (timezone name).

---

## Crypto

| Function | Description |
|---|---|
| `sha1sum` / `sha256sum` / `sha512sum` / `md5sum` / `adler32sum` | hash a string → hex |
| `hmacSha256` | `hmacSha256(message, key)` → hex |
| `bcrypt` | `bcrypt(password)` → bcrypt hash |
| `htpasswd` | `htpasswd(user, password)` → htpasswd line |
| `encryptAES` / `decryptAES` | AES-256-CBC with PKCS#7 |
| `genPrivateKey` | `genPrivateKey("rsa")` / `"ecdsa"` / `"ed25519"` |
| `genCA` | `genCA(cn, expiryDays)` — return CA cert + key |
| `genSelfSignedCert` | `genSelfSignedCert(cn, ips, dns, expiryDays)` |
| `genSignedCert` | `genSignedCert(cn, ips, dns, expiryDays, ca)` |
| `randAlphaNum` / `randAlpha` / `randNumeric` / `randAscii` / `randBytes` | random strings of length N |
| `uuidv4` | random UUID |
| `derivePassword` | deterministic password derivation (PBKDF2-style) |

**Input:**

```yaml
sha:    ${sha256sum values.data}
pass:   ${randAlphaNum 32}
uuid:   ${uuidv4}
hash:   ${bcrypt values.password}
```

**Values: `{ data: "hello", password: "secret" }`**

**Output (deterministic for sha; random for others):**

```yaml
sha:  2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
pass: <32 chars>
uuid: <RFC-4122 UUID>
hash: <60-char bcrypt $2a$...>
```

### Generating a CA + leaf cert

```yaml
${include "genCert"}
```

```yaml
# _helpers.yaml
genCert:
  ${$ca: $genCA "ca" 3650}
  cert: ${$genSignedCert (printf "%s.%s.svc" release.name release.namespace) (list) (list (printf "%s.%s.svc" release.name release.namespace) (printf "%s.%s.svc.cluster.local" release.name release.namespace)) 365 $ca}
```

In practice, packages use the simpler form per-resource:

**Input:**

```yaml
data:
  ca.crt:    ${$ca := genCA "my-ca" 365 | toJson; $ca.Cert | b64enc}
  tls.crt:   ${(genSignedCert "my-app" (list "127.0.0.1") (list "my-app.default.svc") 365 ($ca := fromJson $caJson)) | toJson | b64enc}
  tls.key:   ...
```

(In real packages, expressions are split across `_helpers.yaml` partials for readability.)

---

## Encoding

| Function | Description |
|---|---|
| `b64enc` / `b64dec` | base64 |
| `b32enc` / `b32dec` | base32 |
| `urlquery` | URL-query escape |
| `urlParse` / `urlJoin` | `urlParse(url)` → struct; `urlJoin(struct)` → url |

**Input:**

```yaml
b64:   ${b64enc "hello"}
b32:   ${b32enc "hello"}
qs:    ${urlquery "a b/c?"}
url:   ${urlParse "https://user:pass@host:8443/path?q=1" | toYaml | nindent 2}
```

**Output:**

```yaml
b64: aGVsbG8=
b32: NBSWY3DP
qs:  a+b%2Fc%3F
url:
  scheme: https
  user: user
  password: pass
  host: host
  port: "8443"
  path: /path
  rawquery: q=1
```

---

## Regex

| Function | Description |
|---|---|
| `regexMatch` | `regexMatch(value, pattern)` → bool |
| `regexFind` | first match string |
| `regexFindAll` | `regexFindAll(value, pattern, n)` — list of matches, n=−1 for all |
| `regexReplaceAll` | `regexReplaceAll(value, pattern, replacement)` |
| `regexSplit` | `regexSplit(value, pattern, n)` — list |
| `regexQuoteMeta` | escape regex metacharacters |
| `mustRegexMatch` / `mustRegexFind` / etc. | error on regex compile failure (others return empty) |

Regexes are RE2 (Go's regexp); no PCRE features like backreferences.

**Input:**

```yaml
isHost:  ${regexMatch "^[a-z][a-z0-9-]+\\.example\\.com$" values.host}
domain:  ${regexFind "[^@]+@(.+)" values.email | regexReplaceAll "[^@]+@" ""}
parts:   ${regexSplit "[\\s,]+" values.csv -1}
```

**Values:**

```yaml
host: api.example.com
email: ada@example.org
csv: "a, b   c,d"
```

**Output:**

```yaml
isHost:  true
domain:  example.org
parts:   [a, b, c, d]
```

---

## Path

| Function | Description |
|---|---|
| `base` / `dir` / `clean` / `ext` / `isAbs` | POSIX-style path manipulation |
| `osBase` / `osDir` / `osClean` / `osExt` / `osIsAbs` | OS-specific (handles Windows separators) |

**Input:**

```yaml
b: ${base "/etc/keramos/config.yaml"}
d: ${dir "/etc/keramos/config.yaml"}
e: ${ext "/etc/keramos/config.yaml"}
a: ${isAbs "../foo"}
```

**Output:**

```yaml
b: config.yaml
d: /etc/keramos
e: .yaml
a: false
```

---

## Misc / printf / dict / get / set

| Function | Description |
|---|---|
| `printf` / `sprintf` | Go-format string |
| `dict` | construct map from `k1, v1, k2, v2, ...` |
| `set` | `set(map, key, value)` returns new map |
| `unset` | remove key |
| `get` | `get(map, key, default)` |
| `len` | length |
| `kindOf` / `kindIs` | type introspection |
| `semver` / `semverCompare` | SemVer parse and constraint check |
| `fail` | abort render with message |

### `printf`

`printf` is one of the strictest cases of the literal-args constraint: every argument after the format string is parsed as a literal. To compose a string from multiple paths, build it from YAML structure instead, or use the `cat` function (concatenates with single spaces) or the `+`-style string operations under partials.

**Works (all literals):**

```yaml
greeting: ${printf 'hello, %s!' 'world'}
```

**Output:**

```yaml
greeting: 'hello, world!'
```

**Doesn't work directly (path arguments):**

```yaml
name: ${printf '%s-%s-%d' release.name values.role release.revision}
# Output: name: 'release.name-values.role-release.revision'    ← literal strings
```

**Use YAML structure instead:**

```yaml
metadata:
  name: ${release.name}-${values.role}-${release.revision}
```

That works because each `${...}` expression is independent and resolves its own path; the YAML scalar concatenates them.

### `dict`

`dict(k1, v1, k2, v2, ...)` constructs a map. Like `printf`, it takes literal arguments — useful for inline lookups but not for building a map from values. For building a map from values, prefer literal YAML.

**Input:**

```yaml
data:
  inline: ${dict 'host' 'db.example.com' 'port' 5432 | toYaml | nindent 4}
```

**Output:**

```yaml
data:
  inline:
    host: db.example.com
    port: 5432
```

For maps composed from values, write the map literally in YAML and let `${...}` substitute leaves:

```yaml
data:
  config: |
    ${values.config | toYaml | nindent 4}
```

(`values.config` is already the map — no `dict` needed.)

**Output:**

```yaml
data:
  config:
    host: db.example.com
    port: 5432
    user: app
```

### `semverCompare`

**Input:**

```yaml
ok:  ${semverCompare ">=1.27.0" capabilities.kubeVersion.Version}
old: ${semverCompare "<1.20.0" capabilities.kubeVersion.Version}
```

**On a 1.28.0 cluster:**

```yaml
ok:  true
old: false
```

### `fail`

**Input:**

```yaml
$if: ${not values.serviceAccount}
data: ${fail "serviceAccount is required for production environments"}
```

`fail` aborts the render with the supplied message — useful inside `$if` blocks for cross-field validation that schemas can't express.

---

## Engine functions: `include`, `tpl`, `lookup`, `Files.*`

These are bound at render time and depend on the package's environment.

### `include`

Splice a partial as a string. Inverse of the YAML `$include:` directive.

**`templates/_helpers.yaml`:**

```yaml
fullname: ${printf "%s-%s" release.name package.name | trunc 63 | trimSuffix "-"}
```

**Template:**

```yaml
metadata:
  name: ${include "fullname"}
```

**Output:**

```yaml
metadata:
  name: hello-my-app
```

### `tpl`

Render a template string from values (templates within templates).

**Values:**

```yaml
greeting: "Hello, ${values.name}!"
name: world
```

**Template:**

```yaml
data:
  greeting: ${tpl values.greeting}
```

**Output:**

```yaml
data:
  greeting: Hello, world!
```

### `lookup`

Read a live cluster resource. Returns `nil` for missing resources (no error) so templates can guard. Returns the resource as a map; for cluster-wide list, pass empty `name`.

```yaml
${lookup "v1" "ConfigMap" "kube-system" "cluster-info"}
${lookup "v1" "Pod" "default" ""}                       # list pods in default
```

**Pattern:** read a config-bearing ConfigMap that may not exist yet:

```yaml
$if: ${lookup "v1" "ConfigMap" release.namespace "external-config"}
data:
  loadedFrom: ${(lookup "v1" "ConfigMap" release.namespace "external-config").data.value}
```

The first `$if` evaluates falsy when the ConfigMap is missing, so the body never runs.

### `Files.Get`

Embed a file from `files/`.

**`files/default.conf`:**

```
log = stdout
port = 8080
```

**Template:**

```yaml
data:
  default.conf: |
    ${Files.Get "default.conf" | indent 4}
```

**Output:**

```yaml
data:
  default.conf: |
    log = stdout
    port = 8080
```

### `Files.Glob`

```yaml
data:
  $each: ${Files.Glob "configs/*.conf"}
  $as: f
  $yield:
    ${$f.name}: ${$f.contents | quote}
```

Each iteration's `$f.name` is the basename, `$f.contents` is the file body.

### `Files.AsConfig` / `Files.AsSecrets`

Bulk-mount a directory:

**Template:**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${release.name}-configs
data:
  ${Files.AsConfig "configs" | nindent 2}
```

`AsConfig` produces `<basename>: <contents>` pairs (no transformation); `AsSecrets` base64-encodes the contents (suitable for `Secret.data`).

### `Files.Lines`

```yaml
${Files.Lines "list.txt"}        # → list of strings, one per line
```

---

## External integrations

### `http` and `httpJson`

Perform an HTTP GET (or other method) at render time.

```yaml
${http "https://api.example.com/health"}
${httpJson "https://api.example.com/data"}     # parse response as JSON
```

**Returns** the response body as a string (or parsed map for `httpJson`). Templates that depend on `http` are non-deterministic — the same package can render differently depending on what the URL returns. Use sparingly, prefer `lookup` for cluster-resident data.

### `vault`

Read a secret from HashiCorp Vault.

```yaml
${vault "secret/data/myapp/config" "username"}
```

Requires `VAULT_ADDR` and `VAULT_TOKEN` (or KUBERNETES_SERVICE_HOST + a configured ServiceAccount token) in the environment. The function caches reads per-render so multiple uses don't re-fetch.

### `sops` / `sopsKey`

Decrypt a SOPS-encrypted file or read a single key.

```yaml
${sops "secrets.enc.yaml"}                   # whole file as map
${sopsKey "secrets.enc.yaml" "database.password"}    # specific key
```

Keramos invokes the local `sops` binary; the encryption keys (PGP, age, KMS) come from the standard SOPS configuration (`.sops.yaml`).

### `externalSecret`

Render a Kubernetes `ExternalSecret` resource referencing a backend secret.

```yaml
${externalSecret "my-cluster-secret" "secret/data/myapp" "database.password"}
```

Returns a manifest fragment suitable for inclusion under `templates/`.

### `sealedSecret`

Encrypt a value with the cluster's bitnami-sealed-secrets controller key (requires the public key file to be set via `KERAMOS_SEALED_SECRETS_PUBKEY`).

```yaml
${sealedSecret "ns" "name" "value"}
```

---

## Sprig parity additions

Keramos implements the long tail of Sprig functions for compatibility:

| Function | Description |
|---|---|
| `env` | `env "VAR"` — read process env var |
| `expandenv` | `expandenv "$VAR/sub"` — expand env vars in a string |
| `getHostByName` | DNS lookup → IP |
| `mustToJson` / `mustFromJson` / `mustToRawJson` / `mustToDate` | error-returning variants |
| `mustRegexMatch` / `mustRegexFind` / `mustRegexFindAll` / `mustRegexReplaceAll` / `mustRegexSplit` | error-returning regex |
| `mustAppend` / `mustPrepend` / `mustSlice` / `mustReverse` | error-returning collection ops |
| `urlquery` / `urlParse` / `urlJoin` | URL handling |
| `regexQuoteMeta` | escape regex |

Sprig users find every function they expect by the same name (some keramos-side improvements are noted as differences in the function table above; the most common one is that keramos's `printf` accepts typed values not stringified ones).

---

## See also

- [Expression syntax](expressions.md) — how to call these functions inside `${...}`.
- [Control flow](control-flow.md) — `$if`, `$each`, `$switch`, `$include`.
- [Capabilities](capabilities.md) — the `capabilities` namespace and `lookup`.
- [Hooks](hooks.md) — directives in template manifests.
