# keramos get hooks

## Synopsis

`keramos get hooks` shows the hook manifests rendered for a release together with the outcome of each hook's last execution (succeeded / failed, with timestamps). Hooks are the lifecycle Jobs and Pods declared under `hooks/` in the package — pre/post-install, pre/post-upgrade, pre/post-rollback, pre/post-delete, and test. Keramos stores the rendered hook YAML alongside the manifest in the release record so that rolling back to an older revision re-runs that revision's hooks.

## When to use it

Use to confirm what hooks ran (or failed to run) on the last install/upgrade, to inspect the exact YAML keramos rendered for a hook (which may differ from the on-disk source if values changed), or to debug a failing hook by extracting it for `kubectl apply` reproduction.

## What happens when you run it

1. Reads the release-storage Secret for `<release-name>` at the requested revision (default: current).
2. Extracts the `hookTemplates` (rendered YAML, filename → body) and `hooks` (last-run results) sections.
3. Prints them in the requested output format.

## Usage

```
keramos get hooks <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for hooks |
| `-o, --output` | string | table | output format: table, json, yaml |
| `--revision` | int | 0 | get hooks from a specific revision (0 = current) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Default tabular view of every hook and its last-run outcome:

```sh
keramos get hooks hello -n prod
```

Print the rendered hook YAML for revision 3 (e.g. to compare against the current revision's hooks):

```sh
keramos get hooks hello --revision 3 -n prod -o yaml
```

Pull hook results as JSON for ingestion by another tool:

```sh
keramos get hooks hello -n prod -o json | jq '.[] | select(.status == "failed")'
```

## See also

- [`get`](get.md)
- [`get all`](get-all.md)
- [`history`](history.md)
- [Hooks guide](../guides/hooks.md)
- [Hooks in templates](../templates/hooks.md)
