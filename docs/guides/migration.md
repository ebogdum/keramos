# Migrate a Helm chart to a keramos package — Helm to keramos conversion guide

This guide walks through converting (migrating) an existing **Helm chart** into a **keramos package** using the `keramos migrate` command. If you're searching for a **Helm chart converter**, **Helm chart migration tool**, or **how to import a Helm chart into keramos**, you're in the right place.

The `keramos migrate` command translates a Helm chart directory into a keramos package: it walks the Helm chart structure (`Chart.yaml`, `templates/`, `values.yaml`, `crds/`, `_helpers.tpl`, `NOTES.txt`, `requirements.yaml`/`Chart.lock`) and emits an equivalent keramos package, rewriting go-template constructs to keramos's `${...}` expressions where possible. Constructs the migrator can't translate cleanly are flagged in a `keramos-migration.md` review report inside the output directory.

The companion `keramos helm-compat` command provides the inverse direction: rendering a keramos package as a Helm-compatible artifact for downstream tooling that consumes Helm output (e.g. `helm template`-driven CI gates, Helm-aware OCI scanners, GitOps tools that only know Helm).

> **Glossary search hooks:** "Helm to keramos migration", "convert Helm chart", "Helm chart to keramos package", "Helm migrator", "Helm chart converter", "import Helm chart into keramos", "Helm-compat keramos", "Helm chart keramos replacement".

## When to migrate

You have an upstream Helm chart you want to install through keramos, **and** any of:

- You want keramos's expression syntax instead of go-templates with sprig.
- You want keramos's ownership labels, drift detection, audit trail, and signing.
- You want to slot the upstream chart into a keramos workspace alongside keramos-native packages.

If you only need a one-shot install of an upstream chart, you don't need migration — `keramos install` accepts a Helm chart's tarball or directory directly via the compatibility layer (`keramos helm-compat install`).

Migration is for **owning** the package long-term.

## The migrator's job

`keramos migrate` walks a Helm chart directory and produces a keramos package directory:

| Helm input | keramos output |
|---|---|
| `Chart.yaml` | `keramos.yaml` (with `apiVersion: keramos/v1`, layers translated, dependencies translated) |
| `values.yaml` | `values.yaml` (unchanged) |
| `values.schema.json` | `values.schema.json` (unchanged) |
| `templates/*.yaml` | `templates/*.yaml` (template body rewritten where possible) |
| `templates/_helpers.tpl` | `templates/_helpers.yaml` (named templates → keramos `${define}` partials) |
| `templates/NOTES.txt` | `notes.yaml` |
| `crds/*.yaml` | `crds/*.yaml` (unchanged) |
| `Chart.lock` | `keramos.lock` |
| `requirements.yaml` (Helm v2) | layers entries in `keramos.yaml` |

Inside templates, the migrator translates a curated set of go-template constructs to keramos expressions:

| Go-template | keramos |
|---|---|
| `{{ .Values.x }}` | `${values.x}` |
| `{{ .Release.Name }}` | `${release.name}` |
| `{{ if .Values.enabled }}` ... `{{ end }}` | `${if .Values.enabled}` ... `${end}` |
| `{{ range .Values.items }}` ... `{{ end }}` | `${range .Values.items}` ... `${end}` |
| `{{ toYaml .Values.x | nindent 4 }}` | `${values.x | toYaml | nindent 4}` |
| `{{ printf "%s-%s" $a $b }}` | `${printf "%s-%s" $a $b}` |
| `{{ tpl .Values.foo . }}` | `${tpl .Values.foo}` |
| `{{ lookup "v1" "Secret" "default" "x" }}` | `${lookup "v1" "Secret" "default" "x"}` |
| `{{ include "named" . }}` | `${include "named"}` |
| `{{ index .Values "foo" "bar" }}` | `${get .Values "foo" "bar"}` |

Conditional blocks, range blocks, and sprig functions (math, string, regex, date, crypto, etc.) are passed through with keramos's equivalents — keramos's expression engine implements every sprig function the migrator can't otherwise translate.

## Things the migrator can't translate

When the migrator finds a construct it can't translate cleanly, it emits the original token unchanged AND adds an entry to the migration's review list:

- `{{ with $foo := ... }}` over multiple variables.
- `{{ range $i, $e := ... }}` with explicit index naming.
- Heavily nested conditionals around YAML structure (which sometimes break a 1:1 line translation).
- Calls to functions keramos doesn't implement (rare; the migrator names them).

## Workflow

```sh
keramos migrate ./upstream-chart -d ./migrated/
# walks upstream-chart, writes ./migrated/<chart-name>/...
keramos lint ./migrated/<chart-name>
# review keramos-migration.md inside the output:
cat ./migrated/<chart-name>/keramos-migration.md
```

The `keramos-migration.md` report lists:

- What the migrator did automatically.
- What it left for manual review (with file + line references).
- Warnings (deprecated fields, ambiguous translations).

## Iteration

The migrator is deterministic and idempotent — re-running on the same input produces the same output. It's safe to:

1. Run `keramos migrate` to get a starting point.
2. Hand-edit the output to clean up review items.
3. Commit.
4. Re-run later when the upstream chart releases a new version, diff the output, and apply the upstream changes selectively.

## Compatibility layer (the inverse)

`keramos helm-compat` exposes keramos packages to Helm-aware tooling:

```sh
keramos helm-compat template my-app ./my-pkg                # Helm-shaped manifest output
keramos helm-compat export ./my-pkg -d ./helm-export/       # writes a Helm chart skeleton
```

This is useful when CI runs `helm-diff`, `helm-conftest`, or `helm-secrets`-style tools and you want keramos to look like the Helm chart it would have been. The export is best-effort — keramos's `${...}` expressions may end up as inert literal strings in the exported chart, so the export is suitable for static analysis but not for actual Helm install.

## Limits

The migrator is **template translation**, not behaviour translation. If the upstream chart relies on:

- A specific Helm release-name format used by clients later (`my-release-redis-master`), the migrated keramos package generates the same names (release name interpolation works the same way), but workflow tooling that scrapes the release record may need updates.
- The Helm release Secret schema (`sh.helm.release.v1.<release>.v<rev>`), keramos's Secret schema differs (`keramos.v1.<release>.v<rev>` with keramos-specific labels). Tools reading Helm releases directly need updating; `keramos helm-compat list` returns the keramos-style data.
- A specific Helm-only test pattern (`helm test` with `helm.sh/hook: test`), `keramos migrate` rewrites it to `$hook: test` with keramos's lifecycle.

## Summary

`keramos migrate` exists because most Kubernetes packages today are Helm charts and there's no point making users rewrite from scratch. It's a translation tool, not a 1:1 emulator — the goal is to land a working keramos package that you then own, not to run Helm under keramos forever.
