# Keramos Use Cases — Kubernetes Package Management for Platform Engineers, SRE Teams, and GitOps Workflows

This page maps keramos's features onto real workflows. Pick the role or pattern that matches your team and follow the links to the relevant guides.

## For platform engineers

You're building an internal developer platform on Kubernetes. Application teams need a predictable way to ship workloads without writing kubectl pipelines. Compliance wants signed artifacts and audit trails. Operations wants drift detection.

**What keramos gives you:**

- A versioned, signable package format (`*.keramos.tgz`) that captures everything an app needs to run on Kubernetes
- A composition system (layers) so platform-provided base layers (logging, RBAC, network policies) flow into every app package
- Per-release audit data (who, when, where, with what flags) baked into the cluster's release record
- An OCI distribution model that integrates with your existing container registry and IAM
- A workspace orchestrator for multi-component platform rollouts
- Cluster-wide ownership queries via a single label selector (`managedBy=keramos`)

**Workflow recipes:**

- **Standard library of layers** — publish base layers (`org-base-rbac`, `org-base-monitoring`, `org-base-network`) to OCI; consumers pull them in via `layers:` in `keramos.yaml`. Updates propagate by bumping the version constraint.
- **Multi-environment promotion** — define `environments:` (`dev`/`staging`/`prod`) inside `keramos.yaml` so the *same* package metadata flows through environments. CI pipelines run `keramos plan --env staging` then `keramos apply` after review.
- **Self-service signed packages** — application teams `keramos package --sign` against your CI's PGP key; platform's keyring on every cluster admits only that key. Tampered packages fail verification.
- **Auditable rollouts** — every install records flags, user, and source. `keramos audit <release>` answers "who upgraded the auth service yesterday?" months later.

→ Start with: [Quickstart](guides/quickstart.md), [Layers](guides/layers.md), [Workspaces](guides/workspaces.md), [Signing](guides/signing.md).

## For SRE / operations teams

You operate clusters. You need to know what's deployed, what's drifted, what's at risk. You need rollback that works.

**What keramos gives you:**

- `keramos list -A` — every release in every namespace
- `keramos drift <release>` — per-field comparison of stored manifest vs live state, with smart filtering for noise
- `keramos reconcile <release>` — converge cluster state back to the stored manifest in one command
- `keramos audit <release>` — full chronological history with audit data
- `keramos rollback <release> <rev>` — re-apply a previous revision and re-run that revision's hooks
- `keramos metrics <release>` — sample CPU/memory and recommend requests/limits
- `keramos multi-install --to <ctx-list> --atomic-cross-cluster` — fleet-wide rollouts with rollback
- `keramos canary` — staged upgrades with bake periods between replica counts

**Workflow recipes:**

- **Drift-detection cron** — schedule `keramos drift -A` against every namespace; alert on non-empty output. The list of managed releases is enumerable via `keramos list`, so the operator can drive its own scheduler.
- **Incident rollback drill** — practise `keramos rollback <release> <prev-rev>` against staging before the next on-call incident. The behaviour is identical in prod.
- **Capacity right-sizing** — run `keramos metrics <release>` after a release has baked for 24 hours; commit the recommended `requests` / `limits` to the package's `values.yaml`.
- **Force-cleanup after node failure** — `keramos purge --yes --force` clears wedged releases and force-finalises stuck Terminating namespaces.
- **Multi-cluster atomic platform** — define a `keramos-releases.yaml` with `dependsOn`-ordered releases; `keramos releases install` brings up the platform; `keramos releases uninstall` tears it down in reverse.

→ Start with: [`keramos drift`](cli/drift.md), [`keramos reconcile`](cli/reconcile.md), [`keramos audit`](cli/audit.md), [`keramos canary`](cli/canary.md), [`keramos purge`](cli/purge.md).

## For GitOps teams (Argo CD, Flux)

You declare desired state in git and reconcile it into the cluster. You want keramos's packaging story without giving up your GitOps reconciler.

**What keramos gives you:**

- `keramos plan` produces a deterministic rendered manifest you can commit to a git repo
- `keramos apply <plan>` re-checks the plan's integrity (SHA-256 of the manifest) before applying — detects drift between plan and apply
- `keramos controller` is an in-cluster KeramosRelease CR reconciler if you want keramos to do reconciliation directly
- The `keramos` binary can be invoked as a CMP (Config Management Plugin) for Argo CD
- Flux's OCI source can pull keramos packages directly via `keramos registry push`

**Workflow recipes (Argo CD):**

- **CMP plugin** — register `keramos template <package>` as a CMP that emits the rendered manifest. Argo CD diffs and syncs as usual.
- **Pre-rendered manifest in git** — CI runs `keramos plan ... -o manifest.yaml`, commits to a git path Argo CD watches. Plan integrity (SHA-256) ensures the rendered manifest hasn't been edited.
- **KeramosRelease CR pattern** — commit `KeramosRelease` CRs to git. Argo CD syncs the CRs; `keramos controller` reconciles them.

**Workflow recipes (Flux):**

- **OCI source + Kustomize controller** — `keramos registry push` to OCI; Flux's OCISource pulls; Kustomize-controller applies.
- **KeramosRelease CR + keramos controller** — Flux syncs CRs; `keramos controller` reconciles.

→ Start with: [`keramos plan`](cli/plan.md), [`keramos apply`](cli/apply.md), [`keramos controller`](cli/controller.md), [Workspaces guide](guides/workspaces.md).

## For application developers

You write code. You want to ship to Kubernetes without becoming a YAML expert.

**What keramos gives you:**

- `keramos create my-app` — scaffold a complete, working package in seconds (Deployment + Service + ConfigMap + helpers)
- `keramos init <template>` — start from richer templates (operator, batch, blank, web app)
- `keramos dev` — watch the package and re-render on every save, like `webpack-dev-server` for Kubernetes
- `keramos lint` — fast static validation before pushing to CI
- `keramos template` — render locally, no cluster contact, see what would be applied
- `keramos diff` — show exactly what would change on the next upgrade
- `keramos test` — run smoke tests against a deployed release
- `keramos config` — interactive walker that fills in `values.yaml` from the schema

**Workflow recipes:**

- **Local kind / k3d loop** — `keramos dev ./my-app -n dev --interval 1s` re-renders on every file save; `kubectl apply` in another terminal applies the output.
- **Pre-commit lint** — wire `keramos lint .` into a pre-commit hook so malformed packages never land in `main`.
- **One-command smoke test** — `keramos install my-app . -n test --create-namespace && keramos test my-app -n test`.

→ Start with: [Quickstart](guides/quickstart.md), [`keramos create`](cli/create.md), [`keramos dev`](cli/dev.md).

## For CI / release engineering

You automate deploys. You need predictability, integrity, and rollback paths.

**What keramos gives you:**

- `keramos plan` / `keramos apply` — separates rendering from cluster contact; signed artifacts in git
- `keramos diff --revision N` — compare the proposed render against any historical revision
- `keramos rollback` — automated rollback on canary failure
- `keramos multi-install --atomic-cross-cluster` — deploy to multiple clusters with rollback semantics
- `keramos canary --stages 1,3,5 --bake 5m` — staged rollouts with health gating
- Plan integrity check — the SHA-256 of the rendered manifest is recorded; `keramos apply` re-renders and verifies before applying
- `keramos sbom <release>` — emit a CycloneDX 1.5 SBOM for compliance

**Workflow recipes:**

- **Plan-and-apply pipeline** — CI runs `keramos plan` and posts the diff as a PR comment. Merging the PR triggers `keramos apply` against prod.
- **Promotion through environments** — `keramos plan --env staging`, review, `keramos apply`. Then bump the package's appVersion and repeat for prod.
- **Canary in CI** — after merge to main, CI runs `keramos canary --stages 10,50,100 --bake 5m -n prod`. Failed stages auto-roll back.

→ Start with: [`keramos plan`](cli/plan.md), [`keramos apply`](cli/apply.md), [`keramos canary`](cli/canary.md), [`keramos sbom`](cli/sbom.md).

## For security and compliance teams

You audit. You sign. You verify. You want forensic trails.

**What keramos gives you:**

- PGP-signed `.prov` provenance files
- Cosign-attached OCI signatures (verify with the standard cosign workflow before install)
- A local PGP keyring (`keramos keyring add/list/remove`) the operator owns
- `keramos install --verify` to fail closed on missing or wrong signatures
- Audit data on every release record (who, when, where, what flags, what values)
- `keramos sbom` — CycloneDX 1.5 SBOM emission per release
- `managedBy=keramos` label on every applied resource and namespace
- Render-time network policy: HTTP/Vault calls disabled by default; SSRF-proof dial layer (loopback, link-local, RFC1918, metadata-IP all blocked)

**Workflow recipes:**

- **Signed-only install policy** — operators add only the keys you sign with; any package not signed by an admitted key fails `keramos install --verify`.
- **Air-gapped install** — pull and mirror packages once; install offline. Render-time network knobs default off.
- **Forensic audit** — `keramos audit <release>` reads the audit trail; `keramos get manifest <release> --revision N` reproduces what was applied at any point.

→ Start with: [Signing](guides/signing.md), [`keramos keyring`](cli/keyring.md), [`keramos audit`](cli/audit.md), [`keramos sbom`](cli/sbom.md).

## For teams switching from Helm

You're using Helm today. The pain points are typical: go-template surprises, brittle umbrella charts, the values-`<env>`.yaml proliferation, no native drift detection, sparse audit trail, manual multi-cluster rollouts. You want a **modern Helm alternative** without throwing away your existing chart investment.

**What keramos gives you:**

- A built-in **Helm chart converter**: `keramos migrate ./chart -d ./pkg` translates `Chart.yaml`, `templates/`, `_helpers.tpl`, and dependencies into a keramos package; go-template constructs become keramos `${...}` expressions where the migrator can do it cleanly.
- A **Helm interop layer**: `keramos helm-compat install my-app /path/to/upstream-chart` runs upstream Helm charts under keramos's release record without converting templates — you get keramos's drift detection, audit trail, and reconcile model around the unmigrated chart.
- **Side-by-side coexistence**: keramos's `managedBy=keramos` label and `keramos.v1.*` Secret naming don't collide with Helm's `app.kubernetes.io/managed-by=Helm` and `sh.helm.release.v1.*`, so a single cluster can host both during a phased migration.
- **The features Helm doesn't have natively**: drift detection (`keramos drift`), reconcile (`keramos reconcile`), per-revision audit (`keramos audit`), multi-cluster atomic deploys (`keramos multi-install --atomic-cross-cluster`), plan/apply with integrity hashing, workspace orchestration with health-gating.

**Workflow recipes:**

- **Phased migration starting with leaf charts.** Migrate a single small chart first (`keramos migrate ./small-chart -d ./small-pkg`); deploy alongside existing Helm releases; build team familiarity; expand outward.
- **Wrap Helm charts you can't fork.** For upstream charts (cert-manager, kube-prometheus-stack, ingress-nginx) where you want keramos's release semantics but don't want to maintain a fork, use `keramos helm-compat install`. You keep upstream's update cadence and gain keramos's audit / drift / reconcile loop.
- **Run drift detection across mixed Helm + keramos releases.** `keramos helm-compat list` enumerates Helm releases the compat layer manages; `keramos drift` against each shows what drifted. Combine with `keramos list -A` for a unified view of every managed release in the cluster.

→ Start with: [Helm-to-keramos migration guide](guides/migration.md), [`keramos migrate`](cli/migrate.md), [`keramos helm-compat`](cli/helm-compat.md), [Keramos as a Helm alternative](comparison.md#keramos-as-a-helm-alternative).

## Cross-cutting concerns

### Multi-tenant clusters

Operators with `create/get` on `KeramosRelease` CRs can install via `keramos controller` while platform admins gate package paths via `--package-root`. Tenants cannot point the controller at host paths because the resolved package must live under the allowlist.

### Air-gapped / sovereign environments

Pull once with `keramos pull`, mirror archives to a local OCI registry, install offline. Render-time network calls (HTTP/Vault/SOPS) are opt-in and default off.

### Compliance-driven environments (PCI, HIPAA, SOC 2)

Keramos's audit trail, signed packages, and SBOM generation cover the deployment-side controls these frameworks require. Combine with `keramos policy` for in-package policy enforcement (keramos-native declarative rules).

## Where next

- [Quickstart](guides/quickstart.md) — first install
- [FAQ](faq.md) — common questions
- [Glossary](glossary.md) — terminology
- [Comparison with other tools](comparison.md)
