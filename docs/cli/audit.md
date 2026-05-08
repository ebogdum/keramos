# keramos audit

## Synopsis

`keramos audit` prints the chronological audit trail for a release: every install, upgrade, rollback, or uninstall action with its timestamp, the user/host that initiated it, the kubeconfig context, the keramos binary version, the CLI flags as passed, and any value files supplied. The audit log is signed metadata embedded in the release record.

## When to use it

Use after an incident to answer "who applied what, when, with what flags?". Useful for compliance, post-mortems, and operator handoffs.

## Usage

```
keramos audit <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for audit |
| `-o, --output` | string | "table" | output format: table, json, yaml |
| `--revision` | int | — | show only the named revision (0 = all) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Full audit trail:

```sh
keramos audit my-app -n prod
```

Single revision as JSON:

```sh
keramos audit my-app --revision 3 -n prod -o json
```

## See also

- [`history`](history.md)
- [`get`](get.md)
