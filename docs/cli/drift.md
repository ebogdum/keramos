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

## Worked example — the three inputs and how they produce the output

`drift` compares **three** things. To read its output you have to see all three
inputs. Here they are, concretely.

**INPUT 1 — the package (`./mychart`), as it renders right now.** You just
edited `values.yaml`, changing the tier label from `stable` to `canary`, but you
have NOT applied it yet:

```yaml
# keramos template ./mychart  →  the Service it produces
apiVersion: v1
kind: Service
metadata:
  name: mychart
  namespace: apps
  labels:
    tier: canary        # ← you changed this locally (was stable)
spec:
  ports:
    - port: 8080
```

**INPUT 2 — the recorded state** (what keramos stored the last time you applied).
It still has the old label AND the original port:

```yaml
# what keramos recorded at the last apply
metadata:
  labels:
    tier: stable        # old label — your canary edit isn't applied yet
spec:
  ports:
    - port: 8080        # original port
```

**INPUT 3 — the live cluster** (what's actually running). Someone ran
`kubectl edit svc/mychart` and changed the port to 9090 behind keramos's back:

```yaml
# what is actually in the cluster right now
metadata:
  labels:
    tier: stable        # matches the state — nobody touched the label live
spec:
  ports:
    - port: 9090        # ← changed in the cluster, out of band
```

**Now run it:**

```sh
cd ./mychart
keramos drift
```

**OUTPUT:**

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

**Tracing every line back to the inputs:**

| Output line | Which inputs it read | Why that verdict |
|---|---|---|
| `spec.ports.0.port` `package: 8080` | INPUT 1 → 8080 | the port in your rendered package |
| `state: 8080` | INPUT 2 → 8080 | the port keramos recorded |
| `running: 9090` | INPUT 3 → 9090 | the port live in the cluster |
| `⚠ cluster drift` | state (8080) **≠** running (9090) | the cluster changed vs what keramos recorded → **someone edited it out of band** |
| `metadata.labels.tier` `package: canary` | INPUT 1 → canary | your local edit |
| `state: stable` / `running: stable` | INPUTs 2 & 3 → stable | state and cluster still agree |
| `→ pending apply` | package (canary) **≠** state (stable) | your edit is not applied yet (`keramos upgrade` would apply it) |

**The two rules, restated:**

- **`⚠ cluster drift`** fires when **state ≠ running** — the live cluster no
  longer matches what keramos last applied (something changed it outside keramos).
- **`→ pending apply`** fires when **package ≠ state** — your local package has
  edits you haven't applied. This is exactly what `keramos plan` previews.

A field where all three agree is not shown. The summary line counts each class.

Detect drift, then converge the cluster back to the stored state:

```sh
keramos drift     ./mychart
keramos reconcile mychart
```

## See also

- [`plan`](plan.md) — package vs state (the 2-way subset)
- [`diff`](diff.md) — compare two files/versions (no cluster)
- [`reconcile`](reconcile.md) — re-apply the stored state onto the cluster
