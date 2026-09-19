<!--
SEO meta-summary (used by GitHub's social preview and surfaced as the
default Google snippet via the og:description fallback). The first 155
characters of this paragraph are what Google indexes most heavily.
-->

# Keramos — Kubernetes Package Manager, Helm Alternative, and YAML Templating Engine

> **Keramos is an open-source Kubernetes package manager, YAML templating engine, and modern Helm alternative** that ships, upgrades, rolls back, signs, and reconciles Kubernetes workloads through a single CLI. It produces deterministic manifests from layered packages, runs drift detection against live clusters, signs releases with PGP or cosign, integrates cleanly with Argo CD and Flux, and migrates existing Helm charts via `keramos migrate`. **A modern alternative to Helm, kustomize, kapp, kpt, ytt, or stitching shell-rendered templates into release flows by hand.**

[![Go Version](https://img.shields.io/badge/go-1.25%2B-blue?logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Built for Kubernetes](https://img.shields.io/badge/Kubernetes-1.25--1.32-blue?logo=kubernetes)](https://kubernetes.io)
[![Helm Alternative](https://img.shields.io/badge/Helm-alternative-blueviolet)](docs/comparison.md#keramos-as-a-helm-alternative)

**Looking for** a Helm alternative · open-source Helm replacement · modern Kubernetes package manager · YAML templating tool for Kubernetes · Helm chart converter and migrator · drift-detection tool · release-management CLI · signed-artifact distribution for Kubernetes · GitOps-friendly packaging · multi-cluster deployment orchestrator · OCI-distributed Kubernetes packages · audit-trail-aware deployment tool · Kubernetes app deployer? **Keramos is for you.**

---

## Table of contents

- [What keramos does](#what-keramos-does) — features at a glance
- [Why keramos](#why-keramos) — what makes it different from kustomize, kapp, kpt, and other Kubernetes packaging tools
- [Quick install](#quick-install) — `go install` one-liner and build-from-source
- [Five-minute tour](#five-minute-tour) — first install through rollback in 10 minutes
- [What is a keramos package?](#what-is-a-keramos-package) — the file layout
- [Composing packages with layers](#composing-packages-with-layers) — package composition
- [Environments, profiles, values](#environments-profiles-values) — config management
- [Workspaces and cross-release graphs](#workspaces-and-cross-release-graphs) — multi-package orchestration
- [Distribution: OCI, HTTP repos, and signing](#distribution-oci-http-repos-and-signing) — supply chain
- [GitOps integration](#gitops-integration) — Argo CD and Flux
- [The CLI](#the-cli) — every command at a glance
- [Keramos as a Helm alternative](#keramos-as-a-helm-alternative) — comparison + migration path from Helm
- [FAQ](#faq) — common questions
- [Documentation map](#documentation-map) — full doc tree
- [Comparison with other Kubernetes packaging tools](#comparison-with-other-kubernetes-packaging-tools)
- [License](#license)

---

## What keramos does

Keramos is a **Kubernetes package manager and templating engine** for teams that ship workloads to one or many clusters. In one binary it gives you:

- **YAML templating for Kubernetes** — `${...}` expression-based substitutions inside YAML manifests; no go-templates that produce broken syntax.
- **Layered package composition** — assemble one Kubernetes deployment from multiple reusable packages (think mixin / overlay system, but type-safe and value-merging).
- **Multi-environment configuration management** — `dev`, `staging`, `prod` and their inheritance live inside `keramos.yaml`; one command activates the right values, namespace, and kubeconfig context.
- **Kubernetes drift detection** — compare the live cluster state to the stored release manifest, see exactly which fields drifted, and reconcile back to known-good with one command.
- **Release management** — versioned, revisioned releases with full history, rollback, and per-revision audit data (who installed, when, from where, with what flags).
- **Lifecycle hooks** — pre-install / post-install / pre-upgrade / post-upgrade / pre-rollback / post-rollback / pre-delete / post-delete / test, scoped per release and per revision.
- **Workspace orchestration** — install/upgrade/uninstall many Kubernetes packages in topological dependency order, with parallelism and health-gating between levels.
- **Multi-cluster deployment** — ship the same release to N clusters from one invocation.
- **Signed Kubernetes packages** — PGP `.prov` provenance files and OCI-attached cosign signatures, verified at pull and install time.
- **OCI-distributed packages** — push and pull Kubernetes packages from any OCI distribution-spec registry (GHCR, Quay, Harbor, ECR, GAR, ACR, Distribution).
- **HTTP repository hosting** — serve packages from any HTTP(S) directory with an `index.yaml`; works with GitHub Pages, S3, ChartMuseum-shaped servers.
- **GitOps-ready** — produces stable rendered manifests for Argo CD or Flux, exposes plan/apply for change-management workflows, and a KeramosRelease CRD for in-cluster reconciliation.
- **Built-in policy, lint, SBOM, scan, dev-loop, canary** — fewer external tools to glue together for a real platform engineering workflow.

---

## Why keramos

Kubernetes manifests are **configuration data**, but most tools treat them as *strings*: text-templated, regex-substituted, hand-edited file by file. Keramos treats them as data:

- **Expression-based templating.** `${...}` substitutions inside YAML, with a typed expression language (string, number, bool, list, map, function call, pipeline, dotted lookup). Output is always valid YAML; there are no template-emitted tokens that produce broken syntax.
- **Layered composition.** A package can pull in other packages as layers (`layers:` in `keramos.yaml`). Values merge top-down; templates merge by file. The result is one rendered manifest belonging to one release, but assembled from reusable building blocks.
- **First-class environments.** `dev`, `staging`, `prod` and their inheritance live in `keramos.yaml` itself, not in side files. `keramos install --env staging` resolves the right values, namespace, and kubeconfig context.
- **Strict ownership semantics.** Every resource keramos applies, and every namespace keramos creates, is stamped with the label `managedBy=keramos`. Cluster-wide ownership queries (`kubectl get all -A -l managedBy=keramos`) are O(label index), not pattern-match guessing.
- **Dependency-aware orchestration.** `keramos workspace` runs many releases level-by-level using a topological sort; `keramos releases` orchestrates entirely separate releases (often from separate sources) the same way.
- **Drift detection and reconcile.** `keramos drift` reports any difference between the stored manifest of a release and live cluster state; `keramos reconcile` re-applies the stored manifest to converge.
- **Provenance and audit trail.** Every release records who installed it, with what flags, from what keramos version, against what kubeconfig context, at what time.
- **Signed packages.** Producers sign with PGP or cosign; consumers verify before install. The trust store is local and operator-controlled.
- **Built-in lint, plan, diff, audit, SBOM, scan, drift, dev-loop, canary, multi-cluster install** — fewer external tools to glue together.

---

## Quick install

### Download a binary (no toolchain required)

Every release ships prebuilt binaries for Linux, macOS, Windows, FreeBSD and
OpenBSD on amd64 and arm64, with a `SHA256SUMS` file beside them.

```sh
VER=$(curl -fsSI https://github.com/ebogdum/keramos/releases/latest \
  | tr -d '\r' | awk -F/ '/^location:/ {print $NF}')
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')

curl -fsSL -O "https://github.com/ebogdum/keramos/releases/download/$VER/keramos-$VER-$OS-$ARCH.tar.gz"
curl -fsSL -O "https://github.com/ebogdum/keramos/releases/download/$VER/SHA256SUMS"
grep " keramos-$VER-$OS-$ARCH.tar.gz$" SHA256SUMS | shasum -a 256 -c -

tar -xzf "keramos-$VER-$OS-$ARCH.tar.gz"
sudo install -m 0755 "keramos-$VER-$OS-$ARCH/keramos" /usr/local/bin/keramos
keramos version
```

Windows builds are `.zip` archives of the same naming shape.

### `go install`

For a Go toolchain you already have:

```sh
go install github.com/ebogdum/keramos/v3/cmd/keramos@latest
```

Binary lands in `$GOBIN` (or `$(go env GOPATH)/bin`). Go 1.25 or later required.
A binary built this way reports its module version but carries no commit or
build date, because the Go toolchain does not stamp them.

### Build from source

```sh
git clone https://github.com/ebogdum/keramos.git
cd keramos
go build -o ./keramos ./cmd/keramos
sudo install -m 0755 ./keramos /usr/local/bin/keramos
keramos version
```

### Verify

```sh
keramos version
keramos --help
```

---

## Five-minute tour

A complete create → install → upgrade → roll-back loop against any Kubernetes cluster (`kind`, `k3d`, `minikube`, EKS, GKE, AKS — all work).

```sh
# 1. Scaffold a new Kubernetes package
keramos create my-app && cd my-app

# 2. Render templates locally (no cluster contact yet)
keramos template .

# 3. Install into a cluster
keramos install my-app . -n my-app-prod --create-namespace

# 4. List your releases
keramos list -A

# 5. Edit values.yaml or any template, then upgrade
keramos upgrade my-app .

# 6. See what drifted in-cluster since install
keramos drift my-app

# 7. Roll back to the previous revision
keramos rollback my-app 1

# 8. Uninstall
keramos uninstall my-app
```

→ Step-by-step walkthrough: [Quickstart guide](docs/guides/quickstart.md).

---

## What is a keramos package?

A keramos package is a directory with this layout (everything past the required three is optional):

```
my-app/
├── keramos.yaml             # required — package manifest (name, version, layers, environments)
├── values.yaml           # required — default configuration for the package
├── templates/            # required — YAML manifests with ${...} expressions
│   ├── deployment.yaml
│   ├── service.yaml
│   └── _helpers.yaml     # underscore prefix = partial, included from other templates
├── values.schema.json    # optional — JSON Schema validation for values
├── crds/                 # optional — CRDs applied first (waits for Established=true)
├── hooks/                # optional — Kubernetes Job/Pod hooks (pre-install, post-upgrade, ...)
├── tests/                # optional — `keramos test` Pods
├── files/                # optional — embedded files reachable from templates
├── notes.yaml            # optional — post-install message template
├── profiles/             # optional — values overlay files (--profile prod)
├── policies/             # optional — package-level policies (keramos-native declarative rules)
├── README.md             # optional — human-facing docs
├── LICENSE               # optional
└── keramos.lock             # auto-generated — pinned layer/dependency digests; commit it
```

→ Full reference: [`keramos.yaml`](docs/reference/keramos-yaml.md), [`values.yaml`](docs/reference/values-yaml.md), [`values.schema.json`](docs/reference/values-schema-json.md).

---

## Composing packages with layers

The flagship feature: assemble a Kubernetes deployment from **multiple reusable packages**.

```yaml
# my-app/keramos.yaml
apiVersion: keramos/v1
name: my-app
version: 1.0.0

layers:
  - name: shared-base
    source: ../shared-base               # local-path layer
  - name: redis
    source: oci://ghcr.io/example/redis-layer
    version: ^2.0.0
    condition: redis.enabled              # only included when values.redis.enabled is truthy
  - name: monitoring
    source: git::https://github.com/example/monitoring-layer.git
    ref: v3.1.0
    tags: [observability]                 # included when values.tags.observability is truthy
```

Templates and values from every layer compose into **one rendered manifest** for a single release. Use `requires:` instead of `layers:` if the pieces should be **separate releases** that the parent depends on.

→ Full guide: [Layers](docs/guides/layers.md). Composition vs sub-charts vs workspaces: [Workspaces guide](docs/guides/workspaces.md).

---

## Environments, profiles, values

Keramos's configuration model maps cleanly to real platform-engineering practice:

```yaml
# keramos.yaml
environments:
  dev:
    namespace: my-app-dev
    values: { replicas: 1, image: { tag: latest } }
  staging:
    inherits: dev
    namespace: my-app-staging
    valueFiles: [profiles/staging.yaml]
    values: { replicas: 2 }
  prod:
    inherits: staging
    namespace: my-app
    cluster: prod-cluster                    # default kubeconfig context
    values: { replicas: 5, image: { tag: 1.4.2 } }
```

Activate with `keramos install --env prod`. The merge order is package defaults → inherited environments → environment's `valueFiles` → environment's `values` → CLI `-f` files → CLI `--set`.

→ Values reference: [`values.yaml`](docs/reference/values-yaml.md), guide: [Values](docs/guides/values.md).

---

## Workspaces and cross-release graphs

Two scopes for orchestrating multiple packages with one command:

- **`keramos-workspace.yaml`** — many member packages from one repo, with `dependsOn` and topological install. See [`keramos-workspace.yaml`](docs/reference/keramos-workspace-yaml.md).
- **`keramos-releases.yaml`** — many releases from anywhere (local paths, OCI, HTTPS, git), with the same `dependsOn` semantics. Run via `keramos releases install/upgrade/uninstall`. See [`keramos-releases.yaml`](docs/reference/keramos-releases-yaml.md).

Both honour the same primitives — Kahn-level grouping, parallelism per level, optional health-gate between levels, optional atomic rollback on failure.

---

## Distribution: OCI, HTTP repos, and signing

| Distribution model | Build | Push | Install from |
|---|---|---|---|
| **`.keramos.tgz` archive on any HTTP(S) server** | `keramos package ./my-app` | upload directory + `keramos repo index .` | `keramos pull <chart> --repo URL` then `keramos install` |
| **OCI registry** (GHCR, Docker Hub, Quay, Harbor, ECR, GAR, ACR, self-hosted) | `keramos package ./my-app` | `keramos registry push <archive> oci://...` | `keramos install my-app oci://host/path:1.2.3` |
| **PGP signed package** | `keramos package ./my-app --sign --key ...` | (any) | `keramos install ... --verify` |
| **Cosign-signed OCI artifact** | `cosign sign ghcr.io/...:1.2.3` after `keramos registry push` | (any) | external `cosign verify` then `keramos install` |

→ [Repositories guide](docs/guides/repositories.md), [OCI guide](docs/guides/oci.md), [Signing guide](docs/guides/signing.md).

---

## GitOps integration

Keramos works alongside Argo CD and Flux, not against them.

- **Argo CD users** can use `keramos plan` to render a deterministic manifest and let Argo CD sync it; `keramos diff` shows exactly what would change.
- **Flux users** can build packages with `keramos` then pull them via Flux's OCI source and apply with kustomize-controller, OR run `keramos controller` in-cluster to reconcile KeramosRelease CRs.
- **`keramos controller`** is an in-cluster reconciler that watches KeramosRelease CRs and runs `keramos install` / `keramos upgrade` to converge.

→ [`keramos controller`](docs/cli/controller.md), [`keramos plan`](docs/cli/plan.md), [`keramos apply`](docs/cli/apply.md).

---

## The CLI

A few commonly-used commands; the full per-command reference (~110 pages, every flag) is in [`docs/cli/`](docs/cli/).

| Command | Purpose |
|---|---|
| `keramos create <name>` | Scaffold a new Kubernetes package. |
| `keramos init <template> <name>` | Scaffold from a built-in template (operator, batch, blank, webapp, ...). |
| `keramos lint <pkg>` | Validate the package — schema, templates, manifests. |
| `keramos template <pkg>` | Render templates locally; do not touch the cluster. |
| `keramos install <release> <pkg>` | Apply manifests as a new release. |
| `keramos upgrade <release> <pkg>` | New revision; apply diff. |
| `keramos rollback <release> <rev>` | Re-apply revision N; re-run its hooks. |
| `keramos diff <release> <pkg>` | Per-resource patch preview. |
| `keramos plan <release> <pkg>` | Render and persist an apply-able plan. |
| `keramos apply <plan>` | Execute a previously-saved plan. |
| `keramos list` | Releases across the cluster. |
| `keramos get <subres> <release>` | Manifest, values, hooks, notes, schema, etc. |
| `keramos history <release>` | All revisions, with audit data. |
| `keramos status <release>` | Current revision + resource readiness. |
| `keramos drift <release>` | Live-vs-stored diff. |
| `keramos reconcile <release>` | Re-apply stored manifest to converge cluster state. |
| `keramos rename <old> <new>` | Rename a release in-place (preserves history). |
| `keramos prune <release>` | Drop superseded revisions; keep most recent N. |
| `keramos values <pkg> [--trace]` | Show effective values with per-key resolution. |
| `keramos canary <release> <pkg>` | Staged upgrade through replica percentages with bake periods. |
| `keramos multi-install` | One invocation, many clusters. |
| `keramos workspace ...` | Multi-package, single repo. |
| `keramos releases ...` | Multi-release, possibly cross-source. |
| `keramos repo / registry / login / logout` | Distribution. |
| `keramos keyring / verify` | Signing trust. |
| `keramos policy` | Run package policies against rendered manifests. |
| `keramos scan` | Find common values across packages; extract a base layer. |
| `keramos sbom` | Emit a CycloneDX 1.5 SBOM for a release. |
| `keramos dev` | Watch a package and re-render on change (live loop). |
| `keramos controller` | Reconcile KeramosRelease CRs in-cluster. |
| `keramos plugin / marketplace` | Plugin discovery and installation. |
| `keramos purge` | Remove every keramos-installed release across the cluster. |

→ Per-command reference: [`docs/cli/`](docs/cli/).

---

## Keramos as a Helm alternative

Keramos is a modern, **open-source Helm alternative** for teams that want a more predictable templating model, native drift detection, audit trails, and a single binary that covers package management, distribution, and reconciliation. If you're searching for a **Helm replacement**, **Helm chart migration tool**, or simply a **modern alternative to Helm** for Kubernetes, the relevant comparison points are:

| Concern | Helm | **Keramos** |
|---|---|---|
| Templating | go-template + sprig (text-level) | YAML-native `${...}` expressions, control flow as YAML keys |
| Output parseability | go-template can emit invalid YAML | every render produces valid YAML by construction |
| Release storage | Secret per revision (`sh.helm.release.v1.*`) | Secret per revision (`keramos.v1.*`) — also `configmap`, `memory`, `sql` drivers |
| Drift detection | external `helm-diff` plugin | `keramos drift` built in, with smart filtering for noise |
| Reconcile to known good | not built in | `keramos reconcile` re-applies the stored manifest |
| Audit trail | sparse | every revision: who, when, where, what flags, what value files |
| Layer composition | sub-charts (umbrella charts) | `layers:` with first-class composition, parent overrides via `layers.<name>.<key>` |
| Multi-environment | values-`<env>`.yaml side files | first-class `environments:` block in `keramos.yaml` with inheritance |
| Workspace orchestration | not built in | `keramos-workspace.yaml` with topological install / parallelism / health-gating |
| Multi-cluster install | not built in | `keramos multi-install --to ctx-eu,ctx-us,ctx-ap` |
| Plan / apply separation | not built in | `keramos plan` + `keramos apply` with SHA-256 integrity check |
| Lifecycle hooks | yes | yes (per-revision persisted hooks for accurate rollback) |
| OCI distribution | yes | yes (same OCI distribution-spec) |
| PGP signing | yes (`.prov`) | yes (`.prov`) |
| Cosign signing | external | external (`cosign sign` + `keramos install --verify`) |
| GitOps integration | Argo CD, Flux helm-controller | Argo CD, Flux OCI source, or `keramos controller` (in-cluster reconciler) |

### Migrating from Helm to keramos

Keramos ships a **Helm chart converter** built in:

```sh
keramos migrate ./my-helm-chart -d ./my-keramos-package
```

The migrator walks the Helm chart structure (`Chart.yaml`, `templates/`, `values.yaml`, `crds/`, `_helpers.tpl`, `NOTES.txt`, `requirements.yaml`/`Chart.lock`) and emits an equivalent keramos package with go-template constructs translated to keramos's `${...}` expressions where possible. Constructs the migrator can't translate cleanly are listed in `keramos-migration.md` for human review.

For one-shot Helm-chart interop without committing to a migration, keramos also provides `keramos helm-compat` — useful when you want keramos's release management around an upstream Helm chart you're not ready to fork.

→ [Helm-to-keramos migration guide](docs/guides/migration.md), [`keramos migrate`](docs/cli/migrate.md), [`keramos helm-compat`](docs/cli/helm-compat.md), [Keramos vs Helm — full comparison](docs/comparison.md#keramos-as-a-helm-alternative).

---

## FAQ

### What is a Kubernetes package manager?

A Kubernetes package manager turns a directory of YAML manifests into a versioned, installable, distributable artifact. Operators install named "releases" of packages into clusters, upgrade them, roll them back, and audit who installed what. Keramos and Helm are both Kubernetes package managers; keramos adds first-class layering, drift detection, signing, OCI distribution, multi-cluster orchestration, and a Helm-chart migrator.

### Is keramos a Helm alternative?

Yes. Keramos is a **modern, open-source Helm alternative** designed for teams who want predictable templating, native drift detection, audit trails, and a single-binary install/upgrade/rollback/reconcile loop. Existing Helm charts can be migrated with `keramos migrate ./chart -d ./keramos-pkg`, and unmigrated charts can still be operated through the `keramos helm-compat` interop layer. See the [Helm-to-keramos comparison](docs/comparison.md#keramos-as-a-helm-alternative) for the side-by-side feature table.

### How do I switch from Helm to keramos?

Three paths, by how much you want to commit:

1. **Migrate the chart.** `keramos migrate ./my-chart -d ./my-pkg` translates `Chart.yaml`, templates, helpers, and dependencies into a keramos package. You then own the package long-term in keramos's vocabulary.
2. **Operate the Helm chart through keramos.** `keramos helm-compat install` runs upstream Helm charts under keramos's release record so you keep keramos's audit trail, drift detection, and reconcile model without converting the templates.
3. **Side-by-side migration.** Keep some releases in Helm, move others to keramos progressively. Keramos's `managedBy=keramos` label and `keramos.v1.*` Secret naming don't collide with Helm's `app.kubernetes.io/managed-by=Helm` and `sh.helm.release.v1.*` so the two coexist cleanly in one cluster.

→ Walkthrough: [Helm-to-keramos migration guide](docs/guides/migration.md).

### Is keramos a templating engine?

Yes. Keramos's `${...}` expression engine renders YAML manifests with values, computed expressions, control flow (`$if`, `$each`, `$switch`), and ~180 builtin functions. Output is always parseable YAML — there are no go-template tokens that produce malformed output.

### Does keramos do drift detection?

Yes. `keramos drift <release>` queries the live cluster state, compares each managed resource to the manifest keramos stored at install time, and reports per-field differences. `keramos reconcile <release>` re-applies the stored manifest to converge state.

### How does keramos compare to kustomize?

Kustomize is a manifest **patcher** — it applies overlays to base YAML to produce a final manifest. Keramos is a **package manager + templating engine + release tracker + drift detector** in one binary. Keramos packages can be installed as named releases, upgraded, rolled back, and audited; kustomize output is just a YAML stream the operator pipes into `kubectl apply`. The two compose: a keramos template can call `kustomize` as a post-renderer.

### How does keramos compare to kapp / kapp-controller?

`kapp` is a deployment tool with diff and waiting; `kapp-controller` is an in-cluster reconciler. Keramos's CLI covers the same install/diff/wait surface and more (templating, layering, signing, audit). Keramos's `keramos controller` is the equivalent of `kapp-controller` for reconciling in-cluster.

### How does keramos compare to kpt?

`kpt` is a function-based KRM tool focused on programmatic transformations. Keramos is a higher-level package manager with templating; the two are complementary if you want kpt-style functions, but keramos doesn't depend on kpt.

### How does keramos compare to carvel ytt / vendir / kapp?

Carvel is a suite. ytt does templating, vendir does dependencies, kapp does deployment. Keramos bundles equivalents of all three into one binary with consistent vocabulary (`packages`, `layers`, `releases`, `workspaces`).

### Does keramos work with Argo CD or Flux?

Yes. `keramos plan` produces deterministic rendered manifests Argo CD can sync. Flux can pull keramos packages from OCI registries via its OCI source. Keramos's `keramos controller` is the in-cluster equivalent for teams that want keramos to do reconciliation directly.

### Is keramos production-ready?

Keramos is an open-source MIT-licensed Go project with comprehensive tests (17 packages, race-detector clean), `govulncheck` clean, signed packages, drift detection, audit trails, and rollback. Real readiness depends on your evaluation criteria; this README and the `docs/` tree are the place to evaluate.

### Where do I report bugs / contribute?

Issues and pull requests at the GitHub repository. See the [docs](docs/) for architecture and contribution guidance.

---

## Documentation map

### Guides (start here)

- [Quickstart](docs/guides/quickstart.md) — from `keramos create` to rollback in 10 minutes
- [Package anatomy](docs/guides/packages.md) — every file in a keramos package
- [Values](docs/guides/values.md) — merging, layering, overrides, schema validation
- [Layers](docs/guides/layers.md) — composing a Kubernetes package from reusable building blocks
- [Hooks](docs/guides/hooks.md) — Job/Pod-based lifecycle hooks
- [Workspaces](docs/guides/workspaces.md) — multi-package orchestration with topological levels
- [Cross-release dependencies](docs/guides/releases.md) — `keramos-releases.yaml` for fleet-wide rollouts
- [Repositories](docs/guides/repositories.md) — HTTP repository hosting and consumption
- [OCI](docs/guides/oci.md) — pushing, pulling, and signing in OCI registries
- [Signing & verification](docs/guides/signing.md) — PGP, cosign, keyring
- [Schema validation](docs/guides/schema-validation.md) — `values.schema.json` patterns
- [Importing packages from external formats](docs/guides/migration.md) — `keramos migrate` walkthrough

### Reference (file formats)

- [`keramos.yaml`](docs/reference/keramos-yaml.md) — package manifest schema
- [`values.yaml`](docs/reference/values-yaml.md) — default configuration and merge semantics
- [`values.schema.json`](docs/reference/values-schema-json.md) — supported schema subset
- [`keramos-workspace.yaml`](docs/reference/keramos-workspace-yaml.md) — workspace manifest
- [`keramos-releases.yaml`](docs/reference/keramos-releases-yaml.md) — cross-release dependency manifest

### Templates

- [Expression syntax](docs/templates/expressions.md) — `${...}`, pipelines, dotted paths, literals
- [Control flow](docs/templates/control-flow.md) — `$if`, `$each`, `$switch`, partials
- [Function reference](docs/templates/functions.md) — every built-in, with input/output snippets
- [Layers in templates](docs/templates/layers.md) — how layer values merge into a flat map
- [Hooks in templates](docs/templates/hooks.md) — directives, weights, deletion policies
- [Capabilities API](docs/templates/capabilities.md) — `capabilities` and `lookup`

### CLI

- [CLI overview](docs/cli/README.md) — global flags, environment variables, exit codes, command index
- [Per-command reference](docs/cli/) — one Markdown file per subcommand with every flag

### Cross-cutting

- [FAQ](docs/faq.md) — common questions, expanded
- [Use cases](docs/use-cases.md) — for SREs, platform engineers, GitOps teams
- [Glossary](docs/glossary.md) — terminology reference
- [Troubleshooting](docs/troubleshooting.md) — common error messages and fixes
- [Comparison with other Kubernetes packaging tools](docs/comparison.md) — kustomize, kapp, kpt, ytt, ArgoCD, Flux

---

## Comparison with other Kubernetes packaging tools

| Tool | Templating | Releases | Drift detection | Signing | OCI distribution | Workspace orchestration |
|---|---|---|---|---|---|---|
| **Keramos** (Helm alternative) | YAML-native `${...}` | versioned, audited | yes (built in) | PGP + cosign | yes | yes |
| **Helm** | go-template + sprig | versioned (Secret) | external (`helm-diff`) | PGP + cosign | yes | no |
| **kustomize** | strategic merge patches | no (kubectl-apply only) | no | no | via flux | no |
| **kapp** | none (raw YAML) | yes (apps) | partial | no | partial | no |
| **kpt** | KRM functions | no | via diff | partial | yes (kpt source) | partial |
| **carvel ytt** | YAML overlays | via kapp | no | partial | via vendir | no |

→ Full comparison with examples: [Comparison guide](docs/comparison.md).

---

## License

Keramos is released under the [MIT License](LICENSE). Free for commercial and personal use.

---


## Keywords for searchability

`helm alternative` · `open source helm alternative` · `modern helm alternative` · `helm replacement` · `migrate from helm` · `helm chart converter` · `helm chart to keramos` · `helm to keramos migration` · `helm vs keramos` · `kubernetes package manager` · `k8s package manager` · `kubernetes` · `yaml templating kubernetes` · `kubernetes manifest tool` · `kubernetes deployment tool` · `kubernetes templating engine` · `kubernetes drift detection` · `kubernetes release management` · `kubernetes audit trail` · `kubernetes rollback` · `kubernetes app deployer` · `infrastructure as code kubernetes` · `gitops tool` · `argocd companion` · `flux companion` · `kustomize alternative` · `kapp alternative` · `kpt alternative` · `carvel alternative` · `ytt alternative` · `signed kubernetes packages` · `oci kubernetes packages` · `kubernetes package signing` · `pgp signed kubernetes` · `cosign kubernetes` · `multi cluster kubernetes deployment` · `kubernetes workspace` · `kubernetes lifecycle management` · `kubernetes promotion pipeline` · `kubernetes manifest renderer` · `kubernetes manifest validator` · `kubernetes lint` · `kubernetes sbom` · `kubernetes plan apply` · `kubernetes change management`
