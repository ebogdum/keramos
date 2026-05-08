# keramos diff

## Synopsis

`keramos diff` shows what would change if you ran `keramos upgrade` against a release with a given package + values. The diff is computed via server-side dry-run, so the cluster's defaulters and admission webhooks contribute to the comparison — you see the diff the cluster would actually compute, not a textual diff of two YAML files.

## When to use it

Run before any production upgrade. The output is a structured per-resource diff with each changed field on its own line. Pair with `keramos plan` if you want to capture the rendered manifest into a reviewable file before applying.

## Usage

```
keramos diff <release-name> <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for diff |
| `--no-color` | — | — | disable colored diff output |
| `--profile` | string | — | profile name to apply |
| `--revision` | int | — | compare against a specific revision |
| `--set` | stringArray | — | set key=value overrides (repeatable) |
| `--set-file` | stringArray | — | set key=path; value read from path (repeatable) |
| `--set-json` | stringArray | — | set key=<json>; value parsed as JSON (repeatable) |
| `--set-string` | stringArray | — | set key=value forcing string interpretation (repeatable) |
| `--show-annotations` | — | — | include metadata.annotations |
| `--show-defaulted-fields` | — | — | include server-side defaults (clusterIP, port protocol, etc.) |
| `--show-finalizers` | — | — | include metadata.finalizers |
| `--show-generation` | — | — | include resourceVersion/uid/generation/creationTimestamp |
| `--show-image-pull-policy` | — | — | include containers[].imagePullPolicy |
| `--show-labels` | — | — | include metadata.labels |
| `--show-managed-fields` | — | — | include metadata.managedFields |
| `--show-owner-refs` | — | — | include metadata.ownerReferences |
| `--show-secret-rotation` | — | — | include rotated Secret data values |
| `--show-status` | — | — | include changes under .status |
| `--smart` | bool | true | use Kubernetes-aware structured diff (use `--smart=false` for raw line-level unified diff) |
| `-f, --values` | stringArray | — | values file overrides (repeatable) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Diff a candidate upgrade:

```sh
keramos diff my-app ./my-app -n prod
```

Compare against an alternate values file:

```sh
keramos diff my-app ./my-app -f new.yaml -n prod
```

Diff against a specific historical revision (compare current package + values to what revision 5 looked like):

```sh
keramos diff my-app ./my-app --revision 5 -n prod
```

Show fields normally hidden (server-side defaults, managed fields):

```sh
keramos diff my-app ./my-app --show-defaulted-fields --show-managed-fields -n prod
```

Force a raw line-level unified diff (no structural awareness):

```sh
keramos diff my-app ./my-app --smart=false -n prod
```

## See also

- [`upgrade`](upgrade.md)
- [`plan`](plan.md)
- [`drift`](drift.md)
