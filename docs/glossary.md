# Keramos Glossary — Terminology Reference for Kubernetes Packaging, Templating, and Releases

A reference for every term keramos uses, alphabetised, with cross-links to the relevant guides. This is also the place where Google indexes "what is X in keramos" definitional searches.

## Apply

The action of sending a rendered manifest to the Kubernetes API. Keramos uses **server-side apply** with the field manager `keramos`, so two keramos invocations against the same release manage the same fields without spurious conflicts. `keramos apply <plan>` executes a previously-rendered plan; `keramos install` and `keramos upgrade` apply directly. → [Apply](cli/apply.md).

## Audit trail

The chronological record of every install, upgrade, rollback, and uninstall against a release. Keramos stores audit data in the release record: action, user, hostname, keramos binary version, kubeconfig context, CLI flags as passed, and value files supplied. `keramos audit <release>` prints it. → [Audit](cli/audit.md).

## Canary

A staged upgrade that ramps a release through a series of replica counts with a bake period at each step. `keramos canary <release> <pkg> --stages 1,3,5 --bake 5m` upgrades to 1 replica, waits 5 minutes for health, advances to 3, etc. Failure at any stage rolls back. → [Canary](cli/canary.md).

## Capabilities

The cluster-info namespace exposed to templates as `${capabilities.kubeVersion.Major}` etc. Used to gate version-specific Kubernetes fields. → [Capabilities](templates/capabilities.md).

## ConfigMap-backed storage

An alternative release-storage driver (`KERAMOS_DRIVER=configmap`) for clusters where Secret access is restricted by RBAC. Mirrors the Secret driver's semantics. → [Drivers](cli/README.md#environment-variables).

## Controller

`keramos controller` is the in-cluster reconciler that watches `KeramosRelease` CRs and runs `keramos install` / `keramos upgrade` to converge declared state. The Kubernetes-native deployment pattern for keramos packages, comparable to Flux's helm-controller for HelmRelease CRs. → [`keramos controller`](cli/controller.md).

## Cosign

Sigstore's container-signing tool. Keramos doesn't bundle cosign signing but works with the standard cosign workflow: `keramos registry push` to OCI, `cosign sign` the artifact, then `cosign verify` before `keramos install`. → [Signing guide](guides/signing.md).

## Cross-release dependencies

Releases that depend on other releases being installed first, declared in `keramos-releases.yaml`. Distinct from layers (which compose into one release) and workspace members (which are sibling releases from one repo). → [Cross-release dependencies guide](guides/releases.md).

## CRD (Custom Resource Definition)

Kubernetes' extensibility mechanism. Keramos packages can ship CRDs in `crds/` (applied first, with keramos waiting for `Established=true` before continuing) or in `templates/` (applied in normal install order, with CRDs sorted before all other kinds in the install graph).

## Drift

Divergence between the manifest keramos stored at install time and the current live cluster state. Drift can be caused by `kubectl edit`, other operators' reconcilers, manual API patches, or admission-webhook mutations. `keramos drift <release>` reports per-field differences. → [Drift](cli/drift.md).

## Dry run

A render that does not contact the cluster (`--dry-run client`) or that does contact but doesn't persist (`--dry-run server`, validates against admission webhooks). Available on install, upgrade, plan, apply, diff. → [Dry run flags](cli/install.md).

## Environment

A named deployment target (`dev`, `staging`, `prod`) declared inside `keramos.yaml` with its own value overrides, namespace, and kubeconfig context. Inheritance is supported (`prod` inherits from `staging` inherits from `dev`). → [Environments](guides/values.md#environments).

## Fingerprint

A PGP key's hex-encoded fingerprint. Used by `keramos keyring list` to identify keys uniquely.

## GitOps

A workflow where desired state lives in git and a reconciler in the cluster converges actual state to declared state. Keramos integrates with Argo CD and Flux as a packaging/templating tool that produces deterministic manifests they can sync; or via `keramos controller` as the reconciler itself. → [Use cases — GitOps](use-cases.md#for-gitops-teams-argo-cd-flux).

## Hook

A Job- or Pod-shaped resource that keramos runs at a specific lifecycle point (`pre-install`, `post-install`, `pre-upgrade`, `post-upgrade`, `pre-rollback`, `post-rollback`, `pre-delete`, `post-delete`, `test`). Each revision's hooks are persisted with the release so rollback re-runs the hooks that revision originally shipped. → [Hooks guide](guides/hooks.md).

## Keramos package

A directory with `keramos.yaml`, `values.yaml`, and `templates/` (and optional `crds/`, `hooks/`, `tests/`, `files/`, `notes.yaml`, `profiles/`, `policies/`, `README.md`, `LICENSE`, `keramos.lock`). The unit keramos packages, distributes, installs, and tracks. → [Package anatomy](guides/packages.md).

## keramos.lock

Auto-generated by `keramos dependency update`. Pins the resolved version, ref, and digest of every layer and required package. Commit it. Without it, two builds of the same package can pull different layer versions even with the same constraint.

## keramos.yaml

The package manifest at the root of every keramos package. Declares `name`, `version`, `apiVersion`, layers, requires, environments, immutables, and metadata. → [`keramos.yaml` reference](reference/keramos-yaml.md).

## keramos-releases.yaml

The manifest for cross-source release orchestration. Declares releases (each with a package source — local, OCI, HTTPS, git) plus optional `dependsOn` for ordering. Operated via `keramos releases plan/install/upgrade/uninstall/status`. → [`keramos-releases.yaml` reference](reference/keramos-releases-yaml.md).

## keramos-workspace.yaml

The manifest for multi-package orchestration in one repo. Declares members (sibling packages) plus `dependsOn`, `atomic`, `wait`, and per-member overrides. Operated via `keramos workspace plan/install/upgrade/uninstall/diff/status`. → [`keramos-workspace.yaml` reference](reference/keramos-workspace-yaml.md).

## Immutable values

Keys declared in `keramos.yaml`'s `immutable:` list that cannot change between revisions of a release once initially set. Enforced at `keramos upgrade` time before any cluster-side admission runs. → [`keramos.yaml`](reference/keramos-yaml.md).

## Install

The action of creating a new release in the cluster. `keramos install <release-name> <package-path>` renders, validates, applies, and stores. → [`keramos install`](cli/install.md).

## Layer

Another keramos package whose templates and values are composed into the current package, producing a single rendered manifest belonging to one release. Distinct from a workspace member (separate release). → [Layers](guides/layers.md).

## managedBy=keramos label

The canonical label keramos applies to every Kubernetes resource it creates and every namespace it provisions. The single source of truth for "did keramos do this?". One selector finds everything: `kubectl get all -A -l managedBy=keramos`.

## Manifest

The rendered YAML output of templating a keramos package — what keramos applies to the cluster. Stored gzip+base64-encoded inside the release record so rollback can re-apply exactly what was applied originally.

## OCI / OCI distribution

The container-image distribution spec, also used for non-image artifacts. Keramos packages push and pull as OCI artifacts with media type `application/vnd.keramos.package.v1.tar+gzip`. → [OCI guide](guides/oci.md).

## Partial

A named block in `_helpers.yaml` (or any `_*.yaml` file in `templates/`) that other templates can `${include}` into themselves. Keramos's reusability primitive within a package. → [Partials](guides/packages.md#partials-and-includes).

## Plan

A persisted, apply-able snapshot of a keramos operation: rendered manifest, parameters, and a SHA-256 integrity hash. `keramos plan` creates, `keramos apply` executes. The integrity hash detects drift between the plan and any subsequent re-render. → [`keramos plan`](cli/plan.md), [`keramos apply`](cli/apply.md).

## Plugin

An external command that integrates with keramos's CLI. Plugins live in `~/.config/keramos/plugins/`. Discovered via `keramos plugin list`, installed via `keramos plugin install <url>`. → [`keramos plugin`](cli/plugin.md).

## Policy

A rule that runs against a rendered manifest. Keramos packages can carry `policies/` (Rego or keramos-native YAML). `keramos policy run <pkg>` evaluates them. → [Policies](cli/policy.md).

## .prov file

A PGP cleartext-signed sidecar that travels alongside a `*.keramos.tgz` archive. Contains the package name, version, and SHA-256 digest, signed by the publisher's key. `keramos install --verify` fetches and validates it.

## Profile

A named values overlay file in `profiles/<name>.yaml`. Activated by `--profile <name>` on the CLI or `profile: <name>` on a workspace member or environment. Orthogonal to environments. → [Profiles](guides/values.md#profiles).

## Provenance

The data that proves where a release came from: the PGP signature on the source archive, the cosign signature on the OCI artifact, and the audit data keramos records on each install (user, time, source path, keramos version, kubeconfig context, CLI flags). → [Audit](cli/audit.md), [Signing guide](guides/signing.md).

## Reconcile

The action of re-applying a release's stored manifest to converge any drifted fields. `keramos reconcile <release>`. → [`keramos reconcile`](cli/reconcile.md).

## Release

A named instance of a keramos package installed in a cluster. Identified by `name + namespace`. Versioned across revisions: every install/upgrade increments a revision counter and stores a new release record. → [Release storage](glossary.md#secret-backed-storage).

## Render

The action of templating a package's `templates/` (and `crds/` and `hooks/` and `notes.yaml`) against merged values. Produces a rendered manifest. `keramos template <pkg>` does this offline.

## Required package

A separate release that the current package depends on. Declared in `keramos.yaml` under `requires:`. Distinct from `layers:` (which compose into the same release). The `requires:` list does not auto-install — operators install required packages first.

## Revision

A monotonically increasing integer per release. Each install/upgrade/rollback creates a new revision. The "current" revision has `status=deployed`; older revisions have `status=superseded`. `keramos history <release>` lists them all.

## Secret-backed storage

The default release-storage driver. Each revision is stored as a labelled Secret named `keramos.v1.<release>.v<revision>` in the install namespace. Switch via `KERAMOS_DRIVER=configmap|memory|sql`. → [Drivers](cli/README.md#environment-variables).

## Server-side apply

Kubernetes' patch mode where the API server tracks per-field ownership by `fieldManager`. Keramos uses `fieldManager=keramos`. Two keramos invocations against the same release manage the same fields without spurious conflicts; an external operator's edits show as "drift" because their fieldManager is different.

## Tag

In OCI distribution, a string label on an artifact (e.g. `1.2.3`, `latest`, `stable`). Keramos uses tags to identify package versions: `oci://host/path:1.2.3`. The version goes in the URI tag, not as a separate `--version` flag.

## Topological order

The dependency-respecting order in which workspace members or cross-release dependencies install. Computed from `dependsOn` declarations using Kahn's algorithm. Members within the same level have no inter-dependencies among themselves and are eligible to run in parallel.

## VersionSet

The data type behind `${capabilities.apiVersions}`. A map of GroupVersion strings the cluster has registered, populated from the API server's discovery (or from `--api-versions` in offline mode).

## Workspace

A `keramos-workspace.yaml` plus a tree of member packages from one repo. Operated as a single unit via `keramos workspace install/upgrade/uninstall/plan/diff/status`. Members are separate releases with a topological install order. → [Workspaces](guides/workspaces.md).

## Where next

- [FAQ](faq.md) — common questions
- [Use cases](use-cases.md) — for SREs, platform engineers, GitOps teams
- [Comparison with other Kubernetes packaging tools](comparison.md)
- [Documentation map](../README.md#documentation-map)
