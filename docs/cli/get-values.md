# keramos get values

## Synopsis

`keramos get values` prints the values that were applied to a release. By default it shows just the **user-supplied** values — the inline overrides, value files, and CLI `--set` flags the operator passed at install/upgrade time. With `--all`, it shows the **fully merged** values: package defaults, layer values, environment overrides, profile overlays, and user inputs combined. The merged form is what the templates actually saw during render.

## When to use it

Use to confirm what configuration was applied to a release in production, to audit who-set-what (with `--all` and a careful eye on the diff between the two views), or to recover a values file from a release whose source has been lost. For per-key resolution (which layer or file or flag set each leaf), use `keramos values --trace` against the source package directory instead.

## What happens when you run it

1. Reads the release-storage Secret for `<release-name>` at the requested revision.
2. Extracts either the `userValues` or the merged `values` map (with `--all`).
3. Prints as YAML or JSON.

## Usage

```
keramos get values <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--all` | bool | false | show all merged values, not just user-supplied |
| `-h, --help` | bool | false | help for values |
| `-o, --output` | string | yaml | output format: json, yaml |
| `--revision` | int | 0 | get values from a specific revision (0 = current) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

User-supplied values used by the current revision (typically a small file showing only what the operator overrode):

```sh
keramos get values hello -n prod
```

Full merged values — what the templates actually saw at render time:

```sh
keramos get values hello -n prod --all
```

Values from a historical revision, captured to a file for re-use:

```sh
keramos get values hello --revision 3 -n prod > recovered-values.yaml
keramos install hello-clone ./my-app -f recovered-values.yaml -n staging
```

JSON for scripting:

```sh
keramos get values hello -n prod --all -o json | jq '.image.tag'
```

## See also

- [`get`](get.md)
- [`get all`](get-all.md)
- [`values`](values.md) — render-time value merge from a package directory
- [Values guide](../guides/values.md)
- [`values.yaml` reference](../reference/values-yaml.md)
