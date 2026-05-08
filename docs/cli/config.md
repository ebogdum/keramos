# keramos config

## Synopsis

`keramos config` walks a package's `values.schema.json` interactively, prompting for each property in turn. The result is a values file written to the path of your choice (default: `values.local.yaml`). Honours `default`, `enum`, `description`, and `oneOf` discriminated unions in the schema for richer prompts.

## When to use it

Use to materialise a values file for a package you're about to install — particularly for packages with deep schemas where remembering every required field is hard. The interactive flow uses the schema's `description`, `default`, and `enum` annotations to make prompts useful, so authors who maintain a thorough `values.schema.json` get the most benefit.

## What happens when you run it

1. Reads `<package-path>/values.schema.json`.
2. Walks the schema depth-first, prompting for each property.
3. Composes a YAML document from the answers.
4. Writes to `--out` (`-` = stdout).
5. No cluster contact, no validation against the live API.

## Usage

```
keramos config <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for config |
| `-o, --out` | string | "-" | output values file path (- for stdout) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Build values interactively, write to a file:

```sh
keramos config ./my-app -o values.dev.yaml
```

Pipe to stdout (default), then redirect:

```sh
keramos config ./my-app > values.staging.yaml
```

End-to-end: configure, then install with the result:

```sh
keramos config  ./my-app -o values.dev.yaml
keramos install hello ./my-app -f values.dev.yaml -n dev --create-namespace
```

## See also

- [`values`](values.md)
- [Schema validation guide](../guides/schema-validation.md)
- [`values.schema.json` reference](../reference/values-schema-json.md)
