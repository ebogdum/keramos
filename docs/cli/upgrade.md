---
title: "keramos upgrade"
parent: "CLI"
---
{% raw %}
# keramos upgrade

`keramos upgrade` renders a package and applies it to an existing release as the
next revision, patching what changed and pruning resources the render no longer
contains.

## When to use it

- To roll out new config or a new package version to a running release.
- To install-or-upgrade idempotently in CI with `--install`.
- To change a single value while keeping everything else with `--only`.

## What happens

1. Reads the release's latest stored revision (this is the input state). With
   `--install`, a missing release is installed instead of failing.
2. Merges values by the chosen strategy — by default package defaults are
   overlaid with the previous release's values, then `-f`/`--set*`. `--reuse-values`,
   `--reset-values`, `--reset-then-reuse-values`, and `--only` change that recipe.
3. Renders the package and validates the merged values against the schema if
   one is present.
4. Stores a new revision (previous + 1), runs pre-upgrade hooks, applies CRDs
   first, then the manifest via server-side apply (a three-way merge). Resources
   absent from the new render are deleted.
5. Waits for readiness unless `--no-wait`, runs post-upgrade hooks, and marks
   the revision deployed.

Requires a reachable cluster. On failure the upgrade rolls back to the previous
revision unless `--no-atomic` is set.

## Usage

```
keramos upgrade <release-name> <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-f, --values` | stringArray | — | values file override; repeatable, later files win |
| `--set` | stringArray | — | `key=value` override (repeatable) |
| `--set-string` | stringArray | — | `key=value` forced to string (repeatable) |
| `--set-file` | stringArray | — | `key=path`; value is read from the file (repeatable) |
| `--set-json` | stringArray | — | `key=<json>`; value parsed as a JSON literal (repeatable) |
| `--profile` | string | — | profile to apply on top of the merged values |
| `--env` | string | — | environment from `keramos.yaml`'s `environments:` section |
| `--reuse-values` | — | — | start from the previous release's merged values, no defaults |
| `--reset-values` | — | — | discard previous values; start from package defaults |
| `--reset-then-reuse-values` | — | — | package defaults first, then merge previous values on top |
| `--only` | strings | — | update only these dotted paths; every other key keeps its previous value (comma-separated) |
| `--install` | — | — | install the release if it doesn't exist yet |
| `--wait` | — | on | wait for every resource to be Ready (default behaviour) |
| `--no-wait` | — | — | return once resources are applied, without waiting for Ready |
| `--wait-for-jobs` | — | — | also block until Job resources complete |
| `--timeout` | duration | 5m0s | how long the readiness wait may run before failing |
| `--dry-run` | string | — | `client` renders locally; `server` also validates against the API |
| `-o, --output` | string | table | result format: `table`, `json`, or `yaml` |
| `--description` | string | — | free-text note stored on the new revision |
| `--no-atomic` | — | — | leave partial changes in place on failure instead of rolling back |
| `--no-force` | — | — | don't force field ownership on server-side apply |
| `--no-hooks` | — | — | skip all lifecycle hooks for this upgrade |
| `--create-namespace` | — | — | create the namespace if missing (only with `--install`) |
| `--include-crds` | — | — | include CRDs from `crds/` in the rendered manifest |
| `--labels` | stringArray | — | `key=value` label recorded on the release (repeatable) |
| `--api-versions` | stringArray | — | extra API versions to report as available in capability checks (repeatable) |
| `--kube-version` | string | — | override the Kubernetes version reported to templates |
| `--post-renderer` | string | — | command fed the manifest on stdin; its stdout is applied |
| `--post-renderers` | stringArray | — | chained post-renderers; output of N feeds N+1 (repeatable) |
| `--post-renderer-timeout` | duration | 5m0s | per-stage timeout for each post-renderer |
| `--cleanup-on-fail` | — | — | delete resources this upgrade created if it fails |
| `--recreate-pods` | — | — | trigger a rolling restart of Deployments/StatefulSets/DaemonSets |
| `--force` | — | — | delete and recreate resources to update immutable fields |
| `--hook-timeout` | duration | 0 | cap each hook's timeout (0 = use the chart-declared value) |
| `--history-max` | int | 0 | max revisions to retain in history (0 = unlimited) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | namespace of the release |

## Worked example — the two inputs and the revision they produce

**INPUT 1 — the stored release (what's running now).** `web` is at revision 1
with two replicas and image tag `1.0.0`:

```yaml
# keramos get web -n apps  →  the current revision 1
name: web
revision: 1
status: deployed
values:
  replicas: 2
  image:
    tag: "1.0.0"       # ← previously applied
```

**INPUT 2 — the command.** You bump only the image tag and keep everything else
by restricting the upgrade to that one path:

```sh
keramos upgrade web ./web -n apps --only image.tag --set image.tag=1.1.0
```

**OUTPUT:**

```
release web upgraded (revision 2)
```

**State written** (the new stored revision):

```yaml
# keramos get web -n apps  →  the recorded revision 2
revision: 2
status: deployed
values:
  replicas: 2          # kept from revision 1
  image:
    tag: "1.1.0"       # changed
```

**Tracing every line back to the inputs:**

| Output / state | Which input it came from | Why |
|---|---|---|
| `upgraded (revision 2)` | INPUT 1 revision 1 + 1 | the counter increments off the previous revision |
| `image.tag: 1.1.0` | INPUT 2 `--set image.tag=1.1.0` | the path you named took the new value |
| `replicas: 2` | INPUT 1 | `--only image.tag` reverted every other key to its revision-1 value |
| `status: deployed` | readiness wait passed | `--wait` (default) confirmed the rollout is Ready |

Install-or-upgrade in one command for CI:

```sh
keramos upgrade web ./web -n apps --install
```

## See also

- [`install`](install.md) — create a release (revision 1)
- [`rollback`](rollback.md) — return a release to an earlier revision
- [`plan`](plan.md) — preview what an upgrade would change
- [`history`](history.md) — list a release's revisions
- [`canary`](canary.md) — staged, health-gated rollout
{% endraw %}
