# keramos canary

## Synopsis

`keramos canary` performs a staged upgrade by advancing through a series of replica counts (e.g. 1, 3, 5) with a configurable bake period at each step. Keramos pauses between steps for the bake duration and waits for the new pods to be Ready; if any stage's readiness fails, the release is rolled back to the prior revision.

## When to use it

Use for risk-sensitive production rollouts where you want load-induced regressions to be caught early, on a small fraction of traffic. Combine with workload-level health checks and external observability for end-to-end validation.

## Usage

```
keramos canary <release> <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--bake` | duration | 1m0s | pause between stages to observe health |
| `-h, --help` | — | — | help for canary |
| `--profile` | string | — | profile name |
| `--set` | stringArray | — | key=value (repeatable) |
| `--set-file` | stringArray | — | set key=path; value read from path (repeatable) |
| `--set-json` | stringArray | — | set key=<json>; value parsed as JSON (repeatable) |
| `--set-string` | stringArray | — | force string interpretation (repeatable) |
| `--stages` | strings | — | comma-separated replica counts to step through (e.g. 1,3,5) |
| `-f, --values` | stringArray | — | values file (repeatable) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Canary an upgrade through 1 → 3 → 5 replicas with 5-minute bake periods at each stage:

```sh
keramos canary my-app ./my-app --stages 1,3,5 --bake 5m -n prod
```

Quick two-stage canary (small bake, suitable for dev clusters):

```sh
keramos canary my-app ./my-app --stages 1,3 --bake 30s -n dev
```

Canary with a values override (e.g. point at a new image tag) applied alongside the staged ramp:

```sh
keramos canary my-app ./my-app --stages 1,3,5 --bake 5m --set image.tag=1.4.2 -n prod
```

## See also

- [`upgrade`](upgrade.md)
- [`rollback`](rollback.md)
