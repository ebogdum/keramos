# Package anatomy

A keramos package is a directory containing one declarative manifest (`keramos.yaml`), a defaults file (`values.yaml`), and a tree of YAML templates. This guide walks through every file and directory keramos recognises, what it's for, and the conventions around it.

## The minimum

```
my-pkg/
├── keramos.yaml
├── values.yaml
└── templates/
    └── deployment.yaml
```

Three files: a package manifest, a values file (may be empty), and at least one template. Everything else is optional.

## The full layout

```
my-pkg/
├── keramos.yaml                       # required — package identity, version, layers, environments
├── values.yaml                     # required — default values (may be empty: `{}`)
├── values.schema.json              # optional — JSON-Schema validation for values.yaml
├── templates/                      # required — YAML manifests (with ${...} expressions)
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── _helpers.yaml               # underscore prefix → partial, not rendered standalone
│   └── _imageref.yaml              # another partial
├── crds/                           # optional — CRDs applied first, waited for Established
│   └── widget-crd.yaml
├── hooks/                          # optional — lifecycle hooks (Jobs / Pods)
│   ├── pre-install.yaml
│   └── post-upgrade.yaml
├── tests/                          # optional — `keramos test` Pods (alternative: hooks/ with $hook: test)
│   └── connection.yaml
├── files/                          # optional — embedded files readable via .Files API
│   ├── default.conf
│   └── certs/
│       └── ca.pem
├── notes.yaml                      # optional — post-install message; templated like everything else
├── profiles/                       # optional — named value-file overlays (`--profile prod`)
│   ├── dev.yaml
│   ├── staging.yaml
│   └── prod.yaml
├── policies/                       # optional — package-defined policy rules (keramos policy YAML)
│   └── require-resources.yaml
├── README.md                       # optional — human documentation, surfaced by `keramos show readme`
├── LICENSE                         # optional
└── keramos.lock                       # auto-generated — pinned layer/require digests; commit it
```

## File-by-file

### `keramos.yaml`

The package manifest. Required. Declares `name`, `version`, `apiVersion`, layers, requires, environments, immutables, and metadata. Full reference: [`keramos.yaml`](../reference/keramos-yaml.md).

### `values.yaml`

Default configuration. Required (may be `{}`). Keramos deep-merges layer values, environment values, `-f` files, and `--set` flags on top of this. Full reference: [`values.yaml`](../reference/values-yaml.md). Authoring guide: [Values](values.md).

### `values.schema.json`

Optional JSON-Schema 2020-12 document describing the expected shape of merged values. When present, validation runs before render and aborts the operation on any violation with a precise path and reason. Reference: [`values.schema.json`](../reference/values-schema-json.md).

### `templates/`

Required directory. Every `*.yaml` (and `*.yml`) file is rendered through the template engine and the result is treated as Kubernetes manifests. Files starting with an underscore (`_*.yaml`) are **partials** — they are loaded into the engine's environment so other templates can `${include "_helpers.yaml"}` from them, but they are never rendered as standalone manifests.

#### Partials and includes

```yaml
# templates/_helpers.yaml
${define "imageRef"}
  ${values.image.repository}:${values.image.tag | default "latest"}
${end}
```

```yaml
# templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: app
          image: ${include "imageRef"}
```

`${include}` returns a string; `${tpl}` evaluates a template string from values; `${render}` is the same on a literal.

### `crds/`

Optional. Every YAML in `crds/` is treated as a `CustomResourceDefinition` (regardless of `kind` — keramos will still apply non-CRD docs from this directory but the contract is "things that must be ready first"). The behaviour:

1. `crds/` resources are applied **before** `templates/`.
2. Keramos waits for each CRD to reach `Established=true` before continuing.
3. Subsequent templates can therefore reference custom resources (like `Foo.example.com`) without race conditions.

CRDs do **not** get rendered through the template engine — they are applied as-is. This is a hard convention to keep CRDs portable.

### `hooks/`

Optional. Holds Jobs and Pods that run at specific lifecycle points. A hook is any YAML with a top-level `$hook:` directive (or a recognised `kind`+annotation pair). Full guide: [Hooks](hooks.md).

```yaml
# hooks/post-install.yaml
$hook: post-install
$weight: 5
$delete-policy: hook-succeeded

apiVersion: batch/v1
kind: Job
metadata:
  name: ${release.name}-migrate
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: migrate
          image: ${values.image.repository}:${values.image.tag}
          command: ["/migrate.sh"]
```

### `tests/`

Optional shorthand for `keramos test`-only hooks. Equivalent to placing a hook with `$hook: test` in `hooks/`. Tests are not run during install — only when the operator runs `keramos test <release>`.

### `files/`

Optional. The contents of `files/` are exposed to templates via the `.Files` API:

- `${Files.Get "default.conf"}` — file contents as a string.
- `${Files.Glob "certs/*.pem"}` — every match's contents (good for fanning out a ConfigMap or Secret).
- `${Files.Lines "default.conf"}` — file split into a list of lines.
- `${Files.AsConfig "configs"}` — every file under `configs/` as a `key: contents` map (`key` is the basename).
- `${Files.AsSecrets "secrets"}` — same but base64-encoded.

Binary data is supported; results are passed as strings (Go strings can hold arbitrary bytes).

### `notes.yaml`

Optional. Rendered through the template engine after install/upgrade. The rendered result is shown to the user and stored in the release record. Use it to print URLs, next steps, credentials lookup commands, etc.

```yaml
# notes.yaml
The release ${release.name} is live.

Service URL:
  http://${release.name}.${release.namespace}.svc.cluster.local

To get the admin password:
  kubectl -n ${release.namespace} get secret ${release.name}-admin -o jsonpath='{.data.password}' | base64 -d
```

### `profiles/`

Optional. A profile is a named values overlay. Activate one with `--profile <name>` (CLI), or with `profile: <name>` on a workspace member or environment.

```yaml
# profiles/prod.yaml
replicas: 5
resources:
  requests: { cpu: 500m, memory: 512Mi }
  limits:   { memory: 2Gi }
```

```sh
keramos install my-app . --profile prod
```

The profile file's contents are merged on top of `values.yaml` and below environment/CLI overrides.

### `policies/`

Optional. Package-defined policies that `keramos policy run` evaluates against the rendered manifest:

- **Keramos policy YAML** — declarative match-and-require rules; suitable for "every Pod must set runAsNonRoot" or "every Service must have a selector".

Policies live with the package so they ship with it.

### `README.md`

Optional human documentation. `keramos show readme <pkg>` prints it. Standard Markdown.

### `LICENSE`

Optional license file. Surfaced by `keramos show all`.

### `keramos.lock`

Auto-generated by `keramos dependency update`. Pins the resolved version, ref, and digest of every layer and required package. **Commit this file.** Without it, two builds of the same package can resolve different versions of the same layer if the constraint allows it.

## Templates: how rendering works

Keramos renders templates in this sequence:

1. **Load `keramos.yaml`** and resolve layers (recursively).
2. **Merge values** from layers (in declared order), the package's own `values.yaml`, environment selection, profile, `-f` files, then `--set` / `--set-file` / `--set-string` / `--set-json`.
3. **Validate** the merged values against `values.schema.json` if present.
4. **Render `crds/`** — passed through unchanged.
5. **Render `templates/`** — every non-underscore YAML is rendered with the engine. Partials (`_*.yaml`) are loaded but not emitted.
6. **Render `hooks/`** — same engine, with the hook directives extracted.
7. **Render `notes.yaml`** if present.
8. **Render `tests/`** — same engine.

Inside a template, the engine exposes four namespaces (lowercase, no leading dot):

- `values` — merged values map. See [Expression syntax](../templates/expressions.md).
- `release` — `{name, namespace, revision, isInstall, isUpgrade, isRollback, service}`.
- `package` — `{name, version, appVersion, apiVersion, ...}` mirroring `keramos.yaml`.
- `capabilities` — cluster info; `kubeVersion.{major, minor, version}`, `apiVersions.has(...)`. See [Capabilities](../templates/capabilities.md).

Plus the `Files` accessors (see above) — note these are bound by name (`Files.Get`, `Files.Glob`, etc.) rather than under a namespace.

## Naming conventions

- DNS-1123 names everywhere: `^[a-z]([-a-z0-9]*[a-z0-9])?$`, max 63 characters.
- Resource names should be derived from `${release.name}` so two releases of the same package don't collide.
- Selectors and labels: prefer `app.kubernetes.io/name: <pkg-name>` and `app.kubernetes.io/instance: ${release.name}` for compatibility with k8s tooling. (Keramos's own `managedBy=keramos` label is added automatically — you don't write it.)

## Lifecycle from author to operator

```
keramos create my-pkg              # author scaffolds
edit values.yaml, templates/, ...
keramos lint .                     # author checks
keramos template .          # author previews
git commit .                    # author versions
keramos package .                  # creates my-pkg-1.0.0.keramos.tgz
keramos push my-pkg-1.0.0.keramos.tgz oci://reg.example.com/charts/my-pkg
                                # author publishes

# operator side:
keramos pull my-pkg --version 1.0.0 -d ./pulled
keramos install my-app ./pulled/my-pkg -n prod --create-namespace
```

For details on any of those steps, see the corresponding guide in [`docs/guides/`](.) or the per-command reference under [`docs/cli/`](../cli/).
