# keramos-workspace.yaml

Groups several keramos packages from one directory tree into a workspace that
installs, upgrades, diffs, or uninstalls in dependency order with a single
command. The `keramos workspace` commands read it. Each member stays a separate
release — use a workspace for a repo of related packages, and use `layers` in
`keramos.yaml` when pieces should merge into one release.

## Minimal example

```yaml
apiVersion: keramos/v1
members:
  - name: postgres
    path: ./postgres
  - name: api
    path: ./api
    dependsOn: [postgres]
```

`keramos workspace install` then installs `postgres` before `api`.

## Fields

The file lives at the workspace root; member `path`s resolve against that root.

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `apiVersion` | string | no | — | Format identifier. Use `keramos/v1`. Parsed but not enforced. |
| `members` | list | yes | — | The packages in the workspace; must be non-empty. See below. |
| `defaults` | object | no | — | Values applied to any member that leaves the field unset. See [defaults](#defaults-fields). |

### Member fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `name` | string | yes | — | Identifier used in CLI output and `dependsOn` references, unique within the workspace. |
| `path` | string | yes | — | Path to the member package, relative to the workspace root (or absolute). The directory must contain a `keramos.yaml`. |
| `namespace` | string | no | `defaults.namespace` | Namespace this member's release installs into. |
| `profile` | string | no | `defaults.profile` | Profile to activate when rendering this member. |
| `dependsOn` | string list | no | — | Names of other members that must be applied before this one. Keramos computes the topological order; cycles are reported as errors. |
| `atomic` | bool | no | `defaults.atomic`, else `true` | Roll back this member's install if it fails. Set `false` to leave a failed install in place for inspection. |
| `wait` | bool | no | `defaults.wait`, else `true` | Wait for this member's resources to become Ready before moving on. |

### defaults fields

`defaults` fills any member field the member itself omits.

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `namespace` | string | no | — | Default namespace for members without one. |
| `profile` | string | no | — | Default profile for members without one. |
| `atomic` | bool | no | — | Default atomic setting for members without one. |
| `wait` | bool | no | — | Default wait setting for members without one. |

## Full example

```yaml
apiVersion: keramos/v1

# Applied to every member unless the member overrides the field.
defaults:
  namespace: platform
  atomic: true
  wait: true

members:
  - name: postgres
    path: ./postgres

  - name: redis
    path: ./redis

  - name: api
    path: ./services/api
    profile: production
    dependsOn: [postgres, redis]      # both must be Ready first

  - name: worker
    path: ./services/worker
    namespace: platform-jobs          # overrides defaults.namespace
    dependsOn: [postgres]
    wait: false                       # long-running; don't block on Ready
```

`keramos workspace plan` prints the order (members with no dependency between them
keep declared order):

```
1. postgres (path=./postgres, ns=platform, profile=)
2. redis (path=./redis, ns=platform, profile=)
3. api (path=./services/api, ns=platform, profile=production)
4. worker (path=./services/worker, ns=platform-jobs, profile=)
```

`keramos workspace install` applies them in that order; `uninstall` reverses it.

## See also

- [`keramos workspace plan`](../cli/workspace-plan.md) — preview the order.
- [`keramos workspace install`](../cli/workspace-install.md) / [`upgrade`](../cli/workspace-upgrade.md) / [`uninstall`](../cli/workspace-uninstall.md) / [`diff`](../cli/workspace-diff.md).
- [`keramos workspace status`](../cli/workspace-status.md).
- [keramos-releases.yaml](keramos-releases-yaml.md) — releases of unrelated packages.
- [Workspaces guide](../guides/workspaces.md).
