# keramos metrics

## Synopsis

`keramos metrics` polls metrics-server periodically and reports CPU/memory usage statistics for a release's pods (min, max, average, spikes). With `--recommend`, it computes suggested `requests` and `limits` based on the observed distribution.

## When to use it

Use after a release has been running for some time to right-size its resource allocations. The first invocation needs a sampling window — keep `--duration` long enough to span the workload's actual traffic pattern (a 30-second window during quiet periods will undersize). Pair with `--recommend` to get a values-shaped suggestion you can paste into your overrides.

## What happens when you run it

1. Resolves the release's pods via the release record.
2. Polls `metrics.k8s.io` (the metrics-server API) every `--interval` for `--duration`.
3. Aggregates per-container statistics: min, avg, p50, p95, max for both CPU and memory.
4. Prints the table.
5. With `--recommend`, computes `requests` (≈ p50 × small headroom) and `limits` (≈ p95 × bigger headroom) and prints a values-yaml-shaped block.

## Usage

```
keramos metrics <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--duration` | duration | 30s | total sampling window |
| `-h, --help` | — | — | help for metrics |
| `--interval` | duration | 5s | interval between samples |
| `-o, --output` | string | "table" | output format: table, json |
| `--recommend` | — | — | also print suggested resources.requests/limits values-block |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Sample for 10 minutes and report:

```sh
keramos metrics my-app --duration 10m -n prod
```

Recommend resource requests and limits after observing for an hour:

```sh
keramos metrics hello --duration 1h --recommend -n prod
```

JSON for ingestion by another tool:

```sh
keramos metrics hello --duration 5m -n prod -o json | jq '.[] | {container, p95}'
```

Tighter sampling cadence for a short bursty workload:

```sh
keramos metrics hello --duration 2m --interval 1s -n prod
```

## See also

- [`status`](status.md)
