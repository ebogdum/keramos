# keramos install

`keramos install` renders a package directory and applies it to the cluster as a
new named release — revision 1 of that release's history.

## When to use it

- To deploy a package for the first time under a name you choose.
- To stand up a release into a fresh namespace in one step (`--create-namespace`).
- To render and validate without touching the cluster (`--dry-run client`).

## What happens

1. Resolves the package at `<package-path>`, merges values (package defaults +
   `--env` + `--profile` + `-f` + `--set*`), and validates them against the
   package's JSON schema if one is present.
2. Renders the templates and hooks, then evaluates package policies against the
   rendered manifest — a deny policy stops the install.
3. Stores the release as revision 1 in pending state (this writes state).
4. Runs pre-install hooks, applies CRDs first, then the remaining manifests via
   server-side apply.
5. Waits for every resource to become Ready unless `--no-wait` is set, then runs
   post-install hooks and marks the release deployed.
6. Installs any required co-deployed packages unless `--skip-requires`.

Requires a reachable cluster except under `--dry-run client`, which renders
locally and applies nothing. On failure the release rolls back automatically
unless `--no-atomic` is set.

## Usage

```
keramos install <release-name> <package-path> [flags]
```

With `--generate-name`, pass only `<package-path>`; the release name is derived
from the package name plus a random suffix.

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-f, --values` | stringArray | — | values file override; repeatable, later files win |
| `--set` | stringArray | — | `key=value` override (repeatable) |
| `--set-string` | stringArray | — | `key=value` forced to string (repeatable) |
| `--set-file` | stringArray | — | `key=path`; value is read from the file (repeatable) |
| `--set-json` | stringArray | — | `key=<json>`; value parsed as a JSON literal (repeatable) |
| `--profile` | string | — | profile to apply on top of defaults |
| `--env` | string | — | environment from `keramos.yaml`'s `environments:` (replaces `values-{env}.yaml`) |
| `--wait` | — | on | wait for every resource to be Ready (default behaviour) |
| `--no-wait` | — | — | return once resources are applied, without waiting for Ready |
| `--wait-for-jobs` | — | — | also block until Job resources complete |
| `--timeout` | duration | 5m0s | how long the readiness wait may run before failing |
| `--dry-run` | string | — | `client` renders locally; `server` also validates against the API |
| `-o, --output` | string | table | result format: `table`, `json`, or `yaml` |
| `--description` | string | — | free-text note stored on revision 1 |
| `--no-atomic` | — | — | leave partial changes in place on failure instead of rolling back |
| `--no-force` | — | — | don't force field ownership on server-side apply |
| `--no-hooks` | — | — | skip all lifecycle hooks for this install |
| `--create-namespace` | — | — | create the target namespace if it doesn't exist |
| `--include-crds` | — | — | include CRDs from `crds/` in the rendered manifest |
| `--labels` | stringArray | — | `key=value` label recorded on the release (repeatable) |
| `--api-versions` | stringArray | — | extra API versions to report as available in capability checks (repeatable) |
| `--kube-version` | string | — | override the Kubernetes version reported to templates |
| `--post-renderer` | string | — | command fed the manifest on stdin; its stdout is applied |
| `--post-renderers` | stringArray | — | chained post-renderers; output of N feeds N+1 (repeatable) |
| `--post-renderer-timeout` | duration | 5m0s | per-stage timeout for each post-renderer |
| `--cleanup-on-fail` | — | — | delete resources this install created if it fails |
| `--recreate-pods` | — | — | trigger a rolling restart of Deployments/StatefulSets/DaemonSets |
| `--force` | — | — | delete and recreate resources to update immutable fields |
| `--hook-timeout` | duration | 0 | cap each hook's timeout (0 = use the chart-declared value) |
| `--keyring` | string | `~/.config/keramos/keyring` | PGP keyring directory used by `--verify` |
| `--generate-name` | — | — | derive the release name from the package (omit `<release-name>`) |
| `--verify` | — | — | verify the package's signatures before installing |
| `--skip-requires` | — | — | don't install the package's required co-deployed packages |
| `--history-max` | int | 0 | max revisions to retain in history (0 = unlimited) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | namespace to install into |

## Worked example — inputs and the release they produce

**INPUT 1 — the package (`./web`).** `keramos.yaml` names it and `values.yaml`
sets a default replica count:

```yaml
# ./web/keramos.yaml
name: web
version: 1.0.0

# ./web/values.yaml
replicas: 2          # ← default in the package
```

**INPUT 2 — the command.** You override the replica count and ask for the
namespace to be created:

```sh
keramos install web ./web -n apps --create-namespace --set replicas=3
```

There is no third input: `install` writes revision 1 from scratch, so there is
no prior state to read.

**OUTPUT:**

```
release web installed (revision 1)
```

**State written** (the stored release `install` created):

```yaml
# keramos get web -n apps  →  the recorded revision 1
name: web
namespace: apps
revision: 1
status: deployed
```

**Tracing every line back to the inputs:**

| Output / state | Which input it came from | Why |
|---|---|---|
| `release web` | INPUT 2 `<release-name>` | the name you passed |
| `installed (revision 1)` | fresh install | the first apply of a name is always revision 1 |
| namespace `apps` | INPUT 2 `-n apps` + `--create-namespace` | the namespace was created and used |
| `replicas: 3` in the applied Deployment | INPUT 2 `--set replicas=3` | your override beat the package default of 2 |
| `status: deployed` | readiness wait passed | `--wait` (default) confirmed every resource is Ready |

Render and validate without touching the cluster:

```sh
keramos install web ./web --dry-run client
```

## See also

- [`upgrade`](upgrade.md) — apply a new revision to an existing release
- [`uninstall`](uninstall.md) — remove a release
- [`plan`](plan.md) — preview what an install or upgrade would change
- [`status`](status.md) — inspect a release after installing
