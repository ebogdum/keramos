# keramos multi-install

## Synopsis

`keramos multi-install` performs a single install/upgrade against multiple Kubernetes clusters in one invocation. Cluster references come from a manifest file or a kubeconfig list; each install runs in parallel and the result is reported per-cluster.

## When to use it

Use when the same release must be deployed to many clusters with the same parameters (per-environment platform releases, regionally-distributed apps). The list of target clusters is supplied as kubeconfig context names via `--to ctx1,ctx2,ctx3`.

## Usage

```
keramos multi-install <release-name> <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--atomic-cross-cluster` | — | — | if any cluster fails, roll back the rest |
| `--env` | string | — | environment name declared in keramos.yaml environments: |
| `-h, --help` | — | — | help for multi-install |
| `--no-wait` | — | — | do not wait for resources to be ready in any cluster |
| `--parallel` | int | 4 | concurrent installs across clusters |
| `--profile` | string | — | profile to apply |
| `--set` | stringArray | — | --set overrides |
| `--set-file` | stringArray | — | --set key=path; value read from path (repeatable) |
| `--set-json` | stringArray | — | --set key=<json> (repeatable) |
| `--set-string` | stringArray | — | --set forcing string interpretation (repeatable) |
| `--timeout` | duration | 5m0s | per-cluster wait timeout |
| `--to` | strings | — | kubeconfig contexts (comma-separated) |
| `-f, --values` | stringArray | — | values file overrides |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Install the same release into three clusters (comma-separated kubeconfig contexts):

```sh
keramos multi-install my-app ./my-app --to ctx-eu,ctx-us,ctx-ap -n prod
```

Atomic across-cluster mode — roll back every cluster if any one install fails:

```sh
keramos multi-install my-app ./my-app --to ctx-eu,ctx-us --atomic-cross-cluster -n prod
```

Higher concurrency for many clusters:

```sh
keramos multi-install my-app ./my-app --to ctx-1,ctx-2,ctx-3,ctx-4,ctx-5,ctx-6,ctx-7,ctx-8 --parallel 8 -n prod
```

## See also

- [`install`](install.md)
- [`upgrade`](upgrade.md)
