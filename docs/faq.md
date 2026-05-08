# Keramos FAQ — Frequently Asked Questions about Kubernetes Packaging, Templating, Drift Detection, and Releases

This page answers the most common questions about keramos, organised by topic. If your question isn't here, check the [glossary](glossary.md), the [troubleshooting guide](troubleshooting.md), or the [docs index](../README.md#documentation-map).

## General

### What is keramos?

Keramos is an **open-source Kubernetes package manager, YAML templating engine, and modern Helm alternative**. It packages Kubernetes manifests into versioned artifacts, installs them as named releases, tracks revisions, runs drift detection, signs packages with PGP or cosign, and distributes them via OCI registries or HTTP repositories. One Go binary covers the install, upgrade, rollback, and reconcile loop end-to-end. Existing Helm charts can be converted via `keramos migrate`, or operated through the `keramos helm-compat` interop layer without conversion.

### Is keramos a Helm alternative?

Yes. Keramos is designed as a **modern, open-source alternative to Helm** for Kubernetes package management. Compared to Helm it adds YAML-native templating that's always valid YAML by construction, native drift detection, full audit trails per revision, multi-cluster atomic rollouts, plan/apply separation with integrity hashing, workspace orchestration, and a built-in Helm-chart converter (`keramos migrate`). Existing Helm charts can run unchanged through `keramos helm-compat install` to get keramos's release semantics around them. See [Keramos as a Helm alternative](comparison.md#keramos-as-a-helm-alternative) for the side-by-side feature table.

### How do I switch from Helm to keramos?

Three migration paths, by commitment level:

1. **Convert the chart.** `keramos migrate ./my-helm-chart -d ./my-keramos-package` translates `Chart.yaml`, `templates/`, `_helpers.tpl`, and dependencies into a keramos package. Go-template constructs become keramos `${...}` expressions where possible; the rest is listed in `keramos-migration.md` for review. → [Migration guide](guides/migration.md).
2. **Operate Helm charts unchanged.** `keramos helm-compat install my-app /path/to/upstream-chart -n prod` runs an upstream Helm chart under keramos's release record, giving you drift detection, reconcile, and audit trail without converting the templates. → [`keramos helm-compat`](cli/helm-compat.md).
3. **Side-by-side migration.** Keep some releases in Helm while moving others to keramos. The label and Secret-naming conventions don't collide (`managedBy=keramos` vs `app.kubernetes.io/managed-by=Helm`; `keramos.v1.*` vs `sh.helm.release.v1.*`), so a single cluster can host both at once during a phased migration.

### What problem does keramos solve?

Most teams writing Kubernetes manifests end up with one of three patterns: hand-edited YAML proliferation, kustomize overlays that grow into unmaintainable patch graphs, or shell-rendered templates piped into `kubectl apply`. Keramos replaces all three with a single coherent model: **packages** (versioned units), **releases** (instances of packages installed in a cluster), **layers** (composable building blocks), **environments** (declarative dev/staging/prod), and **workspaces** (orchestration across many packages).

### What does keramos NOT do?

Keramos is not a CI/CD platform, a service mesh, an admission controller, or a Kubernetes distribution. It is a packaging and release-management tool that integrates with whatever CI/CD, GitOps, or operator pattern you already use.

### Is keramos production-ready?

Keramos is MIT-licensed Go code with comprehensive test coverage (17 packages green under the race detector), `govulncheck` clean, signed packages with PGP and cosign verification, drift detection, audit trails, and rollback. Real production-readiness depends on your specific evaluation criteria; see the [docs](../README.md#documentation-map) for architecture and operational details.

### What's the license?

[MIT License](../LICENSE). Free for commercial and personal use.

## Templating

### Is keramos's templating language different from go-template?

Yes. Keramos uses `${...}` expressions inside YAML rather than `{{ ... }}` go-template tokens. Keramos's expression engine guarantees parseable YAML output — there are no template tokens that produce malformed YAML. Control flow lives at the YAML level (`$if`, `$each`, `$switch`) rather than as embedded text directives.

### Why YAML-native templating?

Three reasons. First, keramos's `${...}` always emits valid YAML, so a malformed template cannot silently break manifest parsing downstream. Second, control-flow directives are first-class YAML keys (`$if`, `$each`, `$switch`), so editors with YAML support already understand the structure. Third, the expression language is typed (string, number, bool, list, map, function call, pipeline, dotted lookup), giving useful errors instead of stringly-typed surprises.

### Does keramos support sprig functions?

Keramos ships ~180 builtin functions covering strings, math, regex, dates, crypto, collections, type conversion, encoding, and external integrations (HTTP, Vault, SOPS, ExternalSecrets). Many are equivalent in name and behaviour to Sprig functions. See the [function reference](templates/functions.md) for the full catalogue with input/output examples.

### Can keramos templates call HTTP, Vault, or SOPS at render time?

Yes, behind explicit opt-in env vars (`KERAMOS_RENDER_NETWORK=1` for HTTP/Vault). The render-time SSRF policy blocks loopback, link-local, RFC 1918 / CGNAT, and metadata-service IPs unless the operator explicitly allows internal addresses. SOPS uses the local `sops` binary; ExternalSecret and SealedSecret render manifests for the in-cluster operators. See the [external integrations docs](templates/functions.md#external-integrations).

### How do I share helpers between templates?

Put named blocks in `_helpers.yaml` (or any `_*.yaml` file under `templates/`). Reference them with `$include: name` in YAML or `${include "name"}` in expressions. See the [partials documentation](guides/packages.md#partials-and-includes).

## Releases and revisions

### How does keramos track releases?

Each release is stored as a labelled Kubernetes Secret named `keramos.v1.<release>.v<revision>` in the install namespace, with `managedBy=keramos` (and the legacy `owner=keramos` for backwards compat). The Secret contains a gzip+base64 encoded JSON document with the rendered manifest, merged values, audit data, and per-revision hooks.

### How does rollback work?

`keramos rollback <release> <revision>` looks up the named revision's stored manifest, re-applies it via server-side apply, and re-runs that revision's `pre-rollback` and `post-rollback` hooks. The hook templates are persisted alongside the manifest, so a rollback to revision N runs the hooks revision N originally shipped — not the current ones.

### Does keramos keep the entire history forever?

By default yes. `keramos prune <release> --keep N` drops superseded revisions, keeping the most recent N. The current `deployed` revision is never dropped regardless of `--keep`.

### Can I rename a release in place?

Yes: `keramos rename <old> <new>` copies every revision Secret under the new name and labels, then deletes the originals. The cluster resources the release manages are not renamed; templates that derive resource names from `${release.name}` will reflect the new name on the next upgrade.

## Drift detection and reconcile

### What is drift detection?

Drift is the gap between what keramos stored as the rendered manifest and what's actually in the cluster right now. Drift can come from `kubectl edit`, another operator's reconciler, manual API patches, or admission-webhook mutations. `keramos drift <release>` queries each managed resource and reports the per-field differences.

### How do I converge cluster state back to the stored manifest?

`keramos reconcile <release>` re-applies the stored manifest to the cluster, taking ownership of any drifted fields. This is the inverse of drift detection.

### Does keramos report drift on every field?

By default yes, with smart filtering for noise (server-side defaulters, managed-fields metadata, and rotated Secret data are hidden unless you pass `--show-defaulted-fields`, `--show-managed-fields`, `--show-secret-rotation`).

## Signing and supply chain

### How does keramos sign packages?

Two mechanisms. **PGP**: `keramos package --sign --key <id> --keyring <path>` produces a `.prov` cleartext-signed sidecar with the package metadata and SHA-256 digest. Consumers verify with `keramos install --verify` or `keramos pull --verify` against a local PGP keyring. **Cosign**: keramos doesn't sign with cosign directly, but works with the standard cosign workflow — push the package as an OCI artifact, then `cosign sign` it. Verification is the operator's pre-install step.

### Where is the local keyring stored?

`~/.config/keramos/keyring/` (or `${KERAMOS_CONFIG_HOME}/keyring/`). It's a directory of armoured PGP public keys, one per signer. Manage with `keramos keyring add/list/remove`.

### Are credentials persisted?

OCI and HTTP-repo credentials live in `~/.config/keramos/credentials.json` keyed by host. Keramos also reads `~/.docker/config.json` as a fallback so existing `docker login` / `aws ecr get-login-password` flows work without re-login.

## OCI distribution

### Which OCI registries does keramos work with?

Anything that implements the OCI distribution spec: GitHub Container Registry (GHCR), Docker Hub, Quay, Harbor, Artifactory, AWS ECR, Google Artifact Registry, Azure Container Registry, self-hosted Distribution, self-hosted Zot. Auth uses the registry's standard pattern; cloud providers' credential helpers work transparently.

### How do I push a package to OCI?

```sh
keramos package ./my-app -d ./build
keramos registry push ./build/my-app-1.2.3.keramos.tgz oci://ghcr.io/my-org/charts/my-app
```

→ Full guide: [OCI](guides/oci.md).

### How do I install from OCI?

```sh
keramos install my-app oci://ghcr.io/my-org/charts/my-app:1.2.3 -n prod
```

The version goes in the URI tag, not as a `--version` flag.

## Workspaces and multi-package

### What's the difference between layers and workspaces?

**Layers** compose multiple packages into **one rendered manifest** belonging to **one release**. Use when the pieces are not separately useful (always shipped together).

**Workspaces** orchestrate multiple **separate releases** declared in `keramos-workspace.yaml` with `dependsOn` for ordering. Use when each package is independently upgradeable.

→ Layer guide: [Layers](guides/layers.md). Workspace guide: [Workspaces](guides/workspaces.md).

### What's the difference between `keramos workspace` and `keramos releases`?

`keramos workspace` orchestrates packages from sibling directories of one repo. `keramos releases` orchestrates releases from anywhere — local paths, OCI, HTTPS, git — declared in `keramos-releases.yaml`. Both honour `dependsOn` and topological install order; `keramos workspace` adds `--parallel` and `--health-gate` for finer-grained orchestration within a level.

→ [`keramos-workspace.yaml`](reference/keramos-workspace-yaml.md) vs [`keramos-releases.yaml`](reference/keramos-releases-yaml.md).

### Can keramos deploy to multiple clusters in one command?

Yes: `keramos multi-install <release> <package> --to ctx-eu,ctx-us,ctx-ap`. Use `--atomic-cross-cluster` to roll back every cluster if any fails.

## GitOps integration

### Does keramos replace Argo CD or Flux?

No, it complements them. Argo CD and Flux are GitOps reconcilers; keramos is a packaging and templating tool that produces deterministic manifests they can sync. The two compose:

- **Argo CD users** can git-commit keramos packages and have Argo CD render them via a custom plugin, OR pre-render with `keramos plan` and commit the rendered output for Argo to sync.
- **Flux users** can pull keramos packages from OCI via Flux's OCI source.
- For teams that want keramos to handle reconciliation directly, `keramos controller` is an in-cluster reconciler for KeramosRelease CRs.

→ [Controller docs](cli/controller.md), [Plan/apply](cli/plan.md).

## Migration

### How do I migrate an existing Helm chart to keramos?

`keramos migrate ./upstream-chart -d ./migrated/`. The migrator translates `Chart.yaml` to `keramos.yaml`, rewrites go-template constructs to keramos `${...}` expressions where it can, and produces a `keramos-migration.md` review report listing what needs manual attention. → [Migration guide](guides/migration.md).

### Can keramos render Helm charts directly without migration?

Yes, with the [`helm-compat`](cli/helm-compat.md) command for one-shot interop. For long-term ownership, migrate.

## Operations

### How do I find every resource keramos manages cluster-wide?

Single label selector:

```sh
kubectl get all -A -l managedBy=keramos
```

Keramos stamps `managedBy=keramos` on every applied resource and on namespaces it creates.

### How do I clean up everything keramos installed?

`keramos purge --yes --force --delete-namespaces`. With `--force` keramos also force-finalises stuck `Terminating` namespaces (use after a node failure or test sprawl). → [`keramos purge`](cli/purge.md).

### Can I see who installed what when?

`keramos audit <release>` prints the chronological audit trail: every revision, the action, who initiated, the kubeconfig context, keramos version, flags as passed, value files supplied. → [`keramos audit`](cli/audit.md).

## Performance and scale

### How fast is keramos's templating engine?

The expression engine is interpreted, not compiled, but the per-render cost is dominated by YAML parsing and disk I/O. Renders of typical packages (50-200 manifests) complete in tens of milliseconds. Parallel renders are race-detector clean.

### How big can a release be?

The default Secret-backed storage caps each revision at 1 MiB encoded (the Kubernetes Secret limit). For larger payloads use the SQL or ConfigMap-backed driver. → [`KERAMOS_DRIVER`](cli/README.md#environment-variables).

### Does keramos scale to many clusters?

`keramos multi-install --to <list>` parallelises by kubeconfig context. The `keramos controller` reconciler is event-driven and scales to thousands of KeramosRelease CRs.

## Compatibility and ecosystem

### What Kubernetes versions does keramos support?

Kubernetes 1.25+ — recent enough for server-side apply with field manager semantics. Specific cluster versions are reported via `${capabilities.kubeVersion.Version}` to templates.

### Does keramos work with OpenShift?

Yes — keramos uses the standard Kubernetes API. OpenShift's restricted SCCs may require pod-spec adjustments in your packages but keramos itself doesn't impose constraints.

### Does keramos work in air-gapped environments?

Yes. Pull packages once with `keramos pull`, mirror to your private OCI registry or HTTP server, and install offline. Render-time network calls (HTTP/Vault) are opt-in, off by default.

### What about cloud-specific deployments (AWS EKS, GCP GKE, Azure AKS)?

Keramos is cloud-agnostic. Cloud-specific concerns (IAM, IRSA on EKS, Workload Identity on GKE, Pod Identity on AKS) are expressed in your package's templates. Standard service-account / RBAC patterns work.

## Troubleshooting

For specific error messages, see the [troubleshooting guide](troubleshooting.md).

## Where next

- [Quickstart](guides/quickstart.md) — first install
- [Glossary](glossary.md) — terminology
- [Use cases](use-cases.md) — for platform engineers, SREs, GitOps teams
- [Comparison with other Kubernetes packaging tools](comparison.md)
- [CLI reference](cli/)
- [Documentation map](../README.md#documentation-map)
