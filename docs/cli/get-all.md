# keramos get all

## Synopsis

`keramos get all` prints the entire stored release record for a release: metadata (name, namespace, revision, status, package, timestamps, labels), the merged values used at install time, the rendered Kubernetes manifest, the rendered hook manifests with their last-run results, and the rendered notes output. The output is one self-contained document, which is convenient for archiving, triage, or feeding into another process via `--template`.

## When to use it

Use when you're triaging a problematic release and want every relevant artefact in one shot instead of running `get manifest`, `get values`, `get hooks`, and `get notes` separately. Also handy for support-bundle generation: pipe the YAML to a file and attach it to an incident ticket.

## What happens when you run it

1. Connects to the cluster using the active kubeconfig.
2. Reads the release-storage Secret (or ConfigMap) for `<release-name>` in the active namespace at the requested revision (defaults to current).
3. Decodes the gzipped + base64-encoded payload in-memory.
4. Renders the resulting struct as YAML, JSON, or via a Go `text/template`.

## Usage

```
keramos get all <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for all |
| `-o, --output` | string | yaml | output format: json, yaml |
| `--revision` | int | 0 | get full record from a specific revision (0 = current) |
| `--template` | string | "" | Go text/template applied to the release record (overrides --output) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Get the full record for the current revision as YAML:

```sh
keramos get all hello -n prod
```

Get the record for revision 3 specifically (what was installed two upgrades ago):

```sh
keramos get all hello --revision 3 -n prod
```

Emit JSON for piping into `jq`:

```sh
keramos get all hello -n prod -o json | jq '.values'
```

Format the output with a Go text/template — here, just the package name and revision:

```sh
keramos get all hello -n prod --template '{{ .package.name }}:{{ .revision }}'
```

## See also

- [`get`](get.md)
- [`get manifest`](get-manifest.md)
- [`get values`](get-values.md)
- [`get hooks`](get-hooks.md)
- [`get metadata`](get-metadata.md)
- [`get notes`](get-notes.md)
- [`history`](history.md)
- [`audit`](audit.md)
