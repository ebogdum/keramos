# Keramos vs Helm vs Kustomize vs kapp vs kpt — Kubernetes packaging and deployment tools compared

A side-by-side look at keramos and the other tools in the Kubernetes
packaging, templating, and deployment space. If you're evaluating a **Helm
alternative** or weighing your packaging options, this page gives you the
detail to choose.

## TL;DR

Keramos is a **Kubernetes package manager, templating engine, and Helm
alternative** that bundles versioned releases, drift detection, signing, OCI
distribution, and workspace orchestration into one binary. The closest
comparisons:

| Tool | Categorically similar to keramos | Where they differ |
|---|---|---|
| **Helm** | full Kubernetes package manager | keramos adds drift detection, audit trails, multi-cluster, plan/apply, and YAML-native templating. |
| **Kustomize** | YAML transformation | Kustomize patches; keramos templates and tracks releases. |
| **kapp / kapp-controller** | release management | keramos also templates and signs. |
| **kpt** | KRM-based YAML mutation | kpt is function-pipeline-driven; keramos is template-driven. |
| **Carvel ytt + vendir + kapp** | full packaging suite | keramos is one binary instead of three. |
| **Argo CD / Flux** | GitOps reconcilers | keramos is a packaging tool; the two compose. |

## Feature matrix

| Feature | **Keramos** | Helm | Kustomize | kapp | kpt | ytt | Argo CD | Flux |
|---|---|---|---|---|---|---|---|---|
| YAML templating | yes (`${...}`) | yes (go-template + sprig) | overlays only | no | KRM functions | yes (overlays) | uses other tools | uses other tools |
| Output is always valid YAML | yes (by construction) | no (text-level templating) | yes | yes | yes | yes | n/a | n/a |
| Versioned releases | yes | yes | no | yes (apps) | no | no | yes (Application) | yes (HelmRelease/Kustomization) |
| Revision history | yes | yes | no | partial | no | no | yes | yes |
| Drift detection | yes (built in) | external (`helm-diff`) | no | partial | yes | no | yes | yes |
| Reconcile to stored state | yes | no | no | yes | partial | no | yes | yes |
| Atomic install/rollback | yes | yes | no | yes | no | no | yes | yes |
| Lifecycle hooks | yes (per-revision persisted) | yes | no | yes | no | no | yes (sync waves) | partial |
| Layered package composition | yes (`layers:`) | sub-charts (umbrella) | overlays | no | partial | overlays | uses other tools | uses other tools |
| Multi-environment values | yes (`environments:`) | side files (`values-<env>.yaml`) | overlays | no | setters | data values | uses other tools | uses other tools |
| Workspace orchestration | yes (`keramos-workspace.yaml`) | no | no | no | no | no | ApplicationSet | uses Kustomization |
| Multi-cluster deploy | yes (`keramos multi-install`) | no | no | no | no | no | yes (per Application) | yes |
| OCI distribution | yes | yes | partial | partial | yes | no | yes | yes |
| HTTP repository | yes (`keramos repo`) | yes (`helm repo`) | no | no | no | no | yes (helm) | yes (helm) |
| PGP signing | yes (`.prov`) | yes (`.prov`) | no | no | no | no | uses cosign | uses cosign |
| Cosign signing | external | external | no | no | no | no | yes | yes |
| Audit trail per release | yes (who/when/where/flags) | sparse | no | no | no | no | partial (sync history) | partial |
| SBOM emission | yes (CycloneDX 1.5) | no | no | no | no | no | no | no |
| Plan/apply separation | yes | no | no | no | partial | no | yes (sync wave) | partial |
| In-cluster reconciler | yes (`keramos controller`) | external (Flux helm-controller) | no | yes (kapp-controller) | no | no | yes | yes |
| Convert Helm charts | yes (`keramos migrate`) | n/a | no | no | no | no | no | no |

## Keramos as a Helm alternative

Keramos is the closest categorical peer to **Helm** — both turn a directory of
templated YAML into a versioned, installable, distributable artifact. If
you're evaluating a **Helm alternative** or planning a **Helm-to-keramos
migration**, this section is the side-by-side detail.

### Why pick keramos over Helm

- **YAML-native templating that stays valid YAML by construction.** Helm's
  go-template + sprig is a text-level engine that can emit malformed YAML
  (whitespace bugs, half-quoted strings). Keramos's `${...}` substitutions and
  `$if` / `$each` / `$switch` directives are themselves YAML, so the output
  can't be syntactically broken.
- **Drift detection is built in.** `keramos drift ./my-app` renders the package
  and reports, per field, where the recorded state and the live cluster
  diverge. Helm needs the third-party `helm-diff` plugin for a narrower view.
  `keramos reconcile <release>` then re-applies the stored manifest to converge.
- **Audit trail per revision.** Every `keramos install` / `keramos upgrade` /
  `keramos rollback` records who ran it, the kubeconfig context, the keramos
  version, the CLI flags, and the value files supplied. `keramos audit <release>`
  answers "who upgraded the auth service yesterday?" months later.
- **Multi-cluster deploys in one command.** `keramos multi-install <release>
  <pkg> --to ctx-eu,ctx-us,ctx-ap --atomic-cross-cluster` ships one release to
  N clusters. Helm needs external orchestration.
- **Plan / apply separation.** `keramos plan -o app.plan` writes an apply-able
  JSON artifact (rendered manifest plus a SHA-256 integrity digest);
  `keramos apply --plan app.plan` re-renders and verifies the digest before
  applying. Helm has no native plan/apply split.
- **Workspace orchestration.** `keramos-workspace.yaml` installs many releases in
  topological order with per-level parallelism (`--parallel`), health gating
  (`--health-gate`), and atomic rollback (`--atomic-workspace`).
- **First-class environments.** An `environments:` block in `keramos.yaml` with
  `inherits:` chaining replaces Helm's `values-<env>.yaml` side-file sprawl.
- **Layer composition with explicit overrides.** Keramos's `layers:` merges other
  packages into one release, overridable per key from the parent's values.
- **Per-revision hooks for accurate rollback.** A rollback to revision N
  re-runs the hooks revision N originally shipped — persisted with the
  release, not re-derived from the current package.
- **One binary** covers package, sign, push, install, upgrade, drift,
  reconcile, audit, SBOM, lint, plan, apply, and an in-cluster reconciler.
  Helm pushes drift / SBOM / multi-cluster / reconciliation onto a
  constellation of plugins and adjacent tools.

### What Helm still does better today

- **Ecosystem maturity.** Artifact Hub, the chart-museum convention, and the
  published prometheus / grafana / cert-manager / etc. charts. Keramos packages
  are new; you'll write or migrate most of them.
- **Operator familiarity.** Most Kubernetes operators have written Helm charts
  for years. Keramos's vocabulary is close but not identical.

### Migration path: Helm to keramos

Three options, by how much commitment they need:

1. **Convert the chart with `keramos migrate`.** The migrator walks the Helm
   chart (`Chart.yaml`, `templates/`, `values.yaml`, `crds/`, `_helpers.tpl`,
   `NOTES.txt`, dependencies) and emits an equivalent keramos package. Go-template
   constructs become keramos `${...}` expressions where the migrator can do it
   cleanly; anything it can't translate is printed as a conversion report of
   items requiring manual review.

   ```sh
   keramos migrate ./my-helm-chart -o ./my-keramos-package
   keramos lint ./my-keramos-package
   ```

   The migrator lists unsupported constructs (file, line, and reason) in its
   output. → Walkthrough: [Helm-to-keramos migration guide](guides/migration.md).

2. **Operate the upstream chart through `keramos helm-compat`.** If you don't
   want to fork, the compat layer runs a Helm-shaped chart under a keramos
   release record, so you get keramos's audit trail, reconcile, and rollback
   around a chart you didn't author.

   ```sh
   keramos helm-compat install my-app /path/to/upstream-chart -n prod
   keramos audit my-app -n prod        # works — release-name commands
   keramos reconcile my-app -n prod    # re-applies the stored manifest
   keramos rollback my-app 1 -n prod   # works
   ```

   → [`keramos helm-compat`](cli/helm-compat.md).

3. **Side-by-side, gradual migration.** Keep critical legacy charts in Helm
   while moving new packages to keramos. Keramos's `managedBy=keramos` label and
   `keramos.v1.<release>.v<rev>` Secret naming don't collide with Helm's
   `app.kubernetes.io/managed-by=Helm` and `sh.helm.release.v1.*`, so the two
   coexist in one cluster without interference.

### When to stay on Helm

If your primary value is the Artifact Hub catalogue (postgresql, redis,
ingress-nginx, kube-prometheus-stack, cert-manager) and your charts work
without templating-engine quirks, the migration cost may exceed the benefit.
`keramos migrate` lowers that cost but doesn't eliminate it. The question is how
often you hit Helm template edge cases, drift-detection gaps, multi-cluster
friction, or audit-trail blindness.

### When keramos is clearly the better choice

- You're building a new platform and don't want to inherit Helm's templating
  tax.
- You operate at multi-cluster scale and need atomic cross-cluster rollouts.
- You need drift detection and reconcile natively, not as a plugin.
- You need signing + audit + SBOM as a deployment-tool requirement.
- You want one binary instead of stacking helm + helm-diff + helm-secrets +
  a reconciler.

---

## Keramos vs Kustomize

[**Kustomize**](https://kustomize.io/) is the built-in Kubernetes manifest
transformer. It applies overlays (named, layered patches) to a base YAML
directory to produce a final manifest piped into `kubectl apply`.

**Use Kustomize when:** you want the simplest "patch a base per environment"
model, you're deeply integrated with `kubectl`, and you don't need release
tracking or drift detection.

**Use keramos when:** you want versioned, named, auditable releases with rollback;
templating with expressions (computed values, conditionals, includes) instead
of overlay patching; signed packages and OCI distribution; drift detection and
reconcile.

**Composition:** keramos can run a post-renderer (`--post-renderer`) over its
output, so `kustomize build` can process rendered manifests before apply.

```sh
# Kustomize pattern
kustomize build overlays/prod | kubectl apply -f -

# keramos equivalent
keramos install my-app ./my-app --env prod -n prod
```

## Keramos vs kapp / kapp-controller

[**kapp**](https://carvel.dev/kapp/) is Carvel's deployment tool with diff and
waiting. [**kapp-controller**](https://carvel.dev/kapp-controller/) is the
in-cluster reconciler.

**Use kapp when:** you want zero templating, raw YAML in, smart server-side
diff out. A good companion to ytt.

**Use keramos when:** you want templating + deployment + release tracking +
signing in one binary, with a GitOps-native KeramosRelease CR pattern.

**Composition:** kapp can deploy keramos's rendered output —
`keramos template ./my-app | kapp deploy -a my-app -f -`.

## Keramos vs kpt

[**kpt**](https://kpt.dev/) is a Kubernetes-native package manager that mutates
YAML with KRM functions — arbitrary programs that read and write structured
resources.

**Use kpt when:** you want function-based pipelines for fine-grained
programmatic mutation (inject sidecars, normalise names across teams) and
you're committed to the KRM-functions paradigm.

**Use keramos when:** you want template-style substitutions (`${values.x}`)
instead of writing a function, plus first-class release tracking, drift
detection, and signing.

## Keramos vs Carvel ytt + vendir

[**ytt**](https://carvel.dev/ytt/) does YAML overlays with annotations.
[**vendir**](https://carvel.dev/vendir/) does vendored dependency fetching.
Together with kapp they form the Carvel suite.

**Use Carvel when:** you want a federated three-tool approach with strict
separation of concerns and you like ytt's overlay annotations.

**Use keramos when:** you want one binary instead of three, with consistent
vocabulary (`packages`, `layers`, `releases`, `workspaces`) across the
lifecycle.

## Keramos vs Argo CD

[**Argo CD**](https://argo-cd.readthedocs.io/) is a GitOps reconciler that
converges cluster state to state declared in git.

**Use Argo CD when:** you want a fully GitOps-native model where every change
goes through git, with a UI for sync status and drift visualisation.

**Use keramos when:** you want CLI-driven deployments with the same auditability,
OR combine the two — keramos as the packaging/templating layer, Argo CD as the
reconciler.

**Composition (recommended):** two patterns work.

1. **Pre-rendered manifests in git.** CI runs `keramos template ./my-app --env
   prod > manifests.yaml` and commits the raw YAML. Argo CD watches the
   rendered file and syncs it.
2. **CMP plugin.** Register `keramos template <package>` as an Argo CD Config
   Management Plugin. Argo CD invokes keramos on each sync; the package source
   lives in git.

→ Detailed walkthrough: [Use cases — GitOps](use-cases.md#for-gitops-teams-argo-cd-flux).

## Keramos vs Flux

[**Flux**](https://fluxcd.io/) is the other major GitOps reconciler. Its
`helm-controller` reconciles HelmRelease CRs; `kustomize-controller` reconciles
plain manifests.

**Use Flux when:** you want a multi-controller model with deep CRD-based
declaration of sources, kustomizations, and helm-releases.

**Use keramos when:** you want keramos's packaging primitives, OR combine them —
Flux's OCI source can pull keramos packages pushed with `keramos registry push`:

```yaml
apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: my-app
spec:
  url: oci://ghcr.io/my-org/charts/my-app
  ref:
    tag: 1.2.3
```

For full keramos semantics in-cluster, deploy `keramos controller` and use the
KeramosRelease CR.

## Keramos vs ad-hoc shell scripting

The most common alternative isn't a tool — it's a `make deploy` that calls
`kubectl apply -f manifests/`. This works until the team needs per-environment
config (you reinvent overlays), rollback (you reinvent revisions), drift
detection (you reinvent `kubectl diff`), signing (you bolt on cosign or PGP),
or distribution (you mirror tarballs to S3). Keramos provides all of these in one
binary. The migration is small: move manifests under `templates/`, write a
minimal `keramos.yaml`, and `keramos install`.

## Choosing the right tool

| Your priority | Best fit |
|---|---|
| Single-binary, versioned, signed, drift-aware Kubernetes packages | **Keramos** |
| Helm alternative with built-in drift detection, audit trail, multi-cluster | **Keramos** |
| Established Helm chart ecosystem (Artifact Hub charts) is the priority | Helm — or operate them through `keramos helm-compat` to keep keramos's release semantics |
| Switching from Helm to a modern alternative | **Keramos** with `keramos migrate` |
| Manifest patching only, no release tracking | Kustomize |
| Function-pipeline-driven YAML mutation | kpt |
| Federated suite, separation of concerns | Carvel (ytt + vendir + kapp) |
| Pure GitOps reconciler, keramos-agnostic | Argo CD or Flux |
| GitOps reconciler **plus** keramos packaging | Keramos + Argo CD or Keramos + Flux |

## Where next

- [Quickstart](guides/quickstart.md) — try keramos
- [Use cases](use-cases.md) — by role
- [FAQ](faq.md) — common questions
- [Documentation map](../README.md#documentation-map)
