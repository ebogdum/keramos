---
title: "keramos pull"
parent: "CLI"
---
{% raw %}
# keramos pull

Download a package from an OCI registry or an HTTP repository, and optionally
unpack it.

## When to use it

- To vendor a copy of an upstream package into your repo, or stage one for an
  air-gapped install.
- To inspect a package's contents offline before installing — pair with
  `--untar`.
- To fetch by chart name from an HTTP repository (`--repo`) with SemVer version
  selection, which the OCI-only [`keramos registry pull`](registry-pull.md) does
  not do.

## What happens

1. Reads `<chart>`. If it starts with `oci://`, keramos pulls from that registry
   reference (`--version` is appended as the tag when set).
2. Otherwise `<chart>` is a name looked up in `<repo>/index.yaml`, so `--repo`
   is required. Keramos picks the version matching `--version`, or the latest when
   it is unset.
3. Downloads the archive into `--destination` (default the current directory).
   With `--prov`, the `.prov` provenance sidecar is fetched alongside.
4. With `--verify`, the provenance signature is checked before the archive is
   kept; a bad signature aborts the pull.
5. With `--untar`, the archive is extracted into `--untardir`
   (default `<destination>/<chart>`).
6. Prints the path of the archive, or the extraction directory when unpacked.

## Usage

```
keramos pull <chart>
```

`<chart>` is either an `oci://…` reference or a chart name used with `--repo`.

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--version` | string | "" | Version to pull. Unset takes the latest available. |
| `--destination` | string | `.` | Directory the archive is saved in. |
| `--repo` | string | "" | HTTP repository URL containing `index.yaml`. Required for non-OCI chart names. |
| `--untar` | bool | false | Extract the archive after downloading. |
| `--untardir` | string | "" | Directory to extract into. Default `<destination>/<chart>`. |
| `--prov` | bool | false | Also download the `.prov` provenance sidecar. |
| `--verify` | bool | false | Verify the provenance signature before saving; a bad signature aborts. |
| `--ca-file` | string | "" | CA bundle to trust for an HTTPS repository. |
| `--cert-file` | string | "" | Client certificate for mutual-TLS to the repository. |
| `--key-file` | string | "" | Client key paired with `--cert-file`. |

Global flags are inherited from `keramos`.

## Worked example

You want version 1.2.3 of `my-app` from an HTTP repository, unpacked so you can
read it, then installed.

**INPUT:**

```sh
keramos pull my-app --repo https://charts.example.com --version 1.2.3 \
  -d ./pulled --untar
```

**OUTPUT:**

```
Pulled and extracted: ./pulled/my-app
```

**RESULT:** the archive is fetched and expanded, leaving a package directory at
`./pulled/my-app` that you can inspect or install:

```sh
keramos install hello ./pulled/my-app -n staging --create-namespace
```

Pulling an OCI reference instead writes the archive and names it:

```sh
keramos pull oci://ghcr.io/example/charts/my-app --version 1.2.3 --destination ./pulled
```
```
Pulled oci://ghcr.io/example/charts/my-app:1.2.3 to ./pulled/my-app-1.2.3.keramos.tgz
```

## See also

- [`registry pull`](registry-pull.md) — OCI-only pull with cosign verification
- [`install`](install.md) — install the pulled package
- [`package`](package.md) — build an archive
- [`login`](login.md) — credentials for a private source
{% endraw %}
