# keramos controller crd

## Synopsis

`keramos controller crd` prints the YAML for the `KeramosRelease` CustomResourceDefinition to stdout. The output is the canonical CRD definition that the in-cluster controller reconciles. It is the same YAML that `keramos controller install-crd` would apply, only printed instead of installed.

## When to use it

Use when you want to inspect the CRD before installing it, commit it to a GitOps repository, or pipe it through `kubectl apply -f -` from a machine that does not have keramos locally permitted to write to the cluster.

## What happens when you run it

1. Keramos emits the embedded CRD definition (group `keramos.dev`, kind `KeramosRelease`) to stdout.
2. No cluster contact, no file changes, no side effects beyond writing to stdout.

## Usage

```
keramos controller crd [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for crd |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Print the CRD to stdout for inspection:

```sh
keramos controller crd
```

Save to a file for committing into a GitOps repo:

```sh
keramos controller crd > crds/keramosrelease.yaml
```

Apply with `kubectl` from a different machine:

```sh
keramos controller crd | kubectl apply -f -
```

## See also

- [`controller`](controller.md)
- [`controller install-crd`](controller-install-crd.md)
- [`controller run`](controller-run.md)
