# keramos repo remove

Drop a repository from your list.

## When to use it

- When you no longer search or pull charts from a repository.
- When cleaning up a development machine.

## What happens

Keramos deletes the named entry from your repository list at
`~/.config/keramos/repositories.yaml` and prints a confirmation line. After this,
the repository no longer appears in [`keramos repo list`](repo-list.md) and its
charts are no longer returned by [`keramos search`](search.md). The repository
server itself is untouched — only this machine's view of it changes. Removing a
name that is not registered reports an error.

## Usage

```
keramos repo remove <name> [flags]
```

## Flags

Inherits the global flags.

## Worked example

```
$ keramos repo remove my-charts
"my-charts" has been removed from your repositories

$ keramos repo list
No repositories configured.
```

## See also

- [`repo add`](repo-add.md) — register a repository
- [`repo list`](repo-list.md) — see what is registered
- [`logout`](logout.md) — remove stored credentials for a host
