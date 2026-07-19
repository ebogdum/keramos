# keramos config

`keramos config` walks a package's `values.schema.json` interactively and writes
the answers to a values file.

## When to use it

- Produce a values file for a package without hand-editing YAML.
- Discover a package's configurable fields by being prompted for each one, with
  the schema's type, description, and default shown inline.

## What happens

1. Reads `<package-path>/values.schema.json` (required; the command fails
   without it).
2. Prompts for each property in sorted key order. The prompt shows the type,
   the `description`, the `default` in brackets, and `*` for required keys.
   Prompts are written to stderr.
3. Press Enter to accept the shown default; type a value to override it. Input
   is validated against the property type (integer, number, boolean, string).
4. Marshals the collected values to YAML and writes them to `--out`, or to
   stdout when `--out` is `-`.

No cluster is contacted.

## Usage

```
keramos config <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-o, --out` | string | "-" | write the values file here; `-` prints to stdout |

## Worked example

**INPUT** — `webapp/values.schema.json`:

```json
{
  "type": "object",
  "required": ["name"],
  "properties": {
    "name":     {"type": "string",  "description": "release name"},
    "replicas": {"type": "integer", "description": "pod count", "default": 2},
    "debug":    {"type": "boolean", "default": false}
  }
}
```

**Session** (`keramos config webapp`) — prompts on stderr, your input after each:

```
debug (boolean) [false]:                    ⏎   (kept the default)
name (string) — release name *: orders
replicas (integer) — pod count [2]: 5
```

You accepted the `debug` default, typed `orders` for the required `name`, and
overrode `replicas` with `5`.

**OUTPUT** (stdout):

```yaml
debug: false
name: orders
replicas: 5
```

`debug: false` is the schema default you kept; `name` and `replicas` are the
values you entered. Add `-o values.yaml` to write the file instead of printing
it.

## See also

- [`values`](values.md) — resolve and trace the merged values
- [`show values`](show-values.md) — print the package's default `values.yaml`
- [`install`](install.md) — install using a values file (`-f`)
