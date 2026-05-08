# keramos controller

## Synopsis

`keramos controller` reconciles `KeramosRelease` custom resources declared in the cluster. The controller watches `KeramosRelease` CRs, materialises each into a regular keramos release (install/upgrade/uninstall as the spec demands), and reports outcome on the CR's status. Suitable for GitOps deployments where the desired set of releases is itself stored in the cluster.

## When to use it

Use when running keramos as a Kubernetes-native operator. Most users will not run this — it is the building block behind GitOps engines that bridge a Git repo to keramos-managed releases.

## Usage

```
keramos controller [command]
```

## Subcommands

- [`keramos controller install-crd`](controller-install-crd.md) — Apply the KeramosRelease CRD to the cluster
- [`keramos controller run`](controller-run.md) — Run the KeramosRelease reconciler in the foreground

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for controller |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Install the KeramosRelease CRD:

```sh
keramos controller install-crd
```

Print the CRD definition:

```sh
keramos controller crd
```

Run the reconciliation loop:

```sh
keramos controller run
```

## See also

- [`keramos-releases.yaml` reference](../reference/keramos-releases-yaml.md)
