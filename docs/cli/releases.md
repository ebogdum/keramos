# keramos releases

## Synopsis

`keramos releases` installs, upgrades, and uninstalls a whole set of related
releases with one command, in dependency order. You list the releases — and
which one depends on which — in a `keramos-releases.yaml` file, and keramos works out
the order for you, so a database release comes up before the API release that
needs it, and comes down last.

Five subcommands drive one spec file:

```
plan       show the order the releases will be applied in (no cluster needed)
install    install every release, in order
upgrade    upgrade every release (installing any that are missing)
uninstall  uninstall every release, in reverse order
status     show the current revision and status of every release
```

Every subcommand reads the same file (`--file`, default `keramos-releases.yaml`).

## How it relates to `keramos workspace`

Both commands read one YAML file that lists packages with `dependsOn` and act
on them in dependency (topological) order. They differ in how much
orchestration they give you — pick by what you need:

| | `keramos releases` | `keramos workspace` |
|---|---|---|
| Spec file | `keramos-releases.yaml` | `keramos-workspace.yaml` |
| Ordering | topological, **sequential** | topological, **parallel** within a level (`--parallel`) |
| Wait for pods to be ready | no | optional (`--health-gate` between levels) |
| Roll back the whole set on failure | no (each release is atomic on its own) | optional (`--atomic-workspace`) |
| Keep going past a failure | no (install/upgrade stop) | optional (`--continue-on-error`) |
| Dry run / whole-set diff | no | yes (`--dry-run`, `keramos workspace diff`) |

Use `keramos releases` for a simple, ordered bundle. Reach for
[`keramos workspace`](workspace.md) when you need parallel rollout, health gating
between tiers, cross-member rollback, or a dry-run of the whole set.

## Subcommands

| Command | What it does |
|---|---|
| [`plan`](releases-plan.md) | print the order the releases will be applied in |
| [`install`](releases-install.md) | install every release in dependency order |
| [`upgrade`](releases-upgrade.md) | upgrade every release, installing any that are missing |
| [`uninstall`](releases-uninstall.md) | uninstall every release in reverse order |
| [`status`](releases-status.md) | show each release's current revision and status |

## Usage

```
keramos releases <command> [flags]
```

The set is described by `keramos-releases.yaml` in the current directory (or the
path you pass to `--file`):

```yaml
releases:
  - name: postgres          # release name keramos records it under
    package: ./charts/postgres
    namespace: data         # falls back to -n / --namespace if omitted

  - name: api
    package: ./charts/api
    namespace: apps
    profile: prod           # profile to render with
    values:                 # values files, applied in order
      - prod.yaml
    set:                    # inline overrides, key=value
      - replicas=3
    dependsOn:              # api is applied after these
      - postgres
```

Each `dependsOn` name must match another `name:` entry in the file; an unknown
name or a dependency cycle is reported as an error and nothing is applied.
Releases that do not depend on each other are ordered by name.

Preview the order, apply the set, then check where it stands:

```sh
keramos releases plan
keramos releases install
keramos releases status
```

## See also

- [`workspace`](workspace.md) — richer multi-package orchestration (parallel
  rollout, health gates, rollback)
- [`install`](install.md) — install a single release
- [`upgrade`](upgrade.md) — upgrade a single release
- [`list`](list.md) — list every release in the cluster
