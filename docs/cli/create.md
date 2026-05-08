# keramos create

## Synopsis

`keramos create` scaffolds a new package in the named directory. The output is a small but complete keramos package — `keramos.yaml`, `values.yaml`, a `templates/` directory with a working Deployment + Service + ConfigMap, an `_helpers.yaml` partial for shared label snippets, and a `README.md`. The result is immediately renderable with `keramos template` and immediately installable with `keramos install`.

## When to use it

Use when starting a new package from scratch. For richer starting points — an operator-pattern package with CRD scaffolding, a batch-job pattern, or a stateful service template — use `keramos init <template>` instead. For migrating an existing Helm chart, use `keramos migrate`.

## What happens when you run it

1. Creates the directory `<name>` (errors if it already exists).
2. Writes `<name>/keramos.yaml` with `apiVersion: keramos/v1`, `name: <name>`, `version: 0.1.0`, and a brief description.
3. Writes `<name>/values.yaml` with conventional defaults (replicas, image, service, resources).
4. Writes `<name>/templates/deployment.yaml`, `service.yaml`, `configmap.yaml`, and `_helpers.yaml`.
5. Writes `<name>/README.md` with a one-line description and install instructions.

The scaffolded package is intentionally minimal — open the files and customise rather than treating them as fixed.

## Usage

```
keramos create <name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for create |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Scaffold a new package and enter the directory:

```sh
keramos create my-app
cd my-app
ls
# README.md  keramos.yaml  templates/  values.yaml
```

Render the scaffolded package straight away to confirm it's well-formed:

```sh
keramos create my-app
keramos template ./my-app
```

Install the scaffolded package into a `kind` cluster as a smoke test:

```sh
keramos create my-app
keramos install my-app ./my-app -n my-app --create-namespace
```

## See also

- [`init`](init.md) — scaffold from a richer named template
- [`migrate`](migrate.md) — convert a Helm chart into a keramos package
- [Package anatomy](../guides/packages.md) — what every file in a keramos package is for
- [Quickstart](../guides/quickstart.md) — full create → install → upgrade walkthrough
