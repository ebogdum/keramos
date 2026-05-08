# keramos repo index

## Synopsis

`keramos repo index` generates an `index.yaml` for a directory of packaged `.keramos.tgz` archives. The index is the catalogue consumers fetch at a repository's root URL — it lists every package version with its download URL, SHA-256 digest, and (optionally) provenance signature link. With `--merge`, an existing `index.yaml` is preserved and only new entries are added; without it, the index is regenerated from scratch.

## When to use it

Use during repository publication — every time you add a new packaged version to the directory, regenerate (or merge into) the index. With `--sign`, the resulting `index.yaml.prov` lets consumers verify the catalogue itself, not just individual package archives.

## What happens when you run it

1. Walks `<dir>` for `*.keramos.tgz` archives.
2. For each archive, extracts metadata (name, version) from the embedded `keramos.yaml` and computes the SHA-256 digest.
3. With `--merge`, reads the existing `index.yaml` and preserves entries; without it, starts fresh.
4. With `--url`, generates per-version download URLs as `<url>/<archive>`.
5. Writes `<dir>/index.yaml`. With `--sign <key>`, also writes `<dir>/index.yaml.prov`.

## Usage

```
keramos repo index <dir> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for index |
| `--merge` | bool | false | merge with existing `index.yaml` instead of regenerating |
| `--sign` | string | "" | private key path to sign the index (produces `index.yaml.prov`) |
| `--url` | string | "" | base URL for download URLs in the index |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Generate a fresh index for a build directory:

```sh
keramos repo index ./build --url https://charts.example.com
```

Merge a new package into the existing index without regenerating from scratch:

```sh
keramos repo index ./build --url https://charts.example.com --merge
```

Generate and sign the index:

```sh
keramos repo index ./build --url https://charts.example.com --sign /path/to/repo-key.asc
```

End-to-end publication: package, index, then upload to S3:

```sh
keramos package    ./my-app -d ./build
keramos repo index ./build --url https://charts.example.com --merge
aws s3 sync     ./build s3://charts-bucket/
```

## See also

- [`repo`](repo.md)
- [`package`](package.md)
- [`publish`](publish.md)
- [Repositories guide](../guides/repositories.md)
