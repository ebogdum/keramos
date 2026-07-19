# keramos completion

`keramos completion` prints a shell completion script for `bash`, `zsh`, `fish`,
or `powershell` to stdout, so your shell can tab-complete keramos commands and
flags.

## When to use it

- Once per shell, to turn on tab completion for `keramos`.
- After upgrading keramos, to refresh the script if new commands or flags were
  added.

## What happens

1. You name a shell. keramos writes that shell's completion script to stdout.
2. You either source the output in the current session or save it into the
   shell's completion directory so it loads automatically on every new shell.
3. Nothing is installed or contacted — the command just prints a script.

## Usage

```
keramos completion [bash|zsh|fish|powershell]
```

## Flags

Inherits the global flags (`--debug`, `--kube-context`, `--kubeconfig`,
`-n/--namespace`); completion only prints a script, so they have no effect.

## Worked example

Pick the block for your shell.

**Bash** — load it for the current session, or install it permanently:

```sh
# current session only
source <(keramos completion bash)

# every new shell (Linux)
keramos completion bash | sudo tee /etc/bash_completion.d/keramos > /dev/null
```

**Zsh** — ensure completion is enabled, then drop the script on `fpath`:

```sh
# once, if not already in ~/.zshrc
echo "autoload -U compinit; compinit" >> ~/.zshrc

keramos completion zsh > "${fpath[1]}/_keramos"
```

**Fish** — save it into the fish completions directory:

```sh
keramos completion fish > ~/.config/fish/completions/keramos.fish
```

**PowerShell** — load it for the session, or append it to your profile:

```powershell
# current session only
keramos completion powershell | Out-String | Invoke-Expression

# every new session
keramos completion powershell >> $PROFILE
```

Start a new shell (or re-source your profile) and `keramos <Tab>` completes.

## See also

- [`env`](env.md) — the resolved paths and environment keramos is using
- [`version`](version.md) — confirm the build you generated completions from
