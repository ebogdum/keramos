# keramos install

## Synopsis

`keramos install` applies a keramos package to the cluster as a brand-new release. The release name is what keramos (and operators) will use to refer to this deployment forever after — for upgrades, rollbacks, status checks, drift reports, and uninstall. The package directory is rendered against merged values, validated against `values.schema.json` if present, then applied to the cluster with server-side apply. A release record (a labelled Secret in the install namespace) captures the rendered manifest, merged values, audit metadata, and rendered hooks so subsequent `upgrade` and `rollback` operations have everything they need.

## When to use it

Use this when you are deploying a keramos package for the first time under a given release name. If a release with that name already exists in the namespace, the command refuses; use `keramos upgrade` instead. For an idempotent "install if missing, upgrade if present" workflow, see `keramos upgrade --install` semantics on the upgrade page.

## Usage

```
keramos install <release-name> <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--api-versions` | stringArray | — | Kubernetes API version available for capability checks (repeatable) |
| `--cleanup-on-fail` | — | — | delete partially-applied resources if the install fails |
| `--create-namespace` | — | — | create the release namespace if missing |
| `--description` | string | — | release description |
| `--dry-run` | string | — | dry-run mode: 'client' (local render) or 'server' (API validation) |
| `--env` | string | — | environment name declared in keramos.yaml environments: (replaces values-{env}.yaml) |
| `--force` | — | — | delete and recreate resources to force update of immutable fields |
| `--generate-name` | — | — | generate a release name from the package name |
| `-h, --help` | — | — | help for install |
| `--history-max` | int | — | maximum number of revisions to retain in history (0 = unlimited) |
| `--hook-timeout` | duration | — | cap each hook's per-hook timeout (0 = use the chart-declared value) |
| `--include-crds` | — | — | include CRDs from crds/ in the rendered manifest |
| `--keyring` | string | — | path to PGP keyring directory for --verify (default: ~/.config/keramos/keyring) |
| `--kube-version` | string | — | override Kubernetes version reported in capabilities |
| `--labels` | stringArray | — | label key=value to attach to the release (repeatable) |
| `--no-atomic` | — | — | don't roll back on failure |
| `--no-force` | — | — | don't force field ownership on server-side apply |
| `--no-hooks` | — | — | skip lifecycle hooks for this operation |
| `--no-wait` | — | — | don't wait for resources to be ready |
| `-o, --output` | string | "table" | output format: table, json, yaml |
| `--post-renderer` | string | — | command piped the rendered manifests on stdin (yields stdout) |
| `--post-renderer-timeout` | duration | 5m0s | per-stage timeout for post-renderers |
| `--post-renderers` | stringArray | — | chained post-renderers (repeatable; output of N feeds N+1) |
| `--profile` | string | — | profile name to apply |
| `--recreate-pods` | — | — | trigger a rolling restart of Deployments/StatefulSets/DaemonSets |
| `--set` | stringArray | — | set key=value overrides (repeatable) |
| `--set-file` | stringArray | — | set key=path; the value is read from path (repeatable) |
| `--set-json` | stringArray | — | set key=<json>; value is parsed as a JSON literal (repeatable) |
| `--set-string` | stringArray | — | set key=value overrides forcing string interpretation (repeatable) |
| `--skip-requires` | — | — | skip installation of required co-deployed packages |
| `--timeout` | duration | 5m0s | timeout for readiness wait |
| `-f, --values` | stringArray | — | values file overrides (repeatable) |
| `--verify` | — | — | verify package signatures before installing |
| `--wait` | — | behaviour | wait for resources to be ready |
| `--wait-for-jobs` | — | — | wait for Job resources to complete (in addition to --wait) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Install from a local package directory, creating the namespace if missing:

```sh
keramos install my-app ./my-app -n my-app-prod --create-namespace
```

Install from an OCI registry at a specific tag, verifying the package's PGP signature first:

```sh
keramos install my-app oci://ghcr.io/example/charts/my-app:1.2.3 -n my-app-prod --verify
```

Install with values overrides and a non-default profile:

```sh
keramos install my-app ./my-app -f overrides.yaml --set replicas=5 --profile ha-3node -n prod
```

Render-only sanity check before commit (no cluster contact):

```sh
keramos install my-app ./my-app --dry-run client
```

## See also

- [`upgrade`](upgrade.md) — apply changes to an existing release
- [`uninstall`](uninstall.md)
- [`diff`](diff.md) — preview changes
- [`plan`](plan.md) and [`apply`](apply.md)
- [Quickstart](../guides/quickstart.md)
- [Values](../guides/values.md)
