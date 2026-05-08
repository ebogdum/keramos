# keramos lint

## Synopsis

`keramos lint` validates a package directory for correctness. Checks include: parsing `keramos.yaml`, parsing `values.yaml`, validating against `values.schema.json` if present, rendering every template, asserting the rendered output is valid YAML and recognised Kubernetes manifests, and a static set of best-practice checks (unpinned image tags, missing resource limits, hostPort use, etc.). Lint is the cheapest way to catch a bad commit before it reaches a cluster.

## When to use it

Run as a pre-commit and CI gate. Always lint before `keramos package` / `keramos publish`. Add `--strict` to treat warnings as errors when you want stricter quality gates.

## What happens when you run it

1. Reads the package at `<package-path>`.
2. Parses `keramos.yaml` and `values.yaml`.
3. Validates merged values (defaults + `-f` + `--set` + selected `--profile`) against `values.schema.json` if present.
4. Renders every template through the engine (errors on missing keys, unknown functions, malformed expressions).
5. Parses each rendered document as Kubernetes-shaped YAML.
6. Runs static best-practice checks.
7. Reports findings to stdout; exits 0 on success, non-zero on any error or (with `--strict`) any warning.

## Usage

```
keramos lint <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for lint |
| `--profile` | string | "" | profile name to apply |
| `--set` | stringArray | — | set key=value overrides (repeatable) |
| `--strict` | bool | false | treat warnings as errors |
| `-f, --values` | stringArray | — | values file overrides (repeatable) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Lint a local package directory:

```sh
keramos lint ./my-app
```

Lint with a non-default profile and an overrides file (catches profile-specific issues):

```sh
keramos lint ./my-app --profile prod -f overrides.prod.yaml
```

Strict CI mode — any warning fails the build:

```sh
keramos lint ./my-app --strict
```

Lint after every value override to confirm overrides don't break templates:

```sh
keramos lint ./my-app --set replicas=10 --set image.tag=1.5.0
```

## See also

- [`template`](template.md) — render to inspect output
- [`policy check`](policy-check.md) — additional gating with policy rules
- [Package anatomy guide](../guides/packages.md)
- [Schema validation guide](../guides/schema-validation.md)
