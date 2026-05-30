# keramos policy

## Synopsis

`keramos policy` evaluates package-defined policy rules (under `policies/`) against the rendered manifest. Rules are Keramos policy YAML (declarative match-and-require).

## When to use it

Use as a CI gate to enforce organisation-wide rules: "every Pod sets runAsNonRoot", "every Service has a selector", "no hostNetwork", etc. Policies live with the package so they ship together.

## Usage

```
keramos policy [command]
```

## Subcommands

- [`keramos policy list`](policy-list.md) — List policy rules declared in the package

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for policy |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Evaluate policies:

```sh
keramos policy check ./my-app
```

List declared policies:

```sh
keramos policy list ./my-app
```

## See also

- [`lint`](lint.md)
