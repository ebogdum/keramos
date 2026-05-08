# keramos get notes

## Synopsis

`keramos get notes` prints the rendered notes that keramos stored at install / upgrade time. Notes come from a package's `notes.yaml` (templated like everything else), and typically include access URLs, follow-up commands, credentials lookup recipes, and any other post-install instructions the package author wants the operator to see. They were displayed once at install time; this command re-prints them on demand.

## When to use it

Use to re-display the post-install message after the original install output has scrolled off the terminal, to look up an access URL months later, or to fetch the notes for a historical revision (`--revision`) when investigating "what did this used to say?".

## What happens when you run it

1. Reads the release-storage Secret for `<release-name>` at the requested revision.
2. Extracts the `notes` field — the template body resolved against the merged values at install time.
3. Prints it: `raw` is the human-readable string; `yaml` / `json` wraps it in a structured envelope.

## Usage

```
keramos get notes <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for notes |
| `-o, --output` | string | raw | output format: raw, json, yaml |
| `--revision` | int | 0 | get notes from a specific revision (0 = current) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Re-display notes for the current revision:

```sh
keramos get notes hello -n prod
```

The notes shown at revision 2 (e.g. before today's upgrade rewrote them):

```sh
keramos get notes hello --revision 2 -n prod
```

JSON envelope, useful when piping into a tool that expects structured output:

```sh
keramos get notes hello -n prod -o json
```

## See also

- [`get`](get.md)
- [`get all`](get-all.md)
- [Package anatomy: notes.yaml](../guides/packages.md)
