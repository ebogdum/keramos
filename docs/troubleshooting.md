# Keramos Troubleshooting — Common Errors and Fixes

When keramos rejects an operation, the error message goes here. Look up the exact text you saw and follow the fix. If the error you're seeing isn't here, open an issue with the exact message and the command that produced it.

## Install / upgrade errors

### `release X already exists`

You ran `keramos install` against a release name that's already deployed. Use `keramos upgrade` instead, or `keramos uninstall` first.

### `failed to create release secret for X v1: secrets "keramos.v1.X.v1" already exists`

A previous install crashed mid-way and left the storage Secret behind. Delete it: `kubectl delete secret keramos.v1.X.v1 -n <namespace>`, then re-install. Or run `keramos purge --force` to mop up.

### `release X v1 not found for update`

Keramos tried to update a revision that doesn't exist in the storage backend. Usually means the release Secret was deleted out-of-band. `keramos list` to confirm; re-install if needed.

### `encoded release size N bytes exceeds K8s Secret limit of 1048576 bytes`

The rendered manifest plus stored hooks plus audit data exceed the 1 MiB Secret cap. Fix options: trim large CRDs out of `crds/` (apply them separately), reduce inline value verbosity, or switch to the SQL driver (`KERAMOS_DRIVER=sql`).

### `schema validation failed: replicas: 100 exceeds maximum 50`

`values.schema.json` rejected your inputs. Check the named path against the schema. Use `keramos show schema <pkg>` to see the active schema.

### `value <key> is required`

`values.schema.json` declared a required field that's missing from your merged values. Either set it (`--set key=value`) or remove the `required` from the schema.

### `top-level key 'reples' not in schema (did you mean 'replicas'?)`

Schema has `additionalProperties: false`. Fix the typo.

### `package %q resolves outside allowlisted root`

Controller-mode rejection: a KeramosRelease CR's `spec.package` would, after symlink resolution, point outside the controller's `--package-root`. Fix the CR, or extend `--package-root` to an explicit allowlist.

### `plan integrity check failed: package or values changed since plan was generated`

The package or its values changed between `keramos plan` and `keramos apply`. The plan's stored SHA-256 no longer matches a fresh re-render. Either regenerate the plan (`keramos plan ... -o new.plan`) or revert the package change.

### `unsupported plan kind X/Y`

The plan file's `apiVersion` / `kind` don't match `keramos/v1` / `Plan`. Either an older plan format or a corrupted file.

## Templating errors

### `unknown function "eq"`

Keramos's expression engine does not have `eq`, `ne`, `lt`, `gt`, `and`, `or`, `not`, or `coalesce` as runtime functions. Use `$switch` for string comparison or restructure as a discriminator field in values:

```yaml
# Won't work:
$if: ${eq values.env "prod"}

# Use $switch instead:
$switch: ${values.env}
$cases:
  prod: { resources: { requests: { cpu: 500m } } }
  staging: { resources: { requests: { cpu: 200m } } }
```

→ [Function reference — Logic](templates/functions.md#logic), [Expressions — Truthy `$if`](templates/expressions.md#truthy-if-evaluation).

### `function "add" failed: expected numeric, got string`

You called a math function with a path-shaped argument (`${add values.a values.b}`). Function arguments are parsed as **literals**, not paths. Use the pipeline form:

```yaml
${values.a | add 3}                # values.a is the path lookup, 3 is the literal
${values.a | mul values.b}         # ✗ doesn't work — values.b is the literal string
```

For two-path arithmetic, pre-compute the result in values. → [Expressions — function arguments are literals](templates/expressions.md#important-function-arguments-are-literals-not-paths).

### `until: range size N exceeds 65536`

You're trying to materialise a list larger than keramos's range cap. Split the work into batches or pass an explicit limit.

### `repeat: count N exceeds 65536`

Same — `${"x" | repeat 1000000}` is bounded. Reduce the count.

### `expression: include cycle detected: X`

A `$include "name"` chain re-entered itself. Check `_helpers.yaml` for a partial that includes itself transitively.

### `$include chain exceeds depth limit (64)`

A non-cyclical chain of distinct partials exceeds 64 levels. Refactor into fewer levels.

### `partial "name" not found`

`$include: name` referenced a partial that doesn't exist. Check `_helpers.yaml` for the literal block name (top-level YAML keys are partial names).

### `tpl: recursion depth exceeded`

`${tpl ...}` calls have nested too deeply. Keramos caps `tpl` recursion at 16 levels.

### `lookup: cannot map X/Y`

The cluster's discovery doesn't know about that GroupVersionKind. Either the CRD isn't installed or the apiVersion is wrong. The lookup function will retry once with a fresh discovery refresh, then fail.

## Distribution / registry errors

### `unauthorized: authentication required`

You're pulling/pushing to a registry without credentials. Run `keramos registry login <host>` (or `keramos login <host>` for HTTP repos). Keramos also reads `~/.docker/config.json` so any prior `docker login` works.

### `x509: certificate signed by unknown authority`

For internal CAs, set the env var `KERAMOS_CA_FILE=/path/to/ca.pem`. For self-signed dev/test, pass `--insecure-skip-tls-verify` (do not use in production).

### `OCI plain-http: credentials suppressed (set KERAMOS_ALLOW_PLAINTEXT_AUTH=1 to override)`

Keramos refuses to send Basic Auth over plaintext HTTP by default. Either fix the registry to use HTTPS, or set `KERAMOS_ALLOW_PLAINTEXT_AUTH=1` for explicit dev-mode opt-in.

### `redirect to different host "X" blocked (original: "Y")`

A repo redirected to a different host. Keramos blocks cross-host redirects to prevent credential leakage. The repo is misconfigured; fix the index or contact the operator.

### `digest mismatch: expected sha256:abc..., got sha256:def...`

The archive on the server doesn't match its `index.yaml`-recorded digest. Common cause: someone re-published the archive without regenerating the index. Re-run `keramos repo index` on the publisher side.

### `archive contains absolute path: /etc/passwd`

Tar archive contains an entry with an absolute path — the archive is hostile or malformed. Keramos refuses to extract.

### `archive contains path traversal: ../../../etc/passwd`

Tar archive contains a relative path that escapes the destination directory. Hostile or malformed; keramos refuses.

### `archive contains symlink escaping destination: foo -> /etc/shadow`

Symlink inside the archive points outside the destination. Keramos skips all symlinks during extraction; this error means a symlink would have escaped if keramos had not skipped it. The archive is hostile.

### `archive contains more than 65536 entries`

Defence-in-depth against tar bombs that pass per-file and total size caps with millions of tiny entries. The archive is hostile or malformed.

### `archive entry "name" size N exceeds 16777216 bytes`

A single tar entry declared a size larger than 16 MiB. Hostile.

## Signing / verification errors

### `provenance verification failed: unknown signer (key 0xABCD)`

The PGP signature on the package was made by a key not in your local keyring. Add the publisher's public key:

```sh
keramos keyring add /path/to/signer.pub
```

### `provenance verification failed: archive digest sha256:def... does not match signed sha256:abc...`

The archive on disk doesn't match the digest the publisher signed. Possible tampering. Re-fetch the original archive.

### `provenance file (.prov) not found`

Repo doesn't ship a `.prov` for this artifact. Either remove `--verify` (insecure) or use a repo that signs.

### `cosign verify failed: no matching signatures`

No cosign signature is attached to that OCI artifact. Use `cosign tree <ref>` to see what's there, or remove `--verify-cosign`.

### `file ... is not a valid PGP public key`

`keramos keyring add` rejected the input because it doesn't parse as a PGP entity. Confirm with `gpg --list-keys --keyid-format LONG <key-id>` that the file is a real public key.

### `refusing to install key with reserved filename "credentials.json"`

You tried to add a keyring file with a name that would collide with keramos's credential store. Rename the input file.

## Cluster / API errors

### `secrets "keramos.v1.X.v1" is forbidden: User "..." cannot create resource "secrets"`

Keramos writes its release records as Secrets in the install namespace. Your kubeconfig user doesn't have `create/get/update/delete secrets` in that namespace. Either grant RBAC or switch to the ConfigMap-backed driver (`KERAMOS_DRIVER=configmap`) if Secrets are restricted.

### `Deployment X/Y rollout failed: ProgressDeadlineExceeded`

The Deployment's progress deadline (default 10 min) elapsed before all replicas became Available. Common causes: ImagePullBackOff, CrashLoopBackOff, insufficient quota, failing readiness probe. `kubectl describe deploy X -n Y` for details.

### `Deployment X/Y replica failure: ...`

The Deployment controller couldn't create the new ReplicaSet — quota exceeded, RBAC mismatch, or admission webhook rejection. The error message contains the controller's reason.

### `apply: failed to apply X/Y: forbidden: User cannot patch resource`

Server-side apply requires `patch` plus `update` on the resource. Grant the appropriate RBAC.

### `the server was unable to return a response in the time allotted`

Cluster API is overloaded or the operation is timing out. Increase `--timeout`, or run at a quieter time. For destructive operations, see [`keramos purge --force`](cli/purge.md) which has its own resilient cleanup path.

## Workspace / orchestration errors

### `workspace member "X" depends on unknown member "Y"`

`keramos-workspace.yaml` member `X` lists `dependsOn: [Y]` but no member named `Y` exists. Fix the spelling or add the missing member.

### `workspace dependency cycle: A -> B -> A`

Members form a circular dependency. Workspaces cannot install with cycles. Restructure or split into separate workspaces.

### `workspace member %q references unknown dependency`

Same as above with a different framing.

## Migration

### `not a Helm chart: Chart.yaml not found in <path>`

`keramos migrate` was pointed at a directory without a `Chart.yaml`. The path must be a Helm chart's root.

### `refusing to overwrite non-empty output directory ./mypkg; remove it first or set KERAMOS_MIGRATE_FORCE=1`

`keramos migrate` won't silently nuke an existing non-empty output directory. Either delete it first or pass `KERAMOS_MIGRATE_FORCE=1` to acknowledge the data loss.

### `strict mode: N items require manual review`

`keramos migrate --strict` refused to commit the output because the migrator couldn't translate everything cleanly. Either drop `--strict` (the items are listed in `keramos-migration.md`) or hand-edit the chart before re-running.

## Storage / drivers

### `failed to read schema version`

The SQL driver couldn't read its meta table. Check the database connection (`KERAMOS_DRIVER_SQL_DSN`).

### `release payload exceeds storage size limit of 1048576 bytes (memory driver enforces parity with Secret driver)`

The memory driver enforces the same 1 MiB cap as the Secret driver to catch oversized releases at write time in dev/test environments. Same fix as the Secret error: trim CRDs, switch to SQL, or split the package.

## Where next

- [FAQ](faq.md) — common questions
- [Glossary](glossary.md) — terminology
- [Documentation map](../README.md#documentation-map)
- Open an issue at the GitHub repository if your error isn't here
