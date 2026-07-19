# keramos get all

`keramos get all` prints the entire stored release record in one document —
metadata, values, manifest, hooks, and notes together.

## When to use it

- You are triaging a release and want every artefact in one shot instead of
  running the individual `get` subcommands.
- You want to render one field out of the record with `--template`.

## What happens

It loads the stored release record for `<release>` (latest, or `--revision`) and
prints a single document combining its metadata (`name`, `namespace`,
`revision`, `status`, `package`, `info`, `labels`), `values`, `manifest`,
`hooks`, and `notes`. With `--template`, it runs a Go `text/template` against
that record instead of printing the whole thing; the record's keys are exposed
both lower-cased (`.package`) and capitalised (`.Package`) so template snippets
work either way.

## Flags

| Flag | Cause | Effect |
|---|---|---|
| `--revision <n>` | you name a stored revision | reads that revision instead of the latest |
| `-o, --output <fmt>` | you pass `json` or `yaml` (default `yaml`) | prints the record in that format |
| `--template <tmpl>` | you pass a Go text/template | renders the template against the record and overrides `--output` |

Inherits the global flags (`-n/--namespace`, `--kube-context`, `--kubeconfig`,
`--debug`).

## Usage

```
keramos get all <release> [flags]
```

## Worked example

Stored record for `hello` (revision 4):

```yaml
name: hello
namespace: prod
revision: 4
status: deployed
package:
  name: hello
  version: 1.5.0
values:
  replicaCount: 3
manifest: |
  apiVersion: apps/v1
  kind: Deployment
  # ...
hooks: []
notes: "Hello is deployed."
```

Full record as YAML:

```sh
keramos get all hello -n prod
```

```yaml
hooks: []
manifest: |
  apiVersion: apps/v1
  kind: Deployment
  # ...
name: hello
namespace: prod
notes: Hello is deployed.
package:
  name: hello
  version: 1.5.0
revision: 4
status: deployed
values:
  replicaCount: 3
```

Pull one field out with a template:

```sh
keramos get all hello -n prod --template '{{ .package.name }}:{{ .revision }}'
```

```
hello:4
```

## See also

- [`get`](get.md) — the parent command
- [`get manifest`](get-manifest.md) · [`get values`](get-values.md) · [`get hooks`](get-hooks.md) · [`get metadata`](get-metadata.md) · [`get notes`](get-notes.md) — the individual slices
