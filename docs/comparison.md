# Keramos vs Helm vs kustomize vs kapp vs kpt — Kubernetes Packaging and Deployment Tools Compared

A side-by-side comparison of keramos and the other tools in the Kubernetes packaging, templating, and deployment space. If you're searching for a **Helm alternative**, **Helm replacement**, or simply evaluating Kubernetes packaging options, this page gives you the side-by-side detail to choose.

## TL;DR

Keramos is a **Kubernetes package manager, templating engine, and modern Helm alternative** that bundles versioned releases, drift detection, signing, OCI distribution, and workspace orchestration into one binary. The closest categorical comparisons are:

| Tool | Categorically similar to keramos | Where they differ |
|---|---|---|
| **Helm** | full Kubernetes package manager | keramos adds drift detection, audit trails, multi-cluster, plan/apply, and YAML-native templating. |
| **kustomize** | YAML transformation | kustomize patches; keramos templates and tracks releases. |
| **kapp / kapp-controller** | release management | keramos also templates and signs. |
| **kpt** | KRM-based YAML mutation | kpt is function-pipeline-driven; keramos is template-driven. |
| **carvel ytt + vendir + kapp** | full packaging suite | keramos is one binary instead of three. |
| **Argo CD / Flux** | GitOps reconcilers | keramos is a packaging tool; the two compose. |

## Feature matrix

| Feature | **Keramos** | Helm | kustomize | kapp | kpt | ytt | Argo CD | Flux |
|---|---|---|---|---|---|---|---|---|
| YAML templating | yes (`${...}`) | yes (go-template + sprig) | overlays only | no | KRM functions | yes (overlays) | uses other tools | uses other tools |
| Output is always valid YAML | yes (by construction) | no (text-level templating) | yes | yes | yes | yes | n/a | n/a |
| Versioned releases | yes | yes | no (kubectl-apply only) | yes (apps) | no | no | yes (Application) | yes (HelmRelease/Kustomization) |
| Revision history | yes | yes | no | partial | no | no | yes | yes |
| Drift detection | yes (built in) | external (`helm-diff`) | no | partial | yes | no | yes | yes |
| Reconcile to known good | yes | no | no | yes | partial | no | yes | yes |
| Atomic install/rollback | yes | yes | no | yes | no | no | yes | yes |
| Lifecycle hooks | yes (per-revision persisted) | yes | no | yes | no | no | yes (sync waves) | partial |
| Layered package composition | yes (`layers:`) | sub-charts (umbrella) | overlays | no | partial | overlays | uses other tools | uses other tools |
| Multi-environment values | yes (`environments:`) | side files (values-`<env>`.yaml) | overlays | no | setters | data values | uses other tools | uses other tools |
| Workspace orchestration | yes (`keramos-workspace.yaml`) | no | no | no | no | no | application-set | uses kustomization |
| Multi-cluster deploy | yes (`keramos multi-install`) | no | no | no | no | no | yes (per Application) | yes |
| OCI distribution | yes | yes | partial | partial | yes | no | yes | yes |
| HTTP repository | yes | yes (`helm repo`) | no | no | no | no | yes (helm) | yes (helm) |
| PGP signing | yes (`.prov`) | yes (`.prov`) | no | no | no | no | uses cosign | uses cosign |
| Cosign signing | external | external | no | no | no | no | yes | yes |
| Audit trail per release | yes (who/when/where/flags) | sparse | no | no | no | no | partial (sync history) | partial |
| SBOM emission | yes (CycloneDX 1.5) | no | no | no | no | no | no | no |
| Plan/apply separation | yes | no | no | no | partial | no | yes (sync wave) | partial |
| In-cluster reconciler | yes (`keramos controller`) | external (Flux helm-controller) | no | yes (kapp-controller) | no | no | yes | yes |
| Migrate Helm charts | yes (`keramos migrate`) | n/a | no | no | no | no | no | no |

## Keramos as a Helm alternative

Keramos is the closest categorical peer to **Helm** in this list — both are Kubernetes package managers that turn a directory of templated YAML into a versioned, installable, distributable artifact. If you're searching for a **Helm alternative**, **modern Helm replacement**, or **how to migrate from Helm to keramos**, this section is the side-by-side detail.

### Why pick keramos over Helm

The case for keramos as a Helm alternative is concrete:

- **YAML-native templating that's always valid YAML by construction.** Helm's go-template + sprig is a text-level templating engine — it can produce malformed YAML (whitespace bugs, half-quoted strings, accidentally-emitted directive tokens). Keramos's `${...}` substitutions and `$if` / `$each` / `$switch` directives are themselves YAML, so the output cannot be syntactically broken.
- **Drift detection is built in.** `keramos drift <release>` reports per-field divergence between the stored manifest and live cluster state. Helm requires the third-party `helm-diff` plugin to get a similar (but more limited) view. `keramos reconcile <release>` then re-applies the stored manifest to converge.
- **Audit trail per revision.** Every `keramos install` / `keramos upgrade` / `keramos rollback` records who initiated it, the kubeconfig context, the keramos version, the CLI flags as passed, and the value files supplied. `keramos audit <release>` answers "who upgraded the auth service yesterday?" months later. Helm's metadata is sparser.
- **Multi-cluster deploys in one command.** `keramos multi-install --to ctx-eu,ctx-us,ctx-ap --atomic-cross-cluster` ships the same release to N clusters; Helm requires external orchestration.
- **Plan / apply separation.** `keramos plan` produces a deterministic rendered manifest with a SHA-256 integrity hash; `keramos apply` re-renders and verifies before applying. Helm has no native plan/apply split — `helm template` and `helm install` are independent invocations.
- **Workspace orchestration.** `keramos-workspace.yaml` runs many releases in topological order with per-level parallelism, health-gating, and atomic rollback. Helm has no native workspace concept.
- **First-class environments.** `environments:` block in `keramos.yaml` with `inherits:` chaining replaces the values-`<env>`.yaml side-file proliferation pattern Helm encourages.
- **Layer composition with explicit overrides.** Keramos's `layers:` lets a parent override a layer's contributions via `layers.<layer-name>.<key>` in its own values. Helm's umbrella-chart pattern requires hand-coordinating override paths and is brittle to upstream changes.
- **Per-revision hooks for accurate rollback.** When you roll back to revision N, keramos re-runs the hooks revision N originally shipped — not the current ones. Helm's behaviour can drift across upgrades.
- **One binary covers package, sign, push, install, upgrade, drift, reconcile, audit, SBOM, lint, plan, apply, and an in-cluster reconciler.** Helm covers a smaller surface and pushes drift / SBOM / multi-cluster / reconciliation onto a constellation of plugins and adjacent tools.

### What Helm still does better today

- **Ecosystem maturity.** Artifact Hub, the chart-museum convention, the prom/grafana/cert-manager/etc. published charts. Keramos packages are new; you'll write or migrate most of them.
- **Existing operator familiarity.** Most Kubernetes operators have been writing Helm charts for years. Keramos's vocabulary is similar enough to onboard quickly but not identical.

### Migration path: Helm to keramos

Three options, by how much commitment they need:

1. **Convert the chart with `keramos migrate`.** The migrator walks the Helm chart structure (`Chart.yaml`, `templates/`, `values.yaml`, `crds/`, `_helpers.tpl`, `NOTES.txt`, `requirements.yaml`/`Chart.lock`) and emits an equivalent keramos package. Go-template constructs translate to keramos's `${...}` expressions where the migrator can do it cleanly; constructs it can't translate are listed in `keramos-migration.md` for human review. Once committed, you own the package long-term in keramos's vocabulary.

   ```sh
   keramos migrate ./my-helm-chart -d ./my-keramos-package
   keramos lint  ./my-keramos-package
   cat ./my-keramos-package/keramos-migration.md   # review what needs hand-tuning
   ```

   → Walkthrough: [Helm-to-keramos migration guide](guides/migration.md).

2. **Operate the upstream Helm chart through `keramos helm-compat`.** If you don't want to fork the chart, keramos's compatibility layer runs Helm-shaped charts under keramos's release record. You get keramos's audit trail, drift detection, and reconcile model around an upstream chart you didn't author.

   ```sh
   keramos helm-compat install my-app /path/to/upstream-chart -n prod
   keramos drift my-app -n prod      # works
   keramos rollback my-app 1 -n prod # works
   ```

   → [`keramos helm-compat`](cli/helm-compat.md).

3. **Side-by-side, gradual migration.** Keep critical legacy charts in Helm while moving new packages to keramos. Keramos's `managedBy=keramos` label and `keramos.v1.<release>.v<rev>` Secret naming don't collide with Helm's `app.kubernetes.io/managed-by=Helm` and `sh.helm.release.v1.*`, so the two coexist in one cluster without interference. Migrate at your own pace.

### When to stay on Helm

If your team's primary value is the Artifact Hub catalogue (postgresql, redis, nginx-ingress, kube-prometheus-stack, cert-manager — all published as Helm charts with active maintainers) and your charts work without templating-engine quirks, the migration cost may exceed the benefit. Keramos's `keramos migrate` lowers that cost meaningfully but doesn't eliminate it. The tradeoff line is roughly: how often do you hit Helm template edge cases, drift detection gaps, multi-cluster orchestration friction, or audit-trail blindness?

### When Keramos is clearly the better choice

- You're building a new platform from scratch and don't want to inherit Helm's templating tax.
- You operate at multi-cluster scale and need atomic cross-cluster rollouts.
- You need drift detection and reconcile semantics natively, not as a plugin.
- You need first-class signing + audit + SBOM as a deployment-tool requirement (regulated environments).
- You want one binary covering the full lifecycle instead of stacking helm + helm-diff + helm-secrets + helm-conftest + flux helm-controller.

---

## Keramos vs kustomize

[**kustomize**](https://kustomize.io/) is the official Kubernetes manifest transformation tool. It applies overlays (named, layered patches) to a base YAML directory to produce a final manifest that's piped into `kubectl apply`.

**Use kustomize when:** you want the simplest possible "patch a base for each environment" model, you're already deeply integrated with `kubectl`, you don't need release tracking or drift detection, and the team's mental model is "raw manifests + targeted patches".

**Use keramos when:** you want versioned, named, auditable releases with rollback paths; you want templating with expressions (computed values, conditionals, includes) instead of overlay patching; you want signed packages and OCI distribution; you want drift detection and reconcile.

**Composition:** keramos's post-renderer can run kustomize on the rendered output. Vice versa, kustomize's `helmCharts:` doesn't apply to keramos packages but `kustomize build` can be invoked from a `pre-install` hook.

```sh
# kustomize pattern
kustomize build overlays/prod | kubectl apply -f -

# keramos equivalent
keramos install my-app ./my-app --env prod -n prod
```

## Keramos vs kapp / kapp-controller

[**kapp**](https://carvel.dev/kapp/) is Carvel's deployment tool with diff and waiting. [**kapp-controller**](https://carvel.dev/kapp-controller/) is the in-cluster reconciler.

**Use kapp when:** you want zero templating, raw YAML in, smart server-side diff out, and a focused deployment-only tool. Good companion to ytt for separation of concerns.

**Use keramos when:** you want templating + deployment + release tracking + signing in one binary, and you want a GitOps-native KeramosRelease CR pattern.

**Composition:** kapp can deploy keramos's rendered output (`keramos template ./my-app | kapp deploy -a my-app -f -`). Conversely, the carvel suite can be replaced by keramos entirely if your team is consolidating.

## Keramos vs kpt

[**kpt**](https://kpt.dev/) is a Kubernetes-native package manager that uses KRM functions to mutate YAML. Functions are arbitrary programs that read and write structured Kubernetes resources.

**Use kpt when:** you want function-based pipelines for fine-grained programmatic mutation (e.g. inject sidecars, normalise resource names across teams), you're committed to the KRM-functions paradigm, and you don't need template-style expression substitution.

**Use keramos when:** you want template-style substitutions (`${values.x}`) instead of "write a function", and you want first-class release tracking, drift detection, and signing.

## Keramos vs Carvel ytt + vendir

[**ytt**](https://carvel.dev/ytt/) does YAML overlays with annotations. [**vendir**](https://carvel.dev/vendir/) does vendored dependency fetching. Together with kapp they form the Carvel suite.

**Use Carvel when:** you want a federated three-tool approach with strict separation of concerns, you like ytt's overlay annotations, and your team has already adopted the suite.

**Use keramos when:** you want one binary instead of three, with consistent vocabulary (`packages`, `layers`, `releases`, `workspaces`) across the lifecycle.

## Keramos vs Argo CD

[**Argo CD**](https://argo-cd.readthedocs.io/) is a GitOps reconciler. It watches a git repo (or other sources) and converges cluster state to declared state.

**Use Argo CD when:** you want a fully GitOps-native model where every change goes through git, with a UI for sync status, drift visualisation, and per-Application history.

**Use keramos when:** you want CLI-driven deployments with the same auditability, OR you want to combine — use keramos as the packaging/templating layer and Argo CD as the reconciler.

**Composition (recommended):** keramos renders deterministic manifests; Argo CD syncs them. Two patterns work:

1. **Pre-rendered manifests in git.** CI runs `keramos plan ... -o manifest.yaml` and commits the rendered output. Argo CD watches the rendered file. Plan integrity (SHA-256) detects mid-pipeline tampering.
2. **CMP plugin.** Register `keramos template <package>` as an Argo CD Config Management Plugin. Argo CD invokes keramos on each sync. The package source lives in git; the plugin renders on every reconcile.

→ Detailed walkthrough: [Use cases — GitOps](use-cases.md#for-gitops-teams-argo-cd-flux).

## Keramos vs Flux

[**Flux**](https://fluxcd.io/) is the other major GitOps reconciler. Flux's `helm-controller` reconciles HelmRelease CRs; `kustomize-controller` reconciles plain manifests.

**Use Flux when:** you want a multi-controller reconciler model with deep CRD-based declaration of every aspect (sources, kustomizations, helm-releases).

**Use keramos when:** you want keramos's packaging primitives, OR combine them — Flux's OCI source can pull keramos packages directly:

```yaml
# Flux OCISource pulling a keramos package
apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: my-app
spec:
  url: oci://ghcr.io/my-org/charts/my-app
  ref:
    tag: 1.2.3
```

For full keramos semantics in-cluster, deploy `keramos controller` and use the KeramosRelease CR — equivalent to Flux's HelmRelease but for keramos packages.

## Keramos vs ad-hoc shell scripting

The most common alternative isn't a tool — it's a `make deploy` target that calls `kubectl apply -f manifests/`. This works until:

- The team needs different config per environment (you reinvent overlays).
- The team needs rollback (you reinvent revisions).
- The team needs drift detection (you reinvent `kubectl diff`).
- The team needs signing (you bolt on cosign or PGP scripts).
- The team needs distribution (you mirror tarballs to S3).

Keramos provides all of these in one tested, signed, audited binary. The migration path is small: move your manifests under `templates/`, write a minimal `keramos.yaml`, and `keramos install`.

## Choosing the right tool

| Your priority | Best fit |
|---|---|
| Single-binary, versioned, signed, drift-aware Kubernetes packages | **Keramos** |
| Modern Helm alternative with built-in drift detection, audit trail, multi-cluster | **Keramos** |
| Established Helm chart ecosystem (Artifact Hub charts) is the priority | Helm — or operate them through `keramos helm-compat` to keep keramos's release semantics |
| Switching from Helm to a modern alternative | **Keramos** with `keramos migrate` |
| Manifest patching only, no release tracking | kustomize |
| Function-pipeline-driven YAML mutation | kpt |
| Federated suite, separation of concerns | Carvel (ytt + vendir + kapp) |
| Pure GitOps reconciler, keramos-agnostic | Argo CD or Flux |
| GitOps reconciler **plus** keramos packaging | Keramos + Argo CD or Keramos + Flux |

## Where next

- [Quickstart](guides/quickstart.md) — try keramos
- [Use cases](use-cases.md) — by role
- [FAQ](faq.md) — common questions
- [Documentation map](../README.md#documentation-map)
