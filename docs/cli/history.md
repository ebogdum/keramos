# keramos history

## Synopsis

`keramos history` prints every revision of a release with its timestamp, status, package version, action (`install` / `upgrade` / `rollback`), and the audit user who initiated it. Each row is a point-in-time snapshot of the release; `keramos get manifest <release> --revision N` retrieves the manifest as it stood at revision N.

## When to use it

Use to answer "how did we get to this state?" and to find the right revision number to roll back to. The default sort is oldest-first; the most recent revision is the bottom row.

## What happens when you run it

1. Lists every release-storage Secret for `<release-name>` in the namespace.
2. Decodes each revision's metadata.
3. Sorts by revision number ascending.
4. Truncates to `--max` if set.
5. Renders in the requested format.

## Usage

```
keramos history <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for history |
| `--max` | int | — | maximum number of revisions to show (0 = all) |
| `-o, --output` | string | "table" | output format: table, json, yaml |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Print the history of a release:

```sh
keramos history my-app -n my-app-prod
```

History plus full audit data per revision:

```sh
keramos history my-app --max 50 -n my-app-prod -o yaml
```

## See also

- [`audit`](audit.md)
- [`rollback`](rollback.md)
- [`get`](get.md)
