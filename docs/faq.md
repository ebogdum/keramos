# Keramos FAQ — frequently asked questions

Answers to common questions about keramos, organised by topic. If your question
isn't here, check the [glossary](glossary.md), the
[troubleshooting guide](troubleshooting.md), or the
[docs index](../README.md#documentation-map).

## General

### What is keramos?

Keramos is an **open-source Kubernetes package manager, YAML templating engine,
and Helm alternative**. It packages Kubernetes manifests into versioned
artifacts, installs them as named releases, tracks revisions, detects drift,
signs packages with PGP, and distributes them via OCI registries or HTTP
repositories. One Go binary covers the install, upgrade, rollback, and
reconcile loop. Existing Helm charts convert via `keramos migrate`, or run through
the `keramos helm-compat` interop layer without conversion.

### Is keramos a Helm alternative?

Yes. Compared to Helm, keramos adds YAML-native templating that stays valid YAML
by construction, built-in drift detection, per-revision audit trails,
multi-cluster atomic rollouts, plan/apply separation with integrity hashing,
workspace orchestration, and a built-in Helm-chart converter. Existing charts
run through `keramos helm-compat install` to get keramos's release semantics around
them. See [Keramos as a Helm alternative](comparison.md#keramos-as-a-helm-alternative)
for the feature table.

### How do I switch from Helm to keramos?

Three paths, by commitment level:

1. **Convert the chart.** `keramos migrate ./my-helm-chart -o ./my-keramos-package`
   translates `Chart.yaml`, `templates/`, `_helpers.tpl`, and dependencies.
   Go-template constructs become keramos `${...}` expressions where possible; the
   rest is listed in the migrator's conversion report for review.
   → [Migration guide](guides/migration.md).
2. **Operate charts unchanged.** `keramos helm-compat install my-app
   /path/to/upstream-chart -n prod` runs an upstream Helm chart under a keramos
   release record, giving you reconcile, rollback, and audit without converting
   the templates. → [`keramos helm-compat`](cli/helm-compat.md).
3. **Side-by-side.** The label and Secret-naming conventions don't collide
   (`managedBy=keramos` vs `app.kubernetes.io/managed-by=Helm`; `keramos.v1.*` vs
   `sh.helm.release.v1.*`), so one cluster can host both during a phased
   migration.

### What problem does keramos solve?

Most teams writing Kubernetes manifests land on one of three patterns:
hand-edited YAML proliferation, Kustomize overlays that grow into
unmaintainable patch graphs, or shell-rendered templates piped into `kubectl
apply`. Keramos replaces all three with one model: **packages** (versioned units),
**releases** (installed instances), **layers** (composable building blocks),
**environments** (declarative dev/staging/prod), and **workspaces**
(orchestration across many packages).

### What does keramos NOT do?

Keramos is not a CI/CD platform, a service mesh, an admission controller, or a
Kubernetes distribution. It's a packaging and release-management tool that
integrates with whatever CI/CD, GitOps, or operator pattern you already use.

### What's the license?

[MIT License](../LICENSE). Free for commercial and personal use.

## Templating

### Is keramos's templating language different from go-template?

Yes. Keramos uses `${...}` expressions inside YAML rather than `{{ ... }}`
go-template tokens. The expression engine guarantees parseable YAML output —
there are no template tokens that produce malformed YAML. Control flow lives at
the YAML level (`$if`, `$each`, `$switch`) rather than as embedded text.

### Why YAML-native templating?

Three reasons. First, `${...}` always emits valid YAML, so a malformed template
can't silently break downstream parsing. Second, control-flow directives are
first-class YAML keys, so YAML-aware editors understand the structure. Third,
the expression language is typed (string, number, bool, list, map, function
call, pipeline, dotted lookup), giving useful errors instead of stringly-typed
surprises.

### Does keramos support sprig functions?

Keramos ships a large builtin library (~200 built-in functions) covering
strings, math, collections, dates, crypto, regex, encoding, and external
integrations (HTTP, Vault, SOPS, ExternalSecrets). Many match Sprig functions
in name and behaviour. See the
[function reference](templates/functions.md) for the full catalogue with
input/output examples.

### Does keramos have `eq` / `and` / `or` for comparisons?

No — `eq`, `ne`, `lt`, `gt`, `and`, `or`, and `not` are not functions in keramos's
engine. Use `$if` truthy evaluation or `$switch` for branching. Note that
`coalesce`, `default`, and `ternary` **do** exist. See
[Troubleshooting — `unknown function "eq"`](troubleshooting.md#unknown-function-eq).

### Can keramos templates call HTTP, Vault, or SOPS at render time?

Yes, behind explicit opt-in (`KERAMOS_RENDER_NETWORK=1` for HTTP/Vault). The
render-time SSRF policy blocks loopback, link-local, RFC 1918 / CGNAT, and
metadata-service addresses unless the operator explicitly allows internal
targets. SOPS uses the local `sops` binary; `externalSecret` and `sealedSecret`
render manifests for the in-cluster operators. See the
[function reference](templates/functions.md).

### How do I share helpers between templates?

Put named blocks in `_helpers.yaml` (or any `_*.yaml` file under `templates/`).
Reference them with `$include: name` in YAML or `${include "name"}` in
expressions. See [partials](guides/packages.md#partials-and-includes).

## Releases and revisions

### How does keramos track releases?

Each release revision is stored as a labelled Kubernetes Secret named
`keramos.v1.<release>.v<revision>` in the install namespace, carrying
`managedBy=keramos` (plus the legacy `owner=keramos`). The Secret holds a
gzip+base64-encoded JSON document with the rendered manifest, merged values,
audit data, and per-revision hooks.

### How does rollback work?

`keramos rollback <release> <revision>` looks up that revision's stored manifest,
re-applies it via server-side apply, and re-runs that revision's `pre-rollback`
and `post-rollback` hooks. The hook templates are persisted with the release,
so a rollback to revision N runs the hooks revision N originally shipped.

### Does keramos keep the entire history forever?

By default yes — `--history-max` is unlimited (`0`) on install and upgrade.
`keramos prune --keep N` drops superseded revisions, keeping the most recent N per
release (default 10); add `--release <name>` to target one release. The current
`deployed` revision is never dropped.

### Can I rename a release in place?

Yes: `keramos rename <old> <new>` copies every revision Secret under the new name
and labels, then deletes the originals. The cluster resources the release
manages aren't renamed; templates that derive names from `${release.name}`
reflect the new name on the next upgrade.

## Drift detection and reconcile

### What is drift detection?

Drift is the gap between what keramos recorded and what's live in the cluster.
It comes from `kubectl edit`, another operator's reconciler, manual API
patches, or webhook mutations. `keramos drift ./my-app` renders the package and
compares three views — the package as it renders now, the recorded state, and
the running cluster — flagging both **cluster drift** (recorded ≠ running) and
**pending apply** (package ≠ recorded). The release name is derived from
`keramos.yaml`; override with `-r`.

### How do I converge cluster state back to the stored manifest?

`keramos reconcile <release>` re-applies the stored manifest, taking ownership of
drifted fields. It's addressed by release name, so it works even without the
package source present.

### Does keramos report drift on every field?

Comparison is limited to keramos-managed fields, so cluster-injected noise
(status, `managedFields`, server-side defaults) is ignored. Pass `--server-side`
to compare against a server-side apply dry-run that reflects admission and
defaulting.

## Signing and supply chain

### How does keramos sign packages?

**PGP**: `keramos package ./my-app --sign --key <signer>` (or `package sign` on an
existing archive) produces a `.prov` cleartext-signed sidecar with the package
metadata and SHA-256 digest. Consumers verify with `keramos install --verify` or
`keramos pull --verify` against a local PGP keyring. **Cosign**: keramos doesn't sign
with cosign directly, but works with the standard workflow — push the package
as an OCI artifact, `cosign sign` it, then `cosign verify` before install.

### Where is the local keyring stored?

`~/.config/keramos/keyring/` (or `${KERAMOS_CONFIG_HOME}/keyring/`) — a directory of
armoured PGP public keys, one per signer. Manage it with
`keramos keyring add/list/remove`.

### Are credentials persisted?

OCI and HTTP-repo credentials are stored by host after `keramos login <host>`.
Keramos also reads `~/.docker/config.json` as a fallback, so existing
`docker login` / `aws ecr get-login-password` flows work without re-login.

## OCI distribution

### Which OCI registries does keramos work with?

Anything implementing the OCI distribution spec: GHCR, Docker Hub, Quay,
Harbor, Artifactory, AWS ECR, Google Artifact Registry, Azure Container
Registry, self-hosted Distribution, self-hosted Zot. Cloud credential helpers
work transparently.

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

The version goes in the URI tag, not a `--version` flag.

## Workspaces and multi-package

### What's the difference between layers and workspaces?

**Layers** compose multiple packages into **one rendered manifest** belonging
to **one release**. Use them when the pieces always ship together.

**Workspaces** orchestrate multiple **separate releases** declared in
`keramos-workspace.yaml` with `dependsOn` for ordering. Use them when each package
is independently upgradeable.

→ [Layers](guides/layers.md), [Workspaces](guides/workspaces.md).

### What's the difference between `keramos workspace` and `keramos releases`?

`keramos workspace` orchestrates member packages from one repo tree. `keramos
releases` orchestrates releases from anywhere — local paths, OCI, HTTPS, git —
declared in `keramos-releases.yaml`. Both honour `dependsOn` and topological
order; `keramos workspace` adds `--parallel`, `--health-gate`, and
`--atomic-workspace` for finer control within a level (and a `diff` subcommand
that `keramos releases` doesn't have).

→ [`keramos-workspace.yaml`](reference/keramos-workspace-yaml.md) vs
[`keramos-releases.yaml`](reference/keramos-releases-yaml.md).

### Can keramos deploy to multiple clusters in one command?

Yes: `keramos multi-install <release> <package> --to ctx-eu,ctx-us,ctx-ap`. Add
`--atomic-cross-cluster` to roll back every cluster if any fails.

## GitOps integration

### Does keramos replace Argo CD or Flux?

No, it complements them. They're GitOps reconcilers; keramos produces
deterministic manifests they can sync:

- **Argo CD** users can pre-render with `keramos template` and commit the YAML for
  Argo to sync, or register `keramos template` as a Config Management Plugin.
- **Flux** users can pull keramos packages from OCI via Flux's OCI source.
- For teams that want keramos to reconcile directly, `keramos controller` is an
  in-cluster reconciler for KeramosRelease CRs.

→ [Controller](cli/controller.md), [Plan/apply](cli/plan.md).

## Migration

### How do I migrate an existing Helm chart to keramos?

`keramos migrate ./upstream-chart -o ./migrated/`. The migrator translates
`Chart.yaml` to `keramos.yaml`, rewrites go-template constructs to keramos `${...}`
expressions where it can, and prints a conversion report listing what needs
manual attention. → [Migration guide](guides/migration.md).

### Can keramos render Helm charts without migration?

Yes — `keramos helm-compat render` renders an unmodified chart (like
`helm template`), and `keramos helm-compat install` applies it under a keramos
release record. For long-term ownership, migrate.
→ [`keramos helm-compat`](cli/helm-compat.md).

## Operations

### How do I find every resource keramos manages cluster-wide?

```sh
kubectl get all -A -l managedBy=keramos
```

Keramos stamps `managedBy=keramos` on every applied resource and on namespaces it
creates. For keramos's own view, `keramos list -A`.

### How do I clean up everything keramos installed?

`keramos purge` is dry-run by default; add `--yes` to run. `--force` skips
graceful uninstall and force-finalises stuck `Terminating` namespaces;
`--delete-namespaces` and `--delete-crds` remove those too.
→ [`keramos purge`](cli/purge.md).

### Can I see who installed what when?

`keramos audit <release>` prints the chronological trail: every revision, the
action, who initiated it, the kubeconfig context, the keramos version, the flags,
and the value files. → [`keramos audit`](cli/audit.md).

## Performance and scale

### How fast is keramos's templating engine?

The expression engine is interpreted; per-render cost is dominated by YAML
parsing and disk I/O. Typical packages (50–200 manifests) render in tens of
milliseconds, and parallel renders are race-detector clean.

### How big can a release be?

The default Secret-backed storage caps each revision at 1 MiB encoded (the
Kubernetes Secret limit; the ConfigMap driver shares it). For larger payloads
use the SQL driver (`KERAMOS_DRIVER=sql`).
→ [Environment variables](cli/README.md#environment-variables).

### Does keramos scale to many clusters?

`keramos multi-install --to <list>` parallelises across kubeconfig contexts
(`--parallel`). The `keramos controller` reconciler is event-driven.

## Compatibility and ecosystem

### What Kubernetes versions does keramos support?

Recent versions with server-side apply and field-manager semantics. Cluster
versions are reported to templates via `${capabilities.kubeVersion.Version}`.

### Does keramos work with OpenShift?

Yes — keramos uses the standard Kubernetes API. OpenShift's restricted SCCs may
require pod-spec adjustments in your packages, but keramos imposes no constraints
of its own.

### Does keramos work in air-gapped environments?

Yes. Pull packages once with `keramos pull`, mirror them to a private OCI registry
or HTTP server, and install offline. Render-time network calls are opt-in and
off by default.

### What about AWS EKS, GCP GKE, Azure AKS?

Keramos is cloud-agnostic. Cloud-specific concerns (IRSA on EKS, Workload Identity
on GKE, Pod Identity on AKS) live in your package's templates via standard
service-account / RBAC patterns.

## Troubleshooting

For specific error messages, see the
[troubleshooting guide](troubleshooting.md).

## Where next

- [Quickstart](guides/quickstart.md) — first install
- [Glossary](glossary.md) — terminology
- [Use cases](use-cases.md) — by role
- [Comparison with other tools](comparison.md)
- [CLI reference](cli/README.md)
- [Documentation map](../README.md#documentation-map)
