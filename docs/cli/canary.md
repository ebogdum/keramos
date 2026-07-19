---
title: "keramos canary"
parent: "CLI"
---
{% raw %}
# keramos canary

`keramos canary` runs a staged upgrade, stepping the replica count up through a
list of stages with a bake pause between each, and rolls back automatically if
any stage fails.

## When to use it

- You want a risky upgrade to reach only a few replicas first, so a bad
  release is caught before it runs at full scale.
- You want failure at any stage to undo the whole rollout automatically,
  without a manual `keramos rollback`.

## What happens

1. Requires at least one `--stages` entry; otherwise it errors immediately.
2. Records the release's current revision as the pre-canary baseline.
3. For each stage, upgrades the release from `<package-path>` with
   `replicas=<stage>` (on top of your `--set` / `--values`), waiting up to
   5 minutes for the new pods to become Ready. The first stage installs the
   release if it does not exist yet.
4. Between stages it bakes for `--bake`, giving you a window to observe health
   before ramping further.
5. If any stage fails, it rolls back to the pre-canary baseline revision — or,
   if the release was created by this canary, uninstalls the partial
   deployment.
6. On success it prints the final stage reached. The whole run is bounded by a
   30-minute deadline.

Mutating: it upgrades a real release. Requires a reachable cluster, and the
package's templates must consume a `replicas` value for the staging to take
effect.

## Usage

```
keramos canary <release> <package-path> --stages <n,n,...> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--stages` | strings | — | comma-separated replica counts to step through (e.g. `1,3,5`); at least one is required |
| `--bake` | duration | 1m0s | pause between stages so you can observe health before ramping |
| `--profile` | string | — | profile to apply when rendering the package |
| `-f, --values` | stringArray | — | values file applied at every stage (repeatable) |
| `--set` | stringArray | — | `key=value` applied at every stage (repeatable) |
| `--set-string` | stringArray | — | `key=value` forced as a string (repeatable) |
| `--set-file` | stringArray | — | `key=path`; value read from the file (repeatable) |
| `--set-json` | stringArray | — | `key=<json>`; value parsed as JSON (repeatable) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Worked example — ramp 1 → 3 → 5 with a bake between stages

**INPUT — a running release you want to upgrade cautiously,** using a package
whose Deployment reads `replicas` from values:

```sh
keramos status mychart -n apps
# REVISION  STATUS     ...
# 6         deployed
```

**Run the canary with a new image and three stages:**

```sh
keramos canary mychart ./mychart \
  --stages 1,3,5 --bake 2m --set image.tag=1.6.0 -n apps
```

**OUTPUT (happy path):**

```
canary stage 1/3 → replicas=1
baking for 2m0s …
canary stage 2/3 → replicas=3
baking for 2m0s …
canary stage 3/3 → replicas=5
canary completed at stage 5
```

**OUTPUT (stage 2 fails readiness):**

```
canary stage 1/3 → replicas=1
baking for 2m0s …
canary stage 2/3 → replicas=3
stage 2 failed: … — rolling back to pre-canary revision 6
Error: …
```

**Tracing the output:**

| Output | Cause |
|---|---|
| `stage 1/3 → replicas=1` | first `--stages` entry, upgraded with `replicas=1` |
| `baking for 2m0s` | `--bake 2m` pause before the next stage |
| `completed at stage 5` | every stage reached Ready; last stage was `5` |
| `rolling back to pre-canary revision 6` | stage 2 failed, so keramos reverted to the revision recorded before the canary began |

## See also

- [`upgrade`](upgrade.md) — the single-shot upgrade each stage performs
- [`rollback`](rollback.md) — manually revert a release to a prior revision
- [`status`](status.md) — check the release revision before and after
{% endraw %}
