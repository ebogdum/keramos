# keramos policy list

## Synopsis

`keramos policy list` enumerates every policy rule declared under `<package-path>/policies/` without evaluating them. The output names each rule, its file of origin (`*.yaml` or `*.rego`), and a brief description so you can see at a glance what gates a package ships with.

## When to use it

Use to audit a package's policy surface — particularly when adopting an upstream package, you may want to know what rules it self-imposes before consuming it. For actually running the policies against a manifest, use `keramos policy check`.

## What happens when you run it

1. Loads every `.yaml` and `.rego` file under `<package-path>/policies/`.
2. Parses each for declared rule names and descriptions.
3. Prints a tabular view to stdout.
4. No cluster contact, no manifest evaluation.

## Usage

```
keramos policy list <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for list |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

List policies declared by a package:

```sh
keramos policy list ./my-app
```

Run from inside the package directory:

```sh
cd ./my-app && keramos policy list .
```

Combine with `policy check` to confirm what would be evaluated:

```sh
keramos policy list  ./my-app
keramos template     hello ./my-app | keramos policy check ./my-app
```

## See also

- [`policy`](policy.md)
- [`policy check`](policy-check.md)
- [Package anatomy: policies](../guides/packages.md)
