# keramos releases plan

## Synopsis

`keramos releases plan` reads `keramos-releases.yaml` and prints the topological order in which `keramos releases install` or `keramos releases upgrade` would process the declared releases. The output groups releases by Kahn level — releases at the same level have no inter-dependencies among themselves and are issued before the next level begins. No changes are made to the cluster; the command is read-only and offline.

## When to use it

Run before `install` / `upgrade` to confirm the dependency resolution is what you expect, especially after adding or modifying `dependsOn` entries. Useful in CI as a sanity step (`keramos releases plan` exits 0 when the file parses; non-zero on cycles or unknown references).

## What happens when you run it

1. Reads `--file` (default `keramos-releases.yaml`) from the current directory.
2. Builds the dependency graph from each entry's `dependsOn` list.
3. Runs Kahn's algorithm to produce a level grouping; cycles produce a clear error naming the involved releases.
4. Prints the resulting plan to stdout.
5. No cluster contact, no file writes.

## Usage

```
keramos releases plan [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--file` | string | keramos-releases.yaml | spec file path |
| `-h, --help` | bool | false | help for plan |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Print the plan for the default `keramos-releases.yaml` in the current directory:

```sh
keramos releases plan
```

Plan from a custom-named manifest:

```sh
keramos releases plan --file ./platform.releases.yaml
```

CI sanity check — fail the build if the plan can't be computed:

```sh
keramos releases plan --file ./keramos-releases.yaml || exit 1
```

## See also

- [`releases`](releases.md)
- [`releases install`](releases-install.md)
- [`releases upgrade`](releases-upgrade.md)
- [`keramos-releases.yaml` reference](../reference/keramos-releases-yaml.md)
- [Cross-release dependencies guide](../guides/releases.md)
