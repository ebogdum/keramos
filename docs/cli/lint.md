# keramos lint

`keramos lint` validates a package for correctness — metadata, values, schema,
and a full render — and reports every problem it finds without touching a
cluster.

## When to use it

- As a pre-commit or CI gate, before `keramos package` or `keramos publish`.
- After editing values or templates, to confirm the package still renders.
- With `--strict` when you want warnings (missing `templates/`, base
  overrides) to fail the build too.

## What happens

1. Reads `keramos.yaml` and checks its structure: `apiVersion` must be
   `keramos/v1`, `name` is required, and `version` must be valid semver.
2. Parses `values.yaml` and `values.schema.json` (each optional) and reports
   any that is malformed.
3. Confirms `templates/` exists and holds at least one `.yaml` file, warning
   if not.
4. If the structural checks passed, resolves layers, merges values
   (`-f` + `--set` + `--profile`), and renders every template; a render
   failure is reported as an error.
5. Checks that a declared `base:` exists and has its own `keramos.yaml`, that a
   named `--profile` exists under `profiles/`, and warns on templates that
   override a base template.
6. Prints each finding as `[ERROR]` or `[WARNING]`, then a final line. Exits
   non-zero on any error, or on any warning under `--strict`.

## Usage

```
keramos lint <package-path> [flags]
```

Purely local: lint never contacts a cluster, so the inherited `--kube*` flags
have no effect here.

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-f, --values` | stringArray | — | merge a values file before the render check, so lint sees the same values you deploy with (repeatable) |
| `--set` | stringArray | — | override one `key=value` before the render check (repeatable) |
| `--profile` | string | — | apply and validate the `profiles/<name>` overlay; a missing profile is an error |
| `--strict` | — | false | promote every warning to an error, so the command fails on warnings alone |

## Worked example

**INPUT — the package `./web` with one deliberate mistake.** `keramos.yaml`
carries a version that is not valid semver:

```yaml
# web/keramos.yaml
apiVersion: keramos/v1
name: web
version: "1.2"          # ← not semver; needs three components, e.g. 1.2.0
```

```yaml
# web/values.yaml
replicas: 2
```

```yaml
# web/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: ${values.replicas}
```

**Run it:**

```sh
keramos lint ./web
```

**OUTPUT:**

```
[ERROR] keramos.yaml: version "1.2" is not valid semver
Error: lint failed: 1 error(s), 0 warning(s)
```

The diagnostic points straight at the input: `[ERROR] keramos.yaml:` names the
file, and `version "1.2" is not valid semver` echoes the exact value that
failed rule 1. The command exits non-zero, so a CI step stops here. Because a
structural error was found, lint stops before the render check — fix the
version and rerun to reach it.

Fix the version to a valid semver and lint passes:

```yaml
version: 1.2.0
```

```sh
keramos lint ./web
```

```
lint passed
```

## See also

- [`template`](template.md) — render the package to inspect its output
- [`policy`](policy.md) — check rendered manifests against package policies
- [`test`](test.md) — run a deployed release's tests
- [`plan`](plan.md) — preview changes against the recorded state
