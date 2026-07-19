# keramos sbom

`keramos sbom` emits a CycloneDX 1.5 JSON software bill of materials for a
release — its package plus every container image it runs.

## When to use it

- To produce a supply-chain artifact for each deploy that `cosign attest`,
  Grype, Trivy, or Dependency Track can ingest.
- To answer "what images and package version is this release actually
  running?" for a security or compliance review.
- To diff what shipped between two revisions by generating an SBOM for each.

## What happens

1. You name a release. keramos reads its stored record — the latest revision, or
   the one you pass to `--revision`.
2. keramos walks the rendered manifest and collects every container image it
   references (in containers, init containers, and ephemeral containers).
3. It builds a CycloneDX 1.5 document: the release as the root component, its
   keramos package as a library component, and one container component per image.
4. The JSON document is printed to stdout. Redirect it to a file or pipe it
   into your scanner. Contact with the cluster is read-only.

## Usage

```
keramos sbom <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--revision` | int | `0` | build the SBOM for this revision; `0` uses the latest |

### Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Worked example

Write the SBOM for the current revision to a file:

```sh
keramos sbom web-api -n prod > web-api.cdx.json
```

The document written looks like this:

```json
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "serialNumber": "urn:uuid:4c1e9a2f-...",
  "version": 1,
  "metadata": {
    "timestamp": "2026-07-18T12:00:00Z",
    "tools": [
      { "vendor": "keramos", "name": "keramos", "version": "dev" }
    ],
    "component": {
      "type": "application",
      "bom-ref": "release/web-api",
      "name": "web-api",
      "version": "rev-3",
      "description": "public API service"
    }
  },
  "components": [
    {
      "type": "library",
      "bom-ref": "package/webapp@1.4.0",
      "name": "webapp",
      "version": "1.4.0",
      "purl": "pkg:keramos/webapp@1.4.0"
    },
    {
      "type": "container",
      "bom-ref": "image/registry.example.com/api:1.5.0",
      "name": "api",
      "version": "1.5.0",
      "purl": "pkg:oci/api@1.5.0"
    }
  ]
}
```

Pipe a historical revision straight into a vulnerability scanner:

```sh
keramos sbom web-api --revision 2 -n prod | grype sbom:-
```

## See also

- [`get manifest`](get-manifest.md) — the rendered manifest the images come from
- [`audit`](audit.md) — the change trail for the same release
- [`scan`](scan.md) — refactor packages, not scan for vulnerabilities
