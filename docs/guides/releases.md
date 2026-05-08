# Cross-release dependencies

`keramos-releases.yaml` orchestrates **multiple separate releases** with an explicit dependency graph between them. Where a workspace expects member packages to live in one repository, `keramos-releases.yaml` happily mixes local paths, OCI registries, HTTPS chart URLs, and git references — useful when you need to roll out a platform composed of unrelated upstream packages with strict ordering between them.

The full file-format reference is at [`keramos-releases.yaml`](../reference/keramos-releases-yaml.md). This guide covers the workflow.

## When to use it

Reach for `keramos-releases.yaml` when **all** of these are true:

- The releases come from **different sources** (some local, some OCI, some HTTPS).
- They have **dependency ordering** between them.
- Each is its own **independent release** with its own lifecycle.
- You want to roll them out and tear them down with **single commands**.

If they're sibling packages in one repo, prefer `keramos-workspace.yaml`. If they're independently rolled out by different teams on different schedules, don't put them in one file at all — use a controller or a CI pipeline.

## Layout

```
my-platform-deploy/
├── keramos-releases.yaml
├── values/
│   ├── cert-manager.yaml
│   ├── external-dns.yaml
│   └── monitoring.yaml
├── pulled/                     # optional: locally-cached pulled packages
└── packages/                   # optional: locally-developed packages
    └── ingress-nginx/
        ├── keramos.yaml
        └── ...
```

The directory contains the file, value overrides for each release, and any locally-developed packages.

## Example: platform bootstrap

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

## Commands

```
keramos releases plan       # show topological order, no changes
keramos releases install    # install all releases in order
keramos releases upgrade    # upgrade existing, install missing
keramos releases uninstall  # reverse-order uninstall
keramos releases status     # current state of every release
```

The `--file` flag selects the manifest path; default is `keramos-releases.yaml` in the current directory.

```sh
keramos releases install --file ./platform.releases.yaml
```

## Plan output

```
$ keramos releases plan
Level 0:
  - cert-manager       ← oci://quay.io/jetstack/cert-manager
Level 1:
  - external-dns       ← oci://ghcr.io/kubernetes-sigs/external-dns
  - ingress            ← ./packages/ingress-nginx
Level 2:
  - monitoring         ← oci://ghcr.io/example/kube-prometheus-stack
```

Same Kahn-level grouping as workspaces: members within a level have no inter-dependencies among themselves.

## Install ordering

```sh
keramos releases install
```

Keramos installs level-by-level. Releases at the same level are issued sequentially (in the order they appear in the file); every release in level N must complete before level N+1 begins. Each individual release goes through the standard `keramos install` pipeline — including hooks, schema validation, server-side apply, and readiness wait — so by the time a release is reported as installed, its workloads are Ready. There's no need for an explicit health-gate flag because the per-release `--wait` is the default.

For finer-grained parallelism within a level (say you have 5 cert-manager-style releases that can all install at once), prefer `keramos workspace`, which exposes `--parallel`, `--health-gate`, and `--atomic-workspace` flags.

## Upgrade vs install

`keramos releases upgrade` is a hybrid: each release is upgraded if it already exists, installed if it doesn't. The behaviour matches running `keramos upgrade --install`-style logic per release. Useful in CI: the same command brings the platform up the first time and keeps it up-to-date thereafter.

## Per-release values and set

Each release entry can carry its own `values:` and `set:`:

```yaml
releases:
  - name: monitoring
    package: oci://ghcr.io/example/kube-prometheus-stack
    namespace: monitoring
    values:
      - ./values/monitoring-base.yaml
      - ./values/monitoring-${ENVIRONMENT}.yaml    # variable expansion against env
    set:
      - "alertmanager.config.global.resolve_timeout=5m"
      - "grafana.adminPassword=${GRAFANA_ADMIN_PASSWORD}"
```

`values:` paths are resolved relative to the directory containing `keramos-releases.yaml`. Variable expansion (`${ENVIRONMENT}`, `${GRAFANA_ADMIN_PASSWORD}`) happens against the calling shell's environment, so secrets stay in env vars rather than the file.

## Differences from workspaces

| | `keramos-workspace.yaml` | `keramos-releases.yaml` |
|---|---|---|
| Member packages from | sibling directories of one repo | anywhere — local, OCI, HTTPS, git |
| Field name | `members[].path` | `releases[].package` |
| Per-member overrides | values come from the member's own `values.yaml` and the workspace's selected environment | `values:` and `set:` directly on the entry |
| `atomic` / `wait` per-member | yes (`atomic`, `wait` on member) | per-release behaviour matches `keramos install` defaults |
| Profiles | per-member `profile:` | per-release `profile:` |
| Parallel within a level | `--parallel` flag | sequential within a level |
| Health-gate | `--health-gate` flag | per-release `--wait` is the default |
| Atomic / continue-on-error | `--atomic-workspace` / `--continue-on-error` flags | not exposed |

Both files are first-class — and a project can use both: a workspace for tightly-coupled packages from one repo, and a `keramos-releases.yaml` to orchestrate the workspace plus a few external releases together.

## Patterns

### Bootstrap a fresh cluster

A `keramos-releases.yaml` is the natural unit to commit alongside cluster-bootstrap automation. The first cluster operator runs:

```sh
git clone https://github.com/example/platform.git
cd platform
keramos releases install
```

All operators of every cluster in the fleet end up with the same set of releases at the same versions.

### Drift detection across the platform

```sh
keramos releases status                    # any release not at expected status?
for r in $(keramos releases plan -q); do keramos drift $r; done
```

The first checks release status; the second runs drift on every release named in the plan.

### Tearing down

```sh
keramos releases uninstall
```

Reverse-level order. Members of the last level uninstalled first, levels above wait for theirs to complete.
