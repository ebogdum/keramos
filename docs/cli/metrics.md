---
title: "keramos metrics"
parent: "CLI"
---
{% raw %}
# keramos metrics

`keramos metrics` samples a release's pods over time and prints per-container
CPU and memory statistics — optionally with suggested requests and limits.

## When to use it

- To right-size a release: watch what its containers actually use before you
  set `resources.requests` and `resources.limits`.
- To spot a container that spikes far above its average, so you catch it
  before it gets OOM-killed or throttled.
- Add `--recommend` to get a ready-to-paste values block instead of reading
  the numbers yourself.

Sample over a window that spans real traffic — a 30-second window during a
quiet period will undersize. The cluster needs metrics-server installed;
without it the first sample fails with a clear message.

## What happens

1. You name a release. keramos finds every pod that belongs to it (matched by the
   names of the workloads in the release manifest) and prints which prefixes
   it is sampling.
2. It polls the `metrics.k8s.io` API every `--interval` for `--duration`,
   building a usage history.
3. It prints a table: one row per container with the sample count and the
   min / avg / p50 / p95 / max for CPU (in millicores) and memory.
4. With `--output json` the same statistics print as JSON instead.
5. With `--recommend` it adds a `resources:` block: requests sized from p50
   and limits from p95, each with headroom. Treat these as starting points.

## Usage

```
keramos metrics <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--duration` | duration | `30s` | how long to sample overall — make it span real traffic |
| `--interval` | duration | `5s` | how often to take a sample within the window |
| `-o, --output` | string | `table` | render as `table` or `json` |
| `--recommend` | — | `false` | also print a suggested `resources.requests`/`limits` block |

### Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Worked example

Sample the `web-api` release for ten minutes and recommend resources:

```sh
keramos metrics web-api --duration 10m -n prod --recommend
```

Output:

```
Sampling web-api pods every 5s for 10m0s (matching prefixes: [web-api])

CONTAINER                          SAMPLES    CPU(min/avg/p50/p95/max, m)              MEM(min/avg/p50/p95/max)
api                                   120         12 /     41 /    38 /    92 /   140        70Mi / 118Mi / 112Mi / 210Mi / 240Mi

# suggested resources block (paste into values.yaml):
resources:
  # container: api (over 120 samples)
  requests: {cpu: 50m, memory: 144Mi}
  limits:   {cpu: 150m, memory: 320Mi}
```

Get the raw statistics as JSON for another tool:

```sh
keramos metrics web-api --duration 5m -n prod -o json | jq '.[] | {container, cpuP95, memP95}'
```

If no pods match the release, keramos prints:

```
no samples collected (no pods matching the release labels?)
```

## See also

- [`status`](status.md) — current health of the release's resources
- [`get manifest`](get-manifest.md) — the workloads whose pods are sampled
{% endraw %}
