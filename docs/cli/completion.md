# keramos completion

## Synopsis

`keramos completion` generates shell-completion scripts for `bash`, `zsh`, `fish`, or `powershell`. The generated script defines completion handlers; source it from your shell init or stage it into the shell's standard completion directory.

## When to use it

Use once per shell to enable tab completion of keramos commands and flags.

## Usage

```
keramos completion [bash|zsh|fish|powershell] [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for completion |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Bash:

```sh
keramos completion bash > /etc/bash_completion.d/keramos
```

Zsh:

```sh
keramos completion zsh > "${fpath[1]}/_keramos"
```

Fish:

```sh
keramos completion fish > ~/.config/fish/completions/keramos.fish
```
