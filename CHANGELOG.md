# Changelog

All notable changes to keramos are documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.0] — 2026-07-18

### Breaking changes

- `keramos plan` no longer takes a release-name argument. Use `keramos plan [dir]`
  (default `.`); the release is derived from the package's `keramos.yaml` name and
  compared against the latest stored state. Override with `-r/--release`.
- `keramos diff` is now purely file-oriented (compare two dirs / manifests / value
  sets / git refs). Its old release/revision comparison and `--server-side` mode
  were removed — `plan` owns "vs state" and `drift --server-side` owns "vs live".
- `keramos drift` is now a three-way comparison (package ↔ state ↔ running) and no
  longer emits the old count-table (`-o table/json/yaml` of `DriftItem`).

### Security & correctness hardening (adversarial review)

- **Template engine:** capped output amplification in `repeat`/`indent`/`nindent`
  (chained `repeat` could reach gigabytes); made `untilStep`/`seq` overflow-safe;
  added a document nesting-depth guard (deep input crashed the process); rejected
  leading-dash paths in `sops`/`git diff --from-ref` (arg-injection); `$switch`
  with no match now omits its field instead of emitting `null`; `restoreInString`
  is O(n).
- **Registry / fetch:** the redirect policy now explicitly strips `Authorization`
  and `X-API-Key` on any unauthorized/plaintext hop (closes a same-host
  https→http credential leak and brings `X-API-Key` under the same governance);
  repo matching is host/path-boundary-aware; archive and OCI blob downloads are
  size-bounded.
- **Values:** `--set` array index is bounded (negative index panicked, huge index
  OOM'd); `--set` on a nil base no longer panics; value provenance prunes stale
  entries when a value changes shape.
- **Cluster ops:** `resolveNamespace` derives scope from the REST mapping, so
  cluster-scoped kinds are no longer mis-namespaced (apply 404 / delete orphan);
  a CRD placed in `templates/` now waits for Established before its custom
  resources apply; `cleanup-on-fail` deletes the resources it introduced;
  `reconcile` honours `resource-policy: keep`; rollback carries the full record.
- **Policy:** image-registry allowlist matches at a host boundary (no
  `registry.internal.evil.com` bypass); `imageNotTagged` parses the reference
  (registry ports/digests handled); unknown `severity` is rejected (fail-closed);
  `require.fields` enforces all array elements and treats an explicit `false`/`0`
  as present.
- **Diff:** type-mismatched fields report both sides instead of dropping a value;
  `stripDefaults` no longer hides user-set fields (`sessionAffinity`,
  `rollingUpdate`).
- **Dependency tree:** total node count is bounded so an aliased fan-out graph
  cannot rebuild subtrees exponentially.


### Changed — plan, diff, and drift are now three distinct comparisons

- `keramos plan` no longer takes a release-name argument. It takes a package
  directory (default `.`) and compares it against the current stored state,
  deriving the release identity from the package's `keramos.yaml` name. Use
  `-r/--release` to target a state stored under a different name. This makes
  plan the Terraform-style "desired vs state" command: `keramos plan` in a
  package directory just works.
- `keramos plan`'s terminal output is a per-resource change preview with, by
  default, provenance woven in: each resource shows the template file that
  emitted it (`from: <file>`), and each changed field is rendered line by
  line as `- old (state)` / `+ new ← <origin>`, where the origin traces the
  driving `${values.x}` back through the resolution chain (package-default,
  values-file, layer, profile, or `--set`). Origin resolves for direct
  scalar substitutions; array-nested values are not yet drilled.
- The apply-able JSON artifact (`--out <file>` or `--format json`) is
  unchanged, so `keramos apply --plan` keeps working; provenance is a
  review-time view only.
- `keramos diff` is now purely file-oriented — it never reads cluster or release
  state. Four modes: two package directories, two rendered manifest files, one
  package rendered under two value sets (`--from-values`/`--to-values`,
  `--from-set`/`--to-set`, `--from-profile`/`--to-profile`), or one package at
  two git revisions (`--from-ref`/`--to-ref`). Shared `-f`/`--set`/`--profile`
  apply to both sides. Smart per-resource diff by default; `--smart=false`
  for a raw unified diff. The old release/revision comparison and
  `--server-side` mode were removed from `diff` (plan/drift own state and
  live-cluster comparison respectively).

### Fixed — finish wiring half-implemented features

- `keramos adopt --labels` now actually attaches the labels to the adopted
  release (the flag was previously accepted and discarded). `--create-namespace`
  errors are surfaced instead of silently ignored.
- `keramos drift --server-side` compares the live cluster against a server-side
  apply dry-run (admission/defaulting reflected), re-homing the capability that
  moved out of `keramos diff`.
- `genPrivateKey "ed25519"` is now implemented (PKCS#8 PEM); it previously
  returned an unsupported-type error.
- Removed the orphaned `action.Drift` wrapper left unused after the drift
  command was rewritten.

### Added — transport opt-ins are now flags, not just env vars

- Global flags `--allow-plaintext-auth`, `--oci-plain-http`, and
  `--oci-insecure-skip-tls-verify` expose the previously env-var-only transport
  opt-ins (`KERAMOS_ALLOW_PLAINTEXT_AUTH`, `KERAMOS_OCI_PLAIN_HTTP`,
  `KERAMOS_OCI_INSECURE_SKIP_TLS`). Passing a flag is exactly equivalent to
  exporting the variable and applies to every command that fetches over the
  network. The env vars still work; an unset flag never clears an export.

### Fixed — wire up previously-accepted-but-ignored registry flags

- `repo add --insecure-skip-tls-verify`, `--pass-credentials`, and
  `--pass-credentials-all` are now persisted on the repository entry and
  honored by the fetch client: insecure skips certificate verification for that
  repo; the pass flags relax the default (cross-host redirects blocked) and
  forward the `Authorization` header — `--pass-credentials` on the first
  cross-host hop only, `--pass-credentials-all` on every hop. Credentials are
  still never forwarded over plaintext http:// unless `KERAMOS_ALLOW_PLAINTEXT_AUTH=1`.
  Previously these flags were accepted and silently discarded, and (a bug) were
  only read when `--username/--password` were also present.
- `keramos login --insecure` now records the opt-in on the stored credential and is
  honored per host by the HTTP and OCI clients (skip TLS verification for that
  registry). Previously the flag did nothing.

### Added — value provenance recorded in state

- `install` and `upgrade` now record, on the release, where every value was
  resolved from (package default, values file, layer, profile, or `--set`).
  This provenance persists in the stored state, so the origin of a running
  value survives even after the package changes. Read it back with
  `keramos get provenance <release>` (`-o table|json|yaml`).

### Changed — drift is now a three-way comparison

- `keramos drift [package-path]` compares three views side by side — the package
  as it renders now, the recorded state, and the live cluster — and reports,
  per resource and field, where they disagree. It flags two divergence classes:
  `⚠ cluster drift` (state ≠ running: the cluster was changed out of band) and
  `→ pending apply` (package ≠ state: local edits not yet applied). The release
  is derived from the package's keramos.yaml name, `-r` overrides. Comparison is
  limited to keramos-managed fields, so status/managedFields/defaults noise on the
  live side is ignored. Replaces the old `drift <release>` count table.

### Fixed

- The smart diff now drills into same-length lists element by element, so a
  change to one field of one element (e.g. a container image or a port) is
  reported at its exact indexed path (`spec.template.spec.containers.0.image`)
  instead of replacing the whole list. This also lets `keramos plan` attribute
  such changes to their source value. Lists whose length changes are still
  reported whole.

## [1.0.0] — first stable release

The first stable release of keramos — a Kubernetes package manager, YAML
templating engine, and drift-detection CLI written in Go.

### Template engine

- YAML-native expression engine with `${...}` substitution and
  pipeline syntax (`${values.x | upper | quote}`).
- Lowercase root namespaces: `values`, `release`, `package`,
  `capabilities`, `files`. Capitalised sub-keys on capabilities
  (`kubeVersion.Major`/`.Minor`/`.GitVersion`).
- YAML-level control-flow directives: `$if`/`$then`/`$else`,
  `$each`/`$as`/`$yield`, `$switch`/`$cases`/`$default`.
- `$include` partials with depth-bounded recursion and a
  `_helpers.yaml` convention.
- Hooks declared via `$hook` directive with `$weight`,
  `$delete-policy`, and per-hook `$timeout`.
- Files API: `files.Get`, `files.Glob`, `files.AsConfig`,
  `files.AsSecrets`, `files.Lines`.
- Per-render function registry — context-bound `tpl`, `lookup`,
  `include`, and `files.*` closures wrapped over a stateless base
  registry, safe under parallel rendering.
- ~180 built-in functions covering string, math, regex, date, crypto,
  encoding, type, collection, logic, path, secret, and Sprig-extra
  surfaces. RSA key generation clamped to a sane bit range; range,
  repeat, regex, and rand-* helpers carry allocation caps.
- `tpl` recursion guard and `$include` depth limit.
- `lookup` with per-render cache and automatic mapper-refresh on
  miss; live-cluster reads gated by an explicit context flag.
- Strftime → Go layout converter for `date` / `dateInZone`.
- Truthy/falsy rules cover string forms (`""`, `"false"`, `"0"`,
  `"no"`, case variants) so values round-tripped through env vars
  and `--set` keep their semantics.

### Package model

- `keramos.yaml` schema (`apiVersion`, `name`, `version`, `appVersion`,
  `kubeVersion`, `layers`, `requires`, `environments`, `profiles`,
  `immutables`, `dependencies`).
- `values.yaml` with deep-merge semantics across layers and `--set`.
- `values.schema.json` validation: `pattern`, `format`, `enum`,
  `const`, `multipleOf`, `min`/`max`/`exclusive*`, `minItems` /
  `maxItems` / `uniqueItems`, `minProperties` / `maxProperties`,
  `dependentRequired`, `patternProperties`, `allOf`/`anyOf`/`oneOf`/
  `not`, `$ref`/`$defs`/`definitions` with cycle detection.
- `_helpers.yaml` partials.
- `crds/` directory applied as a separate phase, waiting on
  `Established=true` before the regular manifest applies.
- `tests/` directory for `keramos test`.
- `files/` directory accessible from templates via the Files API.
- `notes.yaml` post-install message.
- `profiles/` for named overlays.
- `policies/` for in-package policy enforcement (keramos-native
  declarative match-and-require rules).
- `.keramosignore` exclusion file.

### Layers and composition

- `layers:` for in-release composition; `requires:` for separate
  releases.
- Layer sources: local path, HTTPS, git, OCI.
- Tag-based and condition-based layer enablement, evaluated against
  fully-merged values (parent overrides applied).
- Parent override of nested layer keys via `layers.<name>.<key>`.
- Forced precedence with the `!` suffix.
- `keramos.lock` auto-generation for reproducible resolution.

### Environments and profiles

- `environments:` block in `keramos.yaml` with `inherits:` chains,
  per-environment values, value-files, profile, namespace, and
  cluster context.
- Profiles independent of environments, applicable to any release.

### Release lifecycle

- `keramos install` / `keramos upgrade` (with `--install`,
  `--reset-values`, `--reuse-values`, `--only`, `--wait`,
  `--wait-for-jobs`, `--atomic`, `--cleanup-on-fail`, `--force`).
- `keramos rollback` — re-applies a previous revision and re-runs that
  revision's stored hooks.
- `keramos uninstall` (with `--keep-history`, `--no-hooks`).
- `keramos list` (`-A`, `--selector`, `--filter`, `--date`).
- `keramos status`, `keramos history`, `keramos get`
  (`manifest`/`values`/`hooks`/`notes`/`metadata`/`all`,
  `--revision`, `--template`).
- `keramos audit` — full chronological release trail (who, when, where,
  flags, values).
- `keramos diff` — smart, per-resource diffing against any historical
  revision, with an optional `--server-side` mode that diffs live
  cluster state against a server-side apply dry-run (reflecting
  API-server defaulting and admission-webhook mutation).
- `keramos plan` — deterministic rendered manifest with SHA-256
  integrity hash.
- `keramos apply` — re-renders, verifies the plan integrity hash, then
  applies.
- `keramos drift` — per-field comparison of stored manifest vs live
  state with smart filtering.
- `keramos reconcile` — converges cluster state back to the stored
  manifest.
- `keramos prune` — drops superseded revisions.
- `keramos rename` — copies revisions to a new release name.
- `keramos canary` — staged rollouts with bake periods between replica
  counts and automatic rollback.
- `keramos multi-install` — fleet-wide rollouts with
  `--atomic-cross-cluster` rollback semantics.

### Storage drivers

- Secret driver (default; `keramos.v1.<release>.v<rev>` naming).
- ConfigMap driver.
- Memory driver (in-process, with deep-clone isolation between
  reads and writes).
- SQL driver (PostgreSQL, SQLite, MySQL) with
  schema-versioned migrations executed under transactions.
- Connection-string env-var configuration for the SQL driver.

### Workspaces and cross-source orchestration

- `keramos-workspace.yaml` with members, defaults, and `dependsOn`.
- Kahn-level scheduling with parallel execution within a level.
- Health-gating between levels.
- Atomic-workspace mode with cross-cluster rollback.
- Continue-on-error mode for best-effort rollouts.
- `keramos-releases.yaml` for cross-source release groups.

### Distribution and registry

- `keramos package` — builds `*.keramos.tgz` archives with optional
  `--sign`, `--app-version`, and `--version` overrides.
- `keramos repo add` / `list` / `remove` / `update` / `index` for
  HTTPS chart-style repositories.
- `keramos pull` — HTTPS or OCI, with `--version` constraint, `--prov`
  provenance download, and TLS material flags
  (`--ca-file`/`--cert-file`/`--key-file`).
- `keramos push`.
- `keramos publish` — HTTP API or OCI, with bounded response bodies
  and per-call timeouts.
- `keramos registry login` / `logout` / `push` / `pull`, with
  `--insecure-skip-tls-verify` and `--plain-http`.
- `keramos search` — Artifact Hub plus repo search, with
  `--endpoint`, `--kind`, and repository-URL output.
- `keramos show` — chart, values, readme, crds, all (with bounded
  archive entry sizes).
- OCI distribution via ORAS.
- Authenticated client with retries, redirects, mTLS, cross-host
  redirect blocking, and refusal of plaintext-HTTP Basic Auth
  unless explicitly opted in.

### Signing and verification

- `keramos package --sign` — PGP `.prov` provenance file emission.
- `keramos keyring add` / `list` / `remove` — operator-controlled
  trust store with PGP public-key validation on add.
- `keramos verify` — standalone verification.
- `keramos install --verify` — fail-closed install gated on signature.
- Cosign integration — verify an OCI artifact's external cosign
  signature (key-based or keyless) before pulling it, via
  `keramos registry pull --cosign-key` / `--cosign-identity` +
  `--cosign-issuer` (fail-closed).

### Cluster operations

- Server-side apply with `FieldManager=keramos`.
- Deterministic install order (`CustomResourceDefinition` first,
  then namespaces, RBAC, and the rest).
- `WaitForReady` for `Deployment`, `StatefulSet`, `DaemonSet`,
  `Job`, `CronJob`, `Pod`, `ReplicaSet`, `ReplicationController`.
- Permanent-failure detection for `ProgressDeadlineExceeded` and
  `ReplicaFailure` so waits don't hang on broken rollouts.
- `managedBy=keramos` label stamped on every applied resource and on
  the namespace itself.
- Stamped pod templates inside `Deployment`, `StatefulSet`,
  `DaemonSet`, `ReplicaSet`, `ReplicationController`, `Job`,
  `CronJob` so embedded controllers are queryable cluster-wide.
- Capabilities API (`kubeVersion`, `apiVersions.Has`).
- `helm.sh/resource-policy: keep` honoured across all delete paths
  (uninstall, upgrade `--force`, atomic cleanup).

### Controller and CRDs

- `KeramosRelease` Custom Resource Definition.
- `keramos controller` — in-cluster reconciler with package-root
  allowlist, per-cycle eviction of deleted CRs, and structured
  status logging.

### Tooling

- `keramos create` — scaffold a working package.
- `keramos init` — template-based scaffold (operator, batch, blank,
  web app).
- `keramos lint` — schema, template, manifest, and best-practice
  validation.
- `keramos template` — local rendering, no cluster contact.
- `keramos dev` — watch and re-render on every save.
- `keramos config` — interactive walker over `values.schema.json`.
- `keramos values --trace` — show the resolved value source per key.
- `keramos policy` — run policies against rendered manifests.
- `keramos scan` — find common values across packages and extract a
  base layer.
- `keramos sbom` — CycloneDX 1.5 SBOM emission per release.
- `keramos metrics` — sample CPU and memory of running workloads and
  recommend `requests` / `limits`.
- `keramos graph` — rendered-manifest resource relationship graph
  (workload→ConfigMap/Secret mounts, hook ordering).
- `keramos adopt` — claim existing resources into a release record.
- `keramos migrate` — Helm-chart-to-keramos-package converter.
- `keramos helm-compat` — interop layer that runs unmodified upstream
  Helm charts under keramos's release record.

### Plugins and marketplace

- `keramos plugin install` / `list` / `remove` / `update` from git URL
  or local path, with split install / update / delete hooks.
- Plugin-as-extension subcommands.
- `keramos marketplace search` / `verify` against a signed plugin
  index.
- `plugin.yaml` schema validation (known-fields-only decode).
- Cross-platform plugin runner (POSIX shell on Unix, `cmd.exe` on
  Windows).
- Compound URL-scheme matching (`s3+https`, `git+ssh`).
- TLS-material plumbing for downloaders.

### Operations and forensics

- `keramos purge` — cluster-wide cleanup driven by the
  `managedBy=keramos` selector (with legacy-label fallback).
- `keramos purge --force` — drains stuck namespaces, force-deletes
  pods (two-pass with `grace=0`), force-finalises terminating
  namespaces, and sweeps orphan pods left behind by node failure.
- `keramos purge --delete-namespaces` for full namespace teardown.
- Status redaction for secret-shaped strings in CLI output.
- `keramos test` with `--parallel`, `--retries`, and per-test
  defer-based cleanup.

### Security

- SSRF-proof dial layer — blocks loopback, link-local, RFC1918,
  CGNAT, and cloud metadata IPs.
- TLS 1.2 minimum across all outbound connections.
- Tar extraction protections: path-traversal rejection, entry-count
  cap, per-entry size cap, file-mode sanitisation, symlink refusal.
- Migration safety: symlink rejection on input, refusal to
  overwrite a non-empty output directory unless explicitly
  forced.
- Plan-time path/action validation.
- PGP keyring with operator-controlled trust and public-key
  validation on add.
- Render-time HTTP / Vault calls opt-in via env var, default off.
- Per-call timeouts on OCI push/pull, marketplace, and publish
  paths; bounded error-response body reads.
- Resource caps on `repeat`, `indent`, `nindent`, `until`,
  `untilStep`, `randAlphaNum`/`randAscii`/`randNumeric`/
  `randAlpha`, and regex find-all/split.
- Function-argument literal-only model (no path injection from the
  argument side of a pipeline).
- Plugin-install argument hardening (dash-prefix rejection,
  `--` separator).
- Cross-host redirect blocking and refusal of plaintext-HTTP Basic
  Auth without an explicit opt-in.

### Documentation

- MIT licence.
- Comprehensive README with quickstart, feature overview, and
  Helm-alternative comparison.
- ~100 per-command CLI reference pages.
- Per-template-field reference (expressions, control flow,
  functions, capabilities, layers, hooks).
- Format references for `keramos.yaml`, `values.yaml`,
  `values.schema.json`, `keramos-workspace.yaml`, `keramos-releases.yaml`.
- Guides: quickstart, layers, workspaces, signing, migration from
  Helm, GitOps integration.
- FAQ, glossary, troubleshooting, comparison, and use-case pages.

[1.0.0]: https://github.com/ebogdum/keramos/releases/tag/v1.0.0
