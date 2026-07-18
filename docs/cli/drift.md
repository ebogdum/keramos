# keramos drift

## Synopsis

`keramos drift` is a **three-way** comparison. It puts three views of a release
side by side — the package as it renders now, the recorded state, and the live
cluster — and reports, per resource and field, where they disagree.

```
package  — what the directory would render right now
state    — what keramos last recorded (the stored manifest)
running  — what is actually in the cluster
```

It flags two distinct kinds of divergence:

- **⚠ cluster drift** — `state ≠ running`: something changed the cluster
  (`kubectl edit`, a controller, a webhook) since the last apply.
- **→ pending apply** — `package ≠ state`: the directory has edits that have
  not been applied yet (this is also what [`keramos plan`](plan.md) previews).

## When to use it

- Before an upgrade, to see both what you changed locally *and* what changed in
  the cluster out of band — in one view.
- As a drift alarm: if `state ≠ running`, someone edited the cluster directly.

Pair with [`keramos reconcile`](reconcile.md) to push the stored state back onto a
drifted cluster.

## What happens when you run it

1. Renders `[package-path]` (default `.`) — the **package** view.
2. Derives the release from `keramos.yaml` (`-r` overrides) and reads the stored
   manifest — the **state** view.
3. Fetches the live object for each resource in the state — the **running** view.
4. Compares the three, limited to keramos-managed fields (so cluster-injected
   noise — status, managedFields, server defaults — is ignored).

Read-only; no resources are modified. Requires a reachable cluster.

## Usage

```
keramos drift [package-path] [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-r, --release` | string | (from keramos.yaml) | state/release name; overrides the derived name |
| `--profile` | string | — | profile to apply when rendering the package side |
| `-f, --values` | stringArray | — | values file for the package side (repeatable) |
| `--set` | stringArray | — | key=value for the package side (repeatable) |
| `--set-string` | stringArray | — | key=value (string) for the package side |
| `--no-color` | — | — | disable colored output |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Three-way drift for the package in the current directory (in → out):

```sh
cd ./mychart
keramos drift
```

```
drift: package ↔ state ↔ running   (release mychart)

~ differs                Service/mychart  (namespace apps)
      spec.ports.0.port  ⚠ cluster drift
          package: 8080
          state:   8080
          running: 9090
      metadata.labels.tier  → pending apply
          package: canary
          state:   stable
          running: stable

1 cluster-drift, 1 pending-apply, 0 orphan, 0 missing, 0 to-create.
```

Reading it: the port was changed **in the cluster** (state and package agree at
8080, running is 9090). The label is a **local edit** not yet applied (package
is `canary`, state and running are `stable`).

Detect drift, then converge the cluster back to the stored state:

```sh
keramos drift     ./mychart
keramos reconcile mychart
```

## See also

- [`plan`](plan.md) — package vs state (the 2-way subset)
- [`diff`](diff.md) — compare two files/versions (no cluster)
- [`reconcile`](reconcile.md) — re-apply the stored state onto the cluster
