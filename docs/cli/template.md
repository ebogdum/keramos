# keramos template

## Synopsis

`keramos template` renders a package locally and prints the resulting Kubernetes manifest to stdout. By default, no cluster contact is made — templates that depend on `lookup` see `nil`, and capabilities default to a baseline Kubernetes version. With `--api-versions` and `--kube-version` you can simulate a specific cluster's view; with `--validate`, keramos does perform a server-side dry-run apply at the end to check the manifest against the live API server's defaulters and admission policies.

## When to use it

Use to inspect what `keramos install` / `upgrade` *would* apply, to feed the output into other tools (kubeval, kube-linter, conftest, OPA, sed/yq), or to render once and apply with raw `kubectl` (skipping keramos's release tracking — only do this for one-off troubleshooting). Pair with `--show-only` to render a single file.

## What happens when you run it

1. Reads `<package-path>`, resolves layers, merges values (defaults → layer values → environment → profile → `-f` files → `--set*` flags).
2. Validates merged values against `values.schema.json` if present.
3. Renders every template; `--show-only <file>` restricts to one named template.
4. With `--include-crds`, prepends the `crds/` content.
5. Optionally pipes through `--post-renderer`.
6. With `--validate`, server-side dry-runs the result for early feedback.
7. Prints to stdout.

## Usage

```
keramos template <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--api-versions` | stringArray | — | Kubernetes API version available for capability checks (repeatable) |
| `--env` | string | "" | environment name declared in `keramos.yaml`'s `environments:` section |
| `-h, --help` | bool | false | help for template |
| `--include-crds` | bool | false | include CRDs from `crds/` in the output |
| `--is-upgrade` | bool | false | render with `.Release.IsUpgrade = true` |
| `--kube-version` | string | "" | override Kubernetes version reported in capabilities |
| `--name-template` | string | "" | name-template (currently equivalent to `--release-name`) |
| `--post-renderer` | string | "" | command piped the rendered manifests on stdin |
| `--profile` | string | "" | profile name to apply |
| `--release-name` | string | "" | override release name (default: package name) |
| `--set` | stringArray | — | set key=value overrides (repeatable) |
| `--set-file` | stringArray | — | set key=path; value read from path (repeatable) |
| `--set-json` | stringArray | — | set key=<json>; value parsed as JSON (repeatable) |
| `--set-string` | stringArray | — | force string interpretation (repeatable) |
| `-s, --show-only` | stringArray | — | render only the named template file (repeatable) |
| `--validate` | bool | false | validate against the cluster after rendering (server-side dry-run) |
| `-f, --values` | stringArray | — | values file overrides (repeatable) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Render with the package's default values:

```sh
keramos template ./my-app
```

Render with overrides and capture to a file:

```sh
keramos template ./my-app -f overrides.yaml --set replicas=3 > rendered.yaml
```

Render simulating a specific Kubernetes version and an installed API:

```sh
keramos template ./my-app \
  --kube-version 1.28.0 \
  --api-versions networking.k8s.io/v1/Ingress \
  --api-versions monitoring.coreos.com/v1/ServiceMonitor
```

Render only one template file (useful for quick author-time iteration):

```sh
keramos template ./my-app -s templates/deployment.yaml
```

Render with --include-crds and validate against the cluster:

```sh
keramos template ./my-app --include-crds --validate
```

Render the prod-environment view of the package:

```sh
keramos template ./my-app --env prod
```

## See also

- [`debug`](debug.md)
- [`lint`](lint.md)
- [`install`](install.md)
- [Template expressions](../templates/expressions.md)
- [Control flow](../templates/control-flow.md)
