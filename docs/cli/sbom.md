# keramos sbom

## Synopsis

`keramos sbom` emits a CycloneDX 1.5 JSON SBOM for a release. The SBOM lists the release's package metadata, its declared layers, and every container image referenced in the rendered manifest with the SHA-256 digests where computable. Output goes to stdout — redirect to a file for archival or feed it directly into `cosign attest`, `grype`, `trivy`, or Dependency Track.

## When to use it

Use as part of supply-chain documentation: every install / upgrade gets an SBOM emitted into the artefact store, pinned to the revision number. With `cosign attest --predicate <sbom>` the SBOM becomes a verifiable attestation alongside the OCI image.

## What happens when you run it

1. Reads the release record at `<release-name>` (current revision unless `--revision` is set).
2. Parses the rendered manifest to discover every `image:` reference.
3. Emits a CycloneDX 1.5 JSON document containing the package metadata and the image bill-of-materials.
4. Prints to stdout. Cluster contact is read-only.

## Usage

```
keramos sbom <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for sbom |
| `--revision` | int | 0 | release revision (0 = latest) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Emit the SBOM for the current revision:

```sh
keramos sbom hello -n prod > hello.cdx.json
```

SBOM for a historical revision:

```sh
keramos sbom hello --revision 3 -n prod > hello-rev3.cdx.json
```

Pipe directly into a vulnerability scanner:

```sh
keramos sbom hello -n prod | grype sbom:-
```

## See also

- [`audit`](audit.md)
- [`get manifest`](get-manifest.md)
- [Signing guide](../guides/signing.md)
