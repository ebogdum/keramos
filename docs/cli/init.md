# keramos init

## Synopsis

`keramos init` scaffolds a package from one of the built-in templates. Each template is a curated starting point for a class of workload (`webapp`, `batch`, `operator`, `blank`).

## When to use it

Use when you know what kind of workload you're packaging and want sensible defaults for that shape. The `blank` template is similar to `keramos create` (minimum viable package); the others ship richer initial templates.

## Usage

```
keramos init <name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--dest` | string | "." | directory to create the package in (default: current directory) |
| `-h, --help` | — | — | help for init |
| `-t, --template` | string | "blank" | template name: webapp, batch, operator, blank |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Scaffold a webapp-style package:

```sh
keramos init webapp my-frontend
```

Scaffold a batch-job package:

```sh
keramos init batch nightly-pipeline
```

Scaffold an operator-style package (CRD + controller):

```sh
keramos init operator my-operator
```

## See also

- [`create`](create.md)
- [Package anatomy guide](../guides/packages.md)
