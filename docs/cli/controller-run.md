# keramos controller run

## Synopsis

`keramos controller run` starts the in-cluster reconciler in the foreground. The reconciler lists every `KeramosRelease` CR (cluster-wide or in a single namespace), compares each spec against the corresponding keramos release record, and applies the difference. It re-checks the world on a configurable interval. The process is meant to run inside a cluster pod or as a sidecar daemon under systemd; killing it stops reconciliation but leaves any already-installed releases alone.

## When to use it

Run keramos as an operator when you want declarative `KeramosRelease` CRs to drive your installs and upgrades — typical in GitOps pipelines where Argo CD or Flux applies the CRs and the keramos controller turns them into actual cluster state. For one-shot operator commands run from a CLI machine, `keramos install` / `keramos upgrade` are simpler.

## What happens when you run it

1. Keramos starts a watcher on `KeramosRelease` resources in `--watch-namespace` (empty = all namespaces).
2. Every `--interval` it lists known CRs, compares each spec to the stored release record, and reconciles any divergence by calling the same code paths as `keramos install` / `keramos upgrade`.
3. Each CR-supplied package path is resolved relative to `--package-root`; paths escaping that root are rejected (anti-traversal protection).
4. Status is reported back to the CR via standard k8s `.status` conditions.
5. The process runs until interrupted (`SIGINT` / `SIGTERM`).

## Usage

```
keramos controller run [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for run |
| `--interval` | duration | 30s | reconcile interval |
| `--package-root` | string | /var/lib/keramos/packages | directory under which CR-supplied package paths must resolve (anti-traversal root) |
| `--watch-namespace` | string | "" | namespace to watch (empty = all) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Watch every namespace, default reconcile interval, default package root:

```sh
keramos controller run
```

Restrict the watch to a single tenant namespace:

```sh
keramos controller run --watch-namespace tenant-a
```

Tighten the reconcile loop for a development cluster:

```sh
keramos controller run --interval 5s --debug
```

Use a non-default package root (e.g. when running outside a pod with packages mounted at a custom path):

```sh
keramos controller run --package-root /opt/keramos/charts
```

## See also

- [`controller`](controller.md)
- [`controller install-crd`](controller-install-crd.md) — install the CRD first
- [`controller crd`](controller-crd.md)
