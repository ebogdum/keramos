# Keramos use cases — for platform engineers, SREs, and GitOps teams

This page maps keramos's features onto real workflows. Pick the role or pattern
that matches your team and follow the links.

## For platform engineers

You're building an internal developer platform on Kubernetes. Application teams
need a predictable way to ship workloads. Compliance wants signed artifacts and
audit trails. Operations wants drift detection.

**What keramos gives you:**

- A versioned, signable package format (`*.keramos.tgz`) capturing everything an
  app needs to run.
- A composition system (layers) so platform-provided base layers (logging,
  RBAC, network policies) flow into every app package.
- Per-release audit data (who, when, where, with what flags) baked into the
  cluster's release record.
- An OCI distribution model that reuses your existing registry and IAM.
- A workspace orchestrator for multi-component rollouts.
- Cluster-wide ownership queries via one label selector (`managedBy=keramos`).

**Workflow recipes:**

- **Standard library of layers** — publish base layers (`org-base-rbac`,
  `org-base-monitoring`) to OCI; consumers pull them in via `layers:` in
  `keramos.yaml`. Updates propagate by bumping the version constraint and running
  `keramos dependency update`.
- **Multi-environment promotion** — define `environments:` (`dev`/`staging`/
  `prod`) in `keramos.yaml` so the same package flows through environments. CI runs
  `keramos plan --env staging -o staging.plan`, then `keramos apply --plan
  staging.plan` after review.
- **Self-service signed packages** — teams run `keramos package --sign` against
  CI's PGP key; each cluster's keyring admits only that key, so tampered
  packages fail `keramos install --verify`.
- **Auditable rollouts** — `keramos audit <release>` answers "who upgraded the
  auth service yesterday?" months later.

→ Start with: [Quickstart](guides/quickstart.md), [Layers](guides/layers.md),
[Workspaces](guides/workspaces.md), [Signing](guides/signing.md).

## For SRE / operations teams

You operate clusters. You need to know what's deployed, what's drifted, and
that rollback works.

**What keramos gives you:**

- `keramos list -A` — every release in every namespace.
- `keramos drift ./pkg` — three-way compare of package, recorded state, and live
  cluster, with cluster-injected noise filtered out.
- `keramos reconcile <release>` — converge cluster state back to the stored
  manifest by release name (no package source needed).
- `keramos audit <release>` — full chronological history.
- `keramos rollback <release> <rev>` — re-apply a previous revision and re-run its
  hooks.
- `keramos metrics <release>` — sample CPU/memory and recommend requests/limits.
- `keramos multi-install --to <ctx-list> --atomic-cross-cluster` — fleet-wide
  rollouts with rollback.
- `keramos canary --stages 1,3,5 --bake 5m` — staged upgrades with bake periods.

**Workflow recipes:**

- **Drift-detection cron** — enumerate releases with `keramos list -A -q`, then run
  `keramos drift ./pkg -r <release>` per package (the check renders the package, so
  the source tree must be available); alert on any non-empty divergence.
- **Incident rollback drill** — practise `keramos rollback <release> <prev-rev>`
  against staging before the next on-call. The behaviour is identical in prod.
- **Capacity right-sizing** — run `keramos metrics <release>` after a release has
  baked for 24 hours; commit the recommended `requests` / `limits` to the
  package's `values.yaml`.
- **Force-cleanup after node failure** — `keramos purge --yes --force` clears
  wedged releases and force-finalises stuck `Terminating` namespaces.
- **Ordered platform teardown** — a `keramos-releases.yaml` with `dependsOn`
  brings the platform up (`keramos releases install`) and tears it down in reverse
  (`keramos releases uninstall`).

→ Start with: [`keramos drift`](cli/drift.md), [`keramos reconcile`](cli/reconcile.md),
[`keramos audit`](cli/audit.md), [`keramos canary`](cli/canary.md),
[`keramos purge`](cli/purge.md).

## For GitOps teams (Argo CD, Flux)

You declare desired state in git and reconcile it into the cluster. You want
keramos's packaging story without giving up your reconciler.

**What keramos gives you:**

- `keramos template` produces raw, deterministic manifest YAML you can commit for
  a reconciler to sync.
- `keramos plan -o app.plan` produces an apply-able JSON artifact (rendered
  manifest plus a SHA-256 digest); `keramos apply --plan app.plan` re-renders,
  verifies the digest, and applies — keramos's own integrity-checked pipeline.
- `keramos controller` is an in-cluster KeramosRelease CR reconciler if you want keramos
  to reconcile directly.
- The `keramos` binary can be registered as an Argo CD Config Management Plugin.
- Flux's OCI source can pull keramos packages pushed via `keramos registry push`.

**Workflow recipes (Argo CD):**

- **CMP plugin** — register `keramos template <package>` as a CMP that emits the
  rendered manifest. Argo CD diffs and syncs as usual.
- **Pre-rendered manifest in git** — CI runs `keramos template ./app --env prod >
  manifests.yaml` and commits it to a path Argo CD watches.
- **KeramosRelease CR pattern** — commit `KeramosRelease` CRs; Argo CD syncs the CRs
  and `keramos controller` reconciles them.

**Workflow recipes (Flux):**

- **OCI source** — `keramos registry push` to OCI; Flux's OCIRepository pulls; a
  Flux Kustomization applies the rendered manifests.
- **KeramosRelease CR + keramos controller** — Flux syncs CRs; `keramos controller`
  reconciles.

→ Start with: [`keramos template`](cli/template.md), [`keramos plan`](cli/plan.md),
[`keramos apply`](cli/apply.md), [`keramos controller`](cli/controller.md).

## For application developers

You write code. You want to ship to Kubernetes without becoming a YAML expert.

**What keramos gives you:**

- `keramos create my-app` — scaffold a working package in seconds.
- `keramos init <template>` — start from a richer built-in template.
- `keramos dev ./my-app` — watch the package and re-render on every save.
- `keramos lint` — fast static validation before pushing to CI.
- `keramos template` — render locally, no cluster contact.
- `keramos diff` — show exactly what changes between two packages, value sets, or
  git revisions.
- `keramos test` — run smoke tests against a deployed release.
- `keramos config` — interactive walker that fills a values file from the schema.

**Workflow recipes:**

- **Local kind / k3d loop** — `keramos dev ./my-app -n dev --interval 1s`
  re-renders on every save; apply the output from another terminal.
- **Pre-commit lint** — wire `keramos lint .` into a pre-commit hook so malformed
  packages never land in `main`.
- **One-command smoke test** — `keramos install my-app . -n test
  --create-namespace && keramos test my-app -n test`.

→ Start with: [Quickstart](guides/quickstart.md), [`keramos create`](cli/create.md),
[`keramos dev`](cli/dev.md).

## For CI / release engineering

You automate deploys. You need predictability, integrity, and rollback paths.

**What keramos gives you:**

- `keramos plan` / `keramos apply --plan` — separate rendering from cluster contact,
  with a digest checked before apply.
- `keramos diff --from-ref v1.2.0 --to-ref HEAD ./chart` — compare two git
  revisions of a package.
- `keramos rollback` — automated rollback on failure.
- `keramos multi-install --atomic-cross-cluster` — deploy to many clusters with
  rollback.
- `keramos canary --stages 10,50,100 --bake 5m` — staged rollouts with health
  gating.
- `keramos sbom <release>` — emit a CycloneDX 1.5 SBOM for compliance.

**Workflow recipes:**

- **Plan-and-apply pipeline** — CI runs `keramos plan -o app.plan` and posts the
  change preview as a PR comment; merging triggers `keramos apply --plan app.plan`
  against prod.
- **Compare against a shipped revision** — dump a historical manifest with
  `keramos get manifest <release> --revision N -o yaml > old.yaml`, render the
  candidate with `keramos template ./chart > new.yaml`, then `keramos diff old.yaml
  new.yaml`.
- **Canary in CI** — after merge, CI runs `keramos canary <release> ./chart
  --stages 10,50,100 --bake 5m -n prod`; a failed stage auto-rolls back.

→ Start with: [`keramos plan`](cli/plan.md), [`keramos apply`](cli/apply.md),
[`keramos canary`](cli/canary.md), [`keramos sbom`](cli/sbom.md).

## For security and compliance teams

You audit. You sign. You verify. You want forensic trails.

**What keramos gives you:**

- PGP-signed `.prov` provenance files.
- Cosign-attached OCI signatures (verify with the standard cosign workflow
  before install).
- A local PGP keyring (`keramos keyring add/list/remove`) the operator owns.
- `keramos install --verify` to fail closed on missing or wrong signatures.
- Audit data on every release record (who, when, where, what flags, what
  values).
- `keramos sbom` — CycloneDX 1.5 SBOM per release.
- `managedBy=keramos` on every applied resource and namespace.
- Render-time network policy: HTTP/Vault calls off by default; the render-time
  dial layer blocks loopback, link-local, RFC 1918, and metadata IPs.

**Workflow recipes:**

- **Signed-only install policy** — operators add only the keys you sign with;
  any package not signed by an admitted key fails `keramos install --verify`.
- **Air-gapped install** — pull and mirror packages once; install offline.
  Render-time network knobs default off.
- **Forensic audit** — `keramos audit <release>` reads the trail; `keramos get
  manifest <release> --revision N` reproduces exactly what was applied.

→ Start with: [Signing](guides/signing.md), [`keramos keyring`](cli/keyring.md),
[`keramos audit`](cli/audit.md), [`keramos sbom`](cli/sbom.md).

## For teams switching from Helm

You use Helm today. The pain points are typical: go-template surprises, brittle
umbrella charts, `values-<env>.yaml` proliferation, no native drift detection,
sparse audit trail, manual multi-cluster rollouts. You want a Helm alternative
without throwing away your chart investment.

**What keramos gives you:**

- A **Helm-chart converter**: `keramos migrate ./chart -o ./pkg` translates
  `Chart.yaml`, `templates/`, `_helpers.tpl`, and dependencies; go-template
  constructs become keramos `${...}` expressions where the migrator can do it
  cleanly, and the rest is printed as a conversion report.
- A **Helm interop layer**: `keramos helm-compat install my-app
  /path/to/upstream-chart` runs upstream charts under a keramos release record —
  you get keramos's reconcile, rollback, and audit around the unmigrated chart.
- **Side-by-side coexistence**: keramos's `managedBy=keramos` label and `keramos.v1.*`
  Secret naming don't collide with Helm's, so one cluster hosts both during a
  phased migration.
- **Features Helm lacks natively**: drift detection (`keramos drift`), reconcile
  (`keramos reconcile`), per-revision audit (`keramos audit`), multi-cluster atomic
  deploys, plan/apply with integrity hashing, workspace orchestration with
  health gating.

**Workflow recipes:**

- **Phased migration from leaf charts.** Migrate a small chart first (`keramos
  migrate ./small-chart -o ./small-pkg`), deploy alongside existing Helm
  releases, build familiarity, then expand.
- **Wrap charts you can't fork.** For upstream charts (cert-manager,
  kube-prometheus-stack, ingress-nginx) where you want keramos's release semantics
  without maintaining a fork, use `keramos helm-compat install`, then `keramos audit`
  / `keramos reconcile` / `keramos rollback` by release name.
- **Unified view of mixed releases.** `keramos helm-compat install` records a keramos
  release, so compat-managed releases appear in `keramos list -A` alongside native
  keramos releases.

→ Start with: [Migration guide](guides/migration.md),
[`keramos migrate`](cli/migrate.md), [`keramos helm-compat`](cli/helm-compat.md),
[Keramos as a Helm alternative](comparison.md#keramos-as-a-helm-alternative).

## Cross-cutting concerns

### Multi-tenant clusters

Operators with `create/get` on `KeramosRelease` CRs install via `keramos controller`
while platform admins gate package paths with `--package-root`. Tenants can't
point the controller at host paths because the resolved package must live under
the allowlist.

### Air-gapped / sovereign environments

Pull once with `keramos pull`, mirror archives to a local OCI registry, install
offline. Render-time network calls (HTTP/Vault/SOPS) are opt-in and off by
default.

### Compliance-driven environments (PCI, HIPAA, SOC 2)

Keramos's audit trail, signed packages, and SBOM generation cover the
deployment-side controls these frameworks require. Combine with `keramos policy
check` for in-package policy enforcement.

## Where next

- [Quickstart](guides/quickstart.md) — first install
- [FAQ](faq.md) — common questions
- [Glossary](glossary.md) — terminology
- [Comparison with other tools](comparison.md)
