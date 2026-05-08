# keramos helm-compat export

## Synopsis

`keramos helm-compat export` writes a keramos package as a Helm v3 chart skeleton: `Chart.yaml`, `values.yaml`, `templates/`, and the rest of the conventional Helm layout. The export is a structural translation, not a faithful re-implementation of keramos's expression engine — `${...}` expressions in templates are preserved as-is, so the resulting chart is suitable for static analysis (scanning, linting, GitOps inspection), not for `helm install`.

## When to use it

Use when downstream tooling expects a Helm chart shape — for example, an OCI mirror that scans for `Chart.yaml`, a security scanner whose Helm parser can read the templates' YAML structure even if the templating language differs, or a chart-museum that rejects keramos's native `.keramos.tgz` archive.

## What happens when you run it

1. Reads the keramos package at `<keramos-package-path>`.
2. Translates `keramos.yaml` to `Chart.yaml` (apiVersion `v2`, name/version preserved, `appVersion` mapped, dependencies translated).
3. Copies `values.yaml`, `values.schema.json`, `templates/`, `crds/`, `README.md` into the output directory unchanged (templates' bodies are not re-rewritten — `${...}` stays).
4. Writes the result to `--out` (defaults to a sibling directory if unset).
5. Prints the output path on success.

## Usage

```
keramos helm-compat export <keramos-package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for export |
| `--out` | string | "" | output directory for the Helm chart |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Export a keramos package to a side directory:

```sh
keramos helm-compat export ./my-app --out ./helm-export
```

Export and immediately tar it up for an OCI registry that expects Helm shape:

```sh
keramos helm-compat export ./my-app --out ./helm-export
tar czf my-app-1.0.0.tgz -C ./helm-export my-app
```

## See also

- [`helm-compat`](helm-compat.md)
- [`helm-compat report`](helm-compat-report.md)
- [`migrate`](migrate.md) — the reverse direction: convert a Helm chart to a keramos package
- [Migration guide](../guides/migration.md)
