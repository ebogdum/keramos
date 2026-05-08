# keramos drift

## Synopsis

`keramos drift` reports per-resource, per-field differences between the manifest keramos stored at the latest revision and the current live state in the cluster. Common sources: out-of-band `kubectl edit`, controllers patching status fields, admission webhooks defaulting fields, or other operators claiming ownership of fields keramos doesn't manage.

## When to use it

Use to detect unmanaged changes before a planned upgrade — once the cluster's drifted, the next `keramos upgrade` will reset whichever fields keramos owns. Pair with `keramos reconcile` to converge to the stored manifest, or run periodically as a drift alarm in monitoring.

## What happens when you run it

1. Reads the latest revision's stored manifest from the release record.
2. For each resource in the stored manifest, fetches the live object from the cluster.
3. Compares stored vs live, surfacing every divergent field.
4. Prints the differences in the chosen output format.
5. Read-only; no resources are modified.

## Usage

```
keramos drift <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for drift |
| `-o, --output` | string | "table" | output format: table, json, yaml |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Drift report:

```sh
keramos drift my-app -n prod
```

JSON output for ingestion by another tool:

```sh
keramos drift hello -n prod -o json | jq '.[] | {kind, name, fields}'
```

Detect drift, then converge:

```sh
keramos drift     hello -n prod
keramos reconcile hello -n prod
```

## See also

- [`reconcile`](reconcile.md)
- [`diff`](diff.md)
- [`get manifest`](get-manifest.md)
