# keramos controller install-crd

## Synopsis

`keramos controller install-crd` applies the `KeramosRelease` CRD to the cluster. The CRD lets operators describe a release declaratively (`apiVersion: keramos.dev/v1, kind: KeramosRelease`) and have the in-cluster reconciler converge it. The CRD must exist before `keramos controller run` can watch and reconcile any `KeramosRelease` objects.

## When to use it

Run once per cluster as a prerequisite step before deploying the controller. Re-running is safe: the apply is idempotent and will leave an existing CRD untouched if the schema already matches, or upgrade it in place if it has changed.

## What happens when you run it

1. Keramos connects to the cluster using the active kubeconfig context.
2. Server-side applies the embedded `KeramosRelease` CRD definition (group `keramos.dev`).
3. Waits briefly for the CRD to reach `Established=true`.
4. Prints the applied object's name on success, or the API server error on failure.

## Usage

```
keramos controller install-crd [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for install-crd |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Install the CRD using the current kubeconfig context:

```sh
keramos controller install-crd
```

Install into an explicit cluster context:

```sh
keramos controller install-crd --kube-context prod-cluster
```

Verify the CRD landed:

```sh
keramos controller install-crd && kubectl get crd keramosreleases.keramos.dev
```

## See also

- [`controller`](controller.md)
- [`controller crd`](controller-crd.md) — print without applying
- [`controller run`](controller-run.md) — start the reconciler
