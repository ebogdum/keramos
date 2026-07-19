---
title: "Troubleshooting"
nav_order: 10
---
{% raw %}
# Keramos troubleshooting — common errors and fixes

Look up the error text you saw and follow the fix. If your error isn't here,
open an issue with the exact message and the command that produced it.

## Install / upgrade errors

### `release X already exists`

You ran `keramos install` against a name that's already deployed. Use
`keramos upgrade` instead, or `keramos uninstall` first.

### `secrets "keramos.v1.X.v1" already exists`

A previous install crashed mid-way and left the storage Secret behind. Delete
it and re-install, or mop up with `keramos purge --force`:

```sh
kubectl delete secret keramos.v1.X.v1 -n <namespace>
```

### `release X v1 not found for update`

Keramos tried to update a revision that isn't in the storage backend — usually the
release Secret was deleted out of band. Confirm with `keramos list`; re-install if
needed.

### `encoded release size N bytes exceeds K8s Secret limit of 1048576 bytes`

The rendered manifest plus stored hooks plus audit data exceed the 1 MiB Secret
cap. Trim large CRDs out of `crds/` and apply them separately, reduce inline
value verbosity, or switch to the SQL driver (`KERAMOS_DRIVER=sql`).

### `schema validation failed: replicas: 100 exceeds maximum 50`

`values.schema.json` rejected your inputs. Check the named path against the
schema — open the package's `values.schema.json`
([reference](reference/values-schema-json.md)) to see the active constraints.

### `value <key> is required`

`values.schema.json` declares a required field missing from your merged values.
Set it (`--set key=value` or a values file) or relax the schema.

### `top-level key 'reples' not in schema (did you mean 'replicas'?)`

The schema has `additionalProperties: false`. Fix the typo.

### `package %q resolves outside allowlisted root`

Controller-mode rejection: a KeramosRelease CR's `spec.package`, after symlink
resolution, points outside the controller's `--package-root`. Fix the CR or
extend `--package-root`.

### `plan integrity check failed: package or values changed since plan was generated`

The package or its values changed between `keramos plan` and `keramos apply`, so the
plan's SHA-256 no longer matches a fresh re-render. Regenerate the plan
(`keramos plan -o new.plan`) or revert the package change.

### `unsupported plan kind X/Y`

The plan file's `apiVersion` / `kind` aren't `keramos/v1` / `Plan` — an older plan
format or a corrupted file. Regenerate it.

## Templating errors

### `unknown function "eq"`

Keramos's expression engine has no `eq`, `ne`, `lt`, `gt`, `and`, `or`, or `not`
functions. (It *does* have `coalesce`, `default`, and `ternary`.) For
comparisons, use `$switch` or restructure as a discriminator field:

```yaml
# Won't work:
$if: ${eq values.env "prod"}

# Use $switch instead:
$switch: ${values.env}
$cases:
  prod: { resources: { requests: { cpu: 500m } } }
  staging: { resources: { requests: { cpu: 200m } } }
```

→ [Math & logic functions](templates/functions/math-logic.md),
[Expressions — Truthy `$if`](templates/expressions.md#truthy-if-evaluation).

### `function "add" failed: expected numeric, got string`

You passed a path-shaped argument (`${add values.a values.b}`). Function
arguments are parsed as **literals**, not paths. Use the pipeline form so the
value before `|` is the path lookup:

```yaml
${values.a | add 3}          # values.a is the path, 3 is a literal
${values.a | mul values.b}   # ✗ values.b is the literal string "values.b"
```

For two-path arithmetic, pre-compute the result in values.
→ [Expressions — arguments are literals](templates/expressions.md#important-function-arguments-are-literals-not-paths).

### `until: range size N exceeds 65536`

You're materialising a list larger than keramos's range cap. Split the work into
batches.

### `repeat: count N exceeds 65536`

Same cap — `${"x" | repeat 1000000}` is bounded. Reduce the count.

### `expression: include cycle detected: X`

A `$include "name"` chain re-entered itself. Check `_helpers.yaml` for a partial
that includes itself transitively.

### `$include chain exceeds depth limit (64)`

A non-cyclical chain of distinct partials exceeds 64 levels. Refactor into
fewer levels.

### `partial "name" not found`

`$include: name` referenced a partial that doesn't exist. Partial names are the
top-level YAML keys in `_helpers.yaml`; check for a typo.

### `tpl: recursion depth exceeded`

`${tpl ...}` calls nested too deeply. Keramos caps `tpl` recursion at 16 levels.

### `lookup: cannot map X/Y`

The cluster's discovery doesn't know that GroupVersionKind — the CRD isn't
installed or the apiVersion is wrong. `lookup` retries once with a fresh
discovery refresh, then fails.

## Distribution / registry errors

### `unauthorized: authentication required`

You're pulling/pushing without credentials. Run `keramos login <host>`. Keramos also
reads `~/.docker/config.json`, so any prior `docker login` works.

### `x509: certificate signed by unknown authority`

For internal CAs, set `KERAMOS_CA_FILE=/path/to/ca.pem`. For self-signed dev/test,
pass `--insecure` on `keramos login` or the OCI TLS-skip flags (never in
production).

### `OCI plain-http: credentials suppressed (set KERAMOS_ALLOW_PLAINTEXT_AUTH=1 to override)`

Keramos refuses to send Basic Auth over plaintext HTTP by default. Fix the
registry to use HTTPS, or set `KERAMOS_ALLOW_PLAINTEXT_AUTH=1` (or pass
`--allow-plaintext-auth`) for explicit dev-mode opt-in.

### `redirect to different host "X" blocked (original: "Y")`

A repo redirected to a different host. Keramos blocks cross-host redirects to
prevent credential leakage. Fix the repo's index or contact its operator.

### `digest mismatch: expected sha256:abc..., got sha256:def...`

The archive on the server doesn't match its `index.yaml`-recorded digest —
usually someone re-published without regenerating the index. Re-run
`keramos repo index` on the publisher side.

### `archive contains absolute path: /etc/passwd`

A tar entry has an absolute path — the archive is hostile or malformed. Keramos
refuses to extract.

### `archive contains path traversal: ../../../etc/passwd`

A tar entry escapes the destination directory. Hostile or malformed; keramos
refuses.

### `archive contains symlink escaping destination: foo -> /etc/shadow`

A symlink inside the archive points outside the destination. Keramos skips all
symlinks during extraction; this error means one would have escaped. The
archive is hostile.

### `archive contains more than 65536 entries`

Defence against tar bombs that pass per-file and total-size caps with millions
of tiny entries. Hostile or malformed.

### `archive entry "name" size N exceeds 16777216 bytes`

A single tar entry declared a size larger than 16 MiB. Hostile.

## Signing / verification errors

### `provenance verification failed: unknown signer (key 0xABCD)`

The PGP signature was made by a key not in your keyring. Add the publisher's
public key:

```sh
keramos keyring add /path/to/signer.pub
```

### `provenance verification failed: archive digest sha256:def... does not match signed sha256:abc...`

The archive on disk doesn't match the digest the publisher signed — possible
tampering. Re-fetch the original archive.

### `provenance file (.prov) not found`

The repo doesn't ship a `.prov` for this artifact. Either drop `--verify`
(insecure) or use a repo that signs.

### `cosign verify failed: no matching signatures`

No cosign signature is attached to that OCI artifact. Use `cosign tree <ref>`
to see what's there, or drop the cosign verification step.

### `file ... is not a valid PGP public key`

`keramos keyring add` rejected the input because it doesn't parse as a PGP entity.
Confirm with `gpg --list-keys` that the file is a real public key.

### `refusing to install key with reserved filename "credentials.json"`

You tried to add a keyring file whose name collides with keramos's credential
store. Rename the input file.

## Cluster / API errors

### `secrets "keramos.v1.X.v1" is forbidden: User "..." cannot create resource "secrets"`

Keramos writes release records as Secrets in the install namespace, and your
kubeconfig user lacks `create/get/update/delete secrets` there. Grant RBAC, or
switch to the ConfigMap driver (`KERAMOS_DRIVER=configmap`) if Secrets are
restricted.

### `Deployment X/Y rollout failed: ProgressDeadlineExceeded`

The Deployment's progress deadline elapsed before all replicas became
Available. Common causes: ImagePullBackOff, CrashLoopBackOff, insufficient
quota, failing readiness probe. Run `kubectl describe deploy X -n Y` for
details.

### `apply: failed to apply X/Y: forbidden: User cannot patch resource`

Server-side apply requires `patch` plus `update` on the resource. Grant the
RBAC.

### `the server was unable to return a response in the time allotted`

The cluster API is overloaded or the operation is timing out. Raise `--timeout`,
or retry at a quieter time. For stuck destructive operations, see
[`keramos purge --force`](cli/purge.md).

## Workspace / orchestration errors

### `workspace member "X" depends on unknown member "Y"`

`keramos-workspace.yaml` member `X` lists `dependsOn: [Y]` but no member `Y`
exists. Fix the spelling or add the member.

### `workspace dependency cycle: A -> B -> A`

Members form a circular dependency. Workspaces can't install with cycles.
Restructure or split into separate workspaces.

## Migration

### `not a Helm chart: Chart.yaml not found in <path>`

`keramos migrate` was pointed at a directory without a `Chart.yaml`. The path must
be a Helm chart's root.

### `refusing to overwrite non-empty output directory ...; remove it first or set KERAMOS_MIGRATE_FORCE=1`

`keramos migrate` won't silently overwrite a non-empty output directory. Delete it
first, or set `KERAMOS_MIGRATE_FORCE=1` to acknowledge the data loss.

### `strict mode: N items require manual review`

`keramos migrate --strict` refused to commit because the migrator couldn't
translate everything cleanly. Drop `--strict` (the items are printed in the
conversion report) or hand-edit the chart before re-running.

## Storage / drivers

### `failed to read schema version`

The SQL driver couldn't read its meta table. Check the connection string
(`KERAMOS_DRIVER_SQL_DSN`).

### `release payload exceeds storage size limit of 1048576 bytes`

The memory driver enforces the same 1 MiB cap as the Secret driver to catch
oversized releases early in dev/test. Same fix as the Secret error: trim CRDs,
switch to SQL, or split the package.

## Where next

- [FAQ](faq.md) — common questions
- [Glossary](glossary.md) — terminology
- [Documentation map](../README.md#documentation-map)
{% endraw %}
