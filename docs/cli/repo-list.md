# keramos repo list

## Synopsis

`keramos repo list` prints every HTTP package repository currently registered with keramos on this machine: name, URL, and a brief flag indicating whether credentials or TLS material are stored. The data comes from `~/.config/keramos/repositories.yaml`.

## When to use it

Use to inventory configured repos, find a repo's URL for `keramos pull --repo`, or verify a `keramos repo add` succeeded.

## What happens when you run it

1. Reads `~/.config/keramos/repositories.yaml`.
2. Prints in the requested output format (table by default).
3. No cluster contact, no network.

## Usage

```
keramos repo list [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for list |
| `-o, --output` | string | table | output format: table, json, yaml |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Default tabular view:

```sh
keramos repo list
```

JSON for scripting:

```sh
keramos repo list -o json | jq '.[] | select(.name == "my-charts") | .url'
```

YAML for diffing across machines:

```sh
keramos repo list -o yaml > /tmp/repos-machine-A.yaml
```

## See also

- [`repo`](repo.md)
- [`repo add`](repo-add.md)
- [`repo update`](repo-update.md)
- [`repo remove`](repo-remove.md)
