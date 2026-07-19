---
title: "keramos reconcile"
parent: "CLI"
---
{% raw %}
# keramos reconcile

`keramos reconcile` re-applies a release's stored manifest onto the cluster,
pulling drifted resources back to the state keramos last recorded.

## When to use it

- After [`keramos drift`](drift.md) reports cluster drift — someone ran
  `kubectl edit`, a controller patched a field, or a resource was deleted out
  of band — and you want the cluster back to the recorded state.
- As a converge step in automation that keeps live resources matching keramos's
  stored state, without cutting a new revision.

## What happens

1. Reads the latest stored manifest for `<release-name>`.
2. Compares it against the live cluster; if nothing has drifted, it prints
   `No drift to reconcile.` and stops.
3. Otherwise it re-applies the stored resources, skipping any annotated
   `resource-policy: keep` (their drift is intentionally preserved).
4. Unless `--no-wait` is set, it waits up to `--timeout` for the re-applied
   resources to become Ready.
5. Prints how many resources it converged and lists them.

Mutating: it writes to the cluster. It does **not** create a new revision —
the package and values are unchanged, so use [`keramos upgrade`](upgrade.md) when
you actually want to roll new content. Requires a reachable cluster.

## Usage

```
keramos reconcile <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--timeout` | duration | 5m0s | how long to wait for re-applied resources to become Ready |
| `--no-wait` | — | false | return as soon as the manifest is applied, without waiting for readiness |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Worked example — push stored state back onto a drifted cluster

**INPUT — the cluster has drifted.** Someone edited the live Service port, and
`keramos drift` flags it:

```sh
keramos drift mychart -n apps
# ~ differs   Service/mychart  (namespace apps)
#       spec.ports.0.port  ⚠ cluster drift
#           state:   8080
#           running: 9090
```

The stored state says `8080`; the live cluster says `9090`.

**Reconcile the release:**

```sh
keramos reconcile mychart -n apps
```

**OUTPUT:**

```
Reconciled 1 resource(s):
  - Service/mychart
```

**Re-run drift to confirm it is clean:**

```sh
keramos drift mychart -n apps
# 0 cluster-drift, 0 pending-apply, 0 orphan, 0 missing, 0 to-create.
```

**Tracing the output:**

| Output | Cause |
|---|---|
| `Reconciled 1 resource(s)` | one resource had drifted from the stored state |
| `- Service/mychart` | the Service whose live port `9090` was re-applied back to the stored `8080` |
| drift now reports `0 cluster-drift` | the cluster matches the stored manifest again |

If nothing had drifted, reconcile would instead print `No drift to reconcile.`
and leave the cluster alone.

## See also

- [`drift`](drift.md) — detect the divergence reconcile fixes
- [`upgrade`](upgrade.md) — apply new package or values (creates a revision)
- [`get manifest`](get-manifest.md) — inspect the stored manifest reconcile re-applies
{% endraw %}
