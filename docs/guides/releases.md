# Orchestrate cross-release dependencies

`keramos-releases.yaml` rolls out **several separate releases** with one
command, in dependency order. Unlike a workspace, the member packages can come
from anywhere — local paths, OCI registries, HTTPS chart URLs, or git — which
makes this the right file for standing up a platform built from unrelated
upstream packages that must come up in a strict order.

The file-format reference is
[`keramos-releases.yaml`](../reference/keramos-releases-yaml.md); the command
reference is [`keramos releases`](../cli/releases.md). This guide covers the
workflow.

## When to use it

Reach for `keramos-releases.yaml` when all of these hold:

- The releases come from **different sources** (some local, some OCI, some
  HTTPS).
- They have **dependency ordering** between them.
- Each is its own **independent release** with its own lifecycle.
- You want to bring them up and tear them down with **single commands**.

If they are sibling packages in one repo, prefer
[`keramos-workspace.yaml`](workspaces.md) — it adds parallelism, health gating,
and whole-set rollback. If they roll out on different schedules, keep them in
separate pipelines.

## Lay out the directory

```
my-platform-deploy/
├── keramos-releases.yaml
├── values/
│   ├── cert-manager.yaml
│   ├── external-dns.yaml
│   └── monitoring.yaml
└── packages/                   # optional: locally-developed packages
    └── ingress-nginx/
        ├── keramos.yaml
        └── ...
```

The directory holds the spec file, the value overrides for each release, and
any locally-developed packages. `values:` paths resolve relative to the
directory containing `keramos-releases.yaml`.

## Write the spec

```yaml
# keramos-releases.yaml
releases:
  - name: cert-manager
    package: oci://quay.io/jetstack/cert-manager
    namespace: cert-manager
    values: [./values/cert-manager.yaml]
    set:
      - installCRDs=true

  - name: external-dns
    package: oci://ghcr.io/kubernetes-sigs/external-dns
    namespace: external-dns
    values: [./values/external-dns.yaml]
    dependsOn: [cert-manager]

  - name: ingress
    package: ./packages/ingress-nginx
    namespace: ingress
    dependsOn: [cert-manager]

  - name: monitoring
    package: oci://ghcr.io/example/kube-prometheus-stack
    namespace: monitoring
    values:
      - ./values/monitoring-base.yaml
      - ./values/monitoring-overrides.yaml
    dependsOn: [ingress]
```

Each entry carries a `name`, a `package` source, and optionally `namespace`,
`profile`, `values`, `set`, and `dependsOn`. A release without its own
`namespace` falls back to `-n/--namespace`. Every `dependsOn` name must match
another entry; an unknown name or a cycle is an error and nothing is applied.

## Preview the order

```sh
keramos releases plan
```

```
1. cert-manager (oci://quay.io/jetstack/cert-manager) ns=cert-manager
2. external-dns (oci://ghcr.io/kubernetes-sigs/external-dns) ns=external-dns
3. ingress (./packages/ingress-nginx) ns=ingress
4. monitoring (oci://ghcr.io/example/kube-prometheus-stack) ns=monitoring
```

`plan` prints a flat, numbered list — dependencies before dependents. Releases
that do not depend on each other are ordered alphabetically by name. It
contacts no cluster, so it is safe as a CI sanity check: a cycle or an unknown
`dependsOn` fails it before anything runs.

## Install

```sh
keramos releases install
```

```
[cert-manager] installed (revision 1, ns cert-manager)
[external-dns] installed (revision 1, ns external-dns)
[ingress] installed (revision 1, ns ingress)
[monitoring] installed (revision 1, ns monitoring)
```

Keramos applies the releases strictly in order — one at a time, each after
everything it depends on. Every release is atomic: if one fails to install, it
rolls **itself** back, the command stops, and the releases already installed
stay in place. Re-running install picks up where it left off.

`install` does **not** wait for a release's pods to become Ready before moving
to the next one. If a dependent must not start until its dependency is actually
serving traffic, use [`keramos workspace`](workspaces.md) with `--health-gate`.

## Keep the platform current

`keramos releases upgrade` is the one command you re-run to converge the set: it
upgrades every release that exists and installs any that do not.

```sh
keramos releases upgrade
```

```
[cert-manager] upgraded (revision 2)
[external-dns] upgraded (revision 2)
[ingress] upgraded (revision 2)
[monitoring] upgraded (revision 2)
```

A release that was missing prints `[name] installed (revision 1, ns ...)`
instead. This works the first time and every time after, so it is safe as your
repeatable CI deploy command.

## Check status

```sh
keramos releases status
```

```
cert-manager: revision 1 status=deployed
external-dns: revision 1 status=deployed
ingress: revision 1 status=deployed
monitoring: not deployed
```

Status reads each release's latest revision from the cluster and prints one
line per entry, in file order. A release with no record shows `not deployed` —
a signal to run `install` or `upgrade`. A reachable cluster is required.

## Tear down

```sh
keramos releases uninstall
```

```
[monitoring] uninstalled
[ingress] uninstalled
[external-dns] uninstalled
[cert-manager] uninstalled
```

Uninstall runs in **reverse** order, so nothing is removed while a still-present
release depends on it. A release that is already gone is treated as success.
Unlike install, uninstall keeps going past a failure — one bad release does not
strand the rest.

## Point at a different file

Every subcommand reads `keramos-releases.yaml` in the current directory. Use
`--file` to select another path:

```sh
keramos releases install --file ./platform.releases.yaml
```

## How it differs from workspaces

| | `keramos releases` | [`keramos workspace`](workspaces.md) |
|---|---|---|
| Spec file | `keramos-releases.yaml` | `keramos-workspace.yaml` |
| Member packages from | anywhere — local, OCI, HTTPS, git | sibling directories of one repo |
| Field for the package | `releases[].package` | `members[].path` |
| Per-member overrides | `values:` and `set:` on the entry | member's own `values.yaml` + `profile` |
| Ordering | topological, **sequential** | topological, **parallel** within a level (`--parallel`) |
| Wait for pods to be Ready | no | optional (`--health-gate`) |
| Roll back the whole set | no (each release is atomic on its own) | optional (`--atomic-workspace`) |
| Continue past a failure | install/upgrade stop; uninstall continues | optional (`--continue-on-error`) |
| Whole-set dry run / diff | no | yes (`--dry-run`, `keramos workspace diff`) |

Both files are first-class, and a project can use both: a workspace for the
tightly-coupled packages from one repo, and a `keramos-releases.yaml` to roll out
that workspace alongside a few external releases.

## See also

- [`keramos releases`](../cli/releases.md) — command reference
- [`keramos-releases.yaml`](../reference/keramos-releases-yaml.md) — file-format
  reference
- [Workspaces](workspaces.md) — richer orchestration for one repo of packages
- [`keramos install`](../cli/install.md) · [`keramos upgrade`](../cli/upgrade.md) —
  the single-release commands each entry runs
