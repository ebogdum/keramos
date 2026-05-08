# keramos reconcile

## Synopsis

`keramos reconcile` re-applies the stored manifest of a release's current revision back into the cluster. Drifted fields are restored to what keramos last persisted; resources that vanished out-of-band are recreated. Unlike `keramos upgrade`, no new revision is created — the release record's revision counter does not increment, because the package, values, and templates are unchanged.

## When to use it

Use after `keramos drift` reports unwanted divergence — somebody ran `kubectl edit`, an unrelated controller patched a field, or admission rewrote something. For a planned re-render that picks up new package or values changes (and increments the revision counter), use `keramos upgrade` instead.

## What happens when you run it

1. Reads the latest revision's stored manifest from the release record.
2. Server-side applies it back to the cluster, taking ownership of any drifted fields.
3. With the default `--wait`, blocks until workloads converge to Ready.
4. Does **not** create a new revision; the release record is unchanged.

## Usage

```
keramos reconcile <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for reconcile |
| `--no-wait` | bool | false | do not wait for resources to be ready after re-apply |
| `--timeout` | duration | 5m0s | readiness wait after apply |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Reconcile a release back to its stored manifest:

```sh
keramos reconcile hello -n prod
```

Reconcile without waiting for readiness (useful for batch operations):

```sh
keramos reconcile hello --no-wait -n prod
```

Detect-then-reconcile workflow:

```sh
keramos drift     hello -n prod
keramos reconcile hello -n prod
keramos drift     hello -n prod   # confirm clean
```

## See also

- [`drift`](drift.md) — detect divergence first
- [`upgrade`](upgrade.md) — for new package or values
- [`get manifest`](get-manifest.md)
