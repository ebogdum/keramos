# keramos get manifest

## Synopsis

`keramos get manifest` prints the rendered Kubernetes manifest keramos stored at install/upgrade time for a release. The output is the exact, fully-resolved YAML that keramos applied to the cluster at the named revision — every `${...}` expression has been evaluated, every layer has been composed, every value has been merged. This is the source of truth for what keramos thinks should be running.

## When to use it

Use to inspect what keramos actually applied (versus what's currently in the cluster, which may have drifted), to compare two revisions of a release with `diff`, or to feed the manifest into another tool like `kubeval`, `kube-linter`, or `kubectl apply`. The `--revision` flag lets you pull a historical revision's manifest, useful for "what was deployed last Tuesday" forensics.

## What happens when you run it

1. Connects to the cluster using the active kubeconfig.
2. Reads the release-storage Secret for `<release-name>` at the requested revision.
3. Decodes the gzipped + base64 payload in-memory and extracts the `manifest` field.
4. Prints it: raw YAML stream by default, or wrapped in a JSON / YAML envelope with `-o json` / `-o yaml`.

## Usage

```
keramos get manifest <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for manifest |
| `-o, --output` | string | raw | output format: raw, json, yaml |
| `--revision` | int | 0 | get manifest from a specific revision (0 = current) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

The current revision's stored manifest:

```sh
keramos get manifest hello -n prod
```

Capture revision 3's manifest to a file for offline diffing:

```sh
keramos get manifest hello --revision 3 -n prod > rev3.yaml
```

Diff what keramos stored against what's currently live in the cluster (catches drift):

```sh
keramos get manifest hello -n prod | kubectl diff -f -
```

Compare two revisions of the same release:

```sh
diff <(keramos get manifest hello --revision 4 -n prod) \
     <(keramos get manifest hello --revision 5 -n prod)
```

## See also

- [`get`](get.md)
- [`get all`](get-all.md)
- [`drift`](drift.md) — automated stored-vs-live comparison
- [`reconcile`](reconcile.md) — re-apply the stored manifest
- [`history`](history.md)
