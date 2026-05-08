# keramos upgrade

## Synopsis

`keramos upgrade` applies a new revision of a release. The package (which may be the same path as the original install, or a new version pulled from a registry) is rendered, validated, and server-side-applied; resources that no longer appear in the rendered manifest are deleted; resources that changed are patched. The release's revision counter increments, the new manifest and merged values are stored, hooks tagged for upgrade events fire, and (with the default `--wait`) keramos blocks until the resulting workloads are Ready.

## When to use it

Use this when an existing release needs new code, new config, or new package metadata. Pass `--install` if the release may not yet exist and you want one command that handles both cases. For risk-averse rollouts, render with `keramos diff` or `keramos plan` first; for production rollouts where each replica should bake before the next, see `keramos canary`.

## What happens when you run it

1. Reads the release's previous revision from the cluster.
2. Renders `<package-path>` against the chosen value-merge strategy:
   - default: defaults → previous user values → `-f` files → `--set*` (the typical incremental upgrade)
   - `--reset-values`: defaults only (drops previous user values)
   - `--reuse-values`: previous merged values, no new overrides
   - `--reset-then-reuse-values`: defaults, then previous user values merged on top
   - `--only key1,key2`: incremental — change only the named keys, preserve everything else
3. Validates merged values against `values.schema.json` if present.
4. Runs `pre-upgrade` hooks.
5. With `--include-crds`, applies CRDs first and waits for `Established=true`.
6. Server-side applies the rendered manifest; deletes resources no longer present.
7. With `--force`, deletes-and-recreates resources where immutable fields changed.
8. With the default `--wait` (or `--wait-for-jobs`), blocks until workloads converge.
9. Runs `post-upgrade` hooks.
10. Records the new revision in the release Secret. On failure (with the default atomic behaviour), rolls back to the previous revision.

## Usage

```
keramos upgrade <release-name> <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--api-versions` | stringArray | — | Kubernetes API version available for capability checks (repeatable) |
| `--cleanup-on-fail` | bool | false | delete partially-applied resources if the upgrade fails |
| `--create-namespace` | bool | false | create the release namespace if missing (with `--install`) |
| `--description` | string | "" | release description |
| `--dry-run` | string | "" | dry-run mode: `client` (local render) or `server` (API validation) |
| `--env` | string | "" | environment name declared in `keramos.yaml`'s `environments:` section |
| `--force` | bool | false | delete-and-recreate resources to force update of immutable fields |
| `-h, --help` | bool | false | help for upgrade |
| `--history-max` | int | 0 | maximum revisions to retain in history (0 = unlimited) |
| `--hook-timeout` | duration | 0 | cap each hook's per-hook timeout (0 = use the chart-declared value) |
| `--include-crds` | bool | false | include CRDs from `crds/` in the rendered manifest |
| `--install` | bool | false | install if release does not exist |
| `--kube-version` | string | "" | override Kubernetes version reported in capabilities |
| `--labels` | stringArray | — | label key=value to attach to the release (repeatable) |
| `--no-atomic` | bool | false | don't roll back on failure |
| `--no-force` | bool | false | don't force field ownership on server-side apply |
| `--no-hooks` | bool | false | skip lifecycle hooks for this operation |
| `--no-wait` | bool | false | don't wait for resources to be ready |
| `--only` | strings | — | incremental upgrade: dotted value paths to update; all other keys retain their previous values (repeatable, comma-separated) |
| `-o, --output` | string | table | output format: table, json, yaml |
| `--post-renderer` | string | "" | command piped the rendered manifests on stdin (yields stdout) |
| `--post-renderer-timeout` | duration | 5m0s | per-stage timeout for post-renderers |
| `--post-renderers` | stringArray | — | chained post-renderers (repeatable; output of N feeds N+1) |
| `--profile` | string | "" | profile name to apply |
| `--recreate-pods` | bool | false | trigger a rolling restart of Deployments/StatefulSets/DaemonSets |
| `--reset-then-reuse-values` | bool | false | reset to defaults then merge previous values |
| `--reset-values` | bool | false | reset values to package defaults |
| `--reuse-values` | bool | false | reuse values from previous release |
| `--set` | stringArray | — | set key=value overrides (repeatable) |
| `--set-file` | stringArray | — | set key=path; the value is read from path (repeatable) |
| `--set-json` | stringArray | — | set key=<json>; value is parsed as a JSON literal (repeatable) |
| `--set-string` | stringArray | — | set key=value overrides forcing string interpretation (repeatable) |
| `--timeout` | duration | 5m0s | timeout for readiness wait |
| `-f, --values` | stringArray | — | values file overrides (repeatable) |
| `--wait` | bool | true | wait for resources to be ready |
| `--wait-for-jobs` | bool | false | wait for Job resources to complete (in addition to `--wait`) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Upgrade an existing release with a new overrides file:

```sh
keramos upgrade hello ./my-app -n prod -f new-overrides.yaml
```

Install-or-upgrade in one command (idempotent for CI), pulling from OCI at a specific tag:

```sh
keramos upgrade hello oci://ghcr.io/example/charts/my-app:1.3.0 --install -n prod
```

Upgrade and force replacement of immutable fields (e.g. `Service.spec.clusterIP`):

```sh
keramos upgrade hello ./my-app --force -n prod
```

Incremental upgrade — change only one value, keep everything else:

```sh
keramos upgrade hello ./my-app --only image.tag --set image.tag=1.5.1 -n prod
```

Reuse the previous release's merged values without re-supplying them:

```sh
keramos upgrade hello ./my-app --reuse-values -n prod
```

Reset to package defaults and start over:

```sh
keramos upgrade hello ./my-app --reset-values -n prod
```

## See also

- [`install`](install.md)
- [`rollback`](rollback.md)
- [`diff`](diff.md) — preview the changes
- [`plan`](plan.md) and [`apply`](apply.md) — capture and apply
- [`canary`](canary.md) — staged upgrade
- [`history`](history.md)
- [Quickstart](../guides/quickstart.md)
