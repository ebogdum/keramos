---
title: "keramos template"
parent: "CLI"
---
{% raw %}
# keramos template

`keramos template` renders a package to Kubernetes YAML locally and prints it to
stdout — no cluster, no release record, no apply.

## When to use it

- To see the exact manifests a package produces before you install or upgrade.
- To pipe rendered YAML into another tool: `keramos policy check`, `kubectl
  apply -f -`, `kubeconform`, or a post-renderer.
- To debug how values, profiles, and `--set` overrides resolve into output.

## What happens

1. Resolves `<package-path>` and its layers. If `--env` is given, the named
   environment's profile and value files fold in at the lowest precedence.
2. Merges values: `values.yaml` defaults, then `-f` files, then `--set` /
   `--set-string` / `--set-file` / `--set-json` (later wins).
3. Validates the merged values against `values.schema.json` if the package
   ships one; a schema violation stops the render.
4. Renders every template, substituting each `${...}` expression. With
   `--show-only`, only the named template files are rendered.
5. Prepends `crds/` with `--include-crds`, pipes the result through
   `--post-renderer`, and runs a server-side dry-run with `--validate`.
6. Writes the joined YAML (documents separated by `---`) to stdout.

## Usage

```
keramos template <package-path> [flags]
```

The package path is required. The release name is not a positional argument —
it defaults to the package name and is overridden with `--release-name`.

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-f, --values` | stringArray | — | read a values file and merge it over the defaults (repeatable, later wins) |
| `--set` | stringArray | — | override one `key=value`, type-inferred (repeatable) |
| `--set-string` | stringArray | — | override `key=value` forcing a string, so `1.27` stays a string |
| `--set-file` | stringArray | — | set `key=path`; the file's contents become the value (repeatable) |
| `--set-json` | stringArray | — | set `key=<json>`; the value is parsed as a JSON literal (repeatable) |
| `--profile` | string | — | apply the `profiles/<name>` overlay before rendering |
| `-s, --show-only` | stringArray | — | render only these template files (by name or basename); everything else is dropped |
| `--release-name` | string | (package name) | set `${release.name}` in the render instead of the package name |
| `--is-upgrade` | — | false | render with `${release.isUpgrade}` true and `isInstall` false, to exercise upgrade-only branches |
| `--validate` | — | false | after rendering, send a server-side dry-run to the cluster and fail on rejection |
| `--include-crds` | — | false | prepend the manifests in `crds/` to the output |
| `--api-versions` | stringArray | — | mark an API version as available for `${capabilities...}` checks (repeatable) |
| `--kube-version` | string | — | override the Kubernetes version reported to capability checks |
| `--name-template` | string | — | alias for `--release-name` (currently equivalent) |
| `--post-renderer` | string | — | pipe the rendered manifests through this command's stdin and use its stdout |
| `--env` | string | — | apply the environment declared under `environments:` in `keramos.yaml` |

## Persistent flags inherited from `keramos`

Consulted only with `--validate`, the sole step that reaches a cluster.

| Flag | Type | Description |
|---|---|---|
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |
| `--debug` | — | enable debug output |

## Worked example

**INPUT — the package `./web`.** Two files. `values.yaml` holds the inputs:

```yaml
# web/values.yaml
name: web
replicas: 2
image:
  repository: nginx
  tag: "1.27"
```

`templates/deployment.yaml` reads those values through `${...}` expressions:

```yaml
# web/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: "${values.name}"
spec:
  replicas: ${values.replicas}
  template:
    spec:
      containers:
        - name: web
          image: "${values.image.repository}:${values.image.tag}"
```

**Run it:**

```sh
keramos template ./web
```

**OUTPUT:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 2
  template:
    spec:
      containers:
        - image: nginx:1.27
          name: web
```

**Tracing every value from input to output:**

| Output line | Expression | Value it read |
|---|---|---|
| `name: web` | `${values.name}` | `name: web` |
| `replicas: 2` | `${values.replicas}` | `replicas: 2` |
| `image: nginx:1.27` | `${values.image.repository}:${values.image.tag}` | `repository: nginx` + `tag: "1.27"` |

Override an input on the command line and only the matching output line moves —
`--set` wins over the file default:

```sh
keramos template ./web --set replicas=5
```

```yaml
  replicas: 5
```

## See also

- [`lint`](lint.md) — validate a package without printing manifests
- [`plan`](plan.md) — render and diff against the recorded state
- [`diff`](diff.md) — compare two renders (no cluster)
- [`install`](install.md) — render and apply as a tracked release
{% endraw %}
