---
title: "Host and consume an HTTP repository"
nav_order: 9
parent: "Guides"
---
{% raw %}
# Host and consume an HTTP repository

A keramos repository is a directory served over HTTP(S) containing packaged
`*.keramos.tgz` archives plus an `index.yaml` that catalogues them. It is the
simplest distribution mechanism — any static HTTP server hosts one, including
GitHub Pages, S3 with a static index, or an internal nginx.

For OCI-registry distribution, see [OCI](oci.md). The two are complementary; a
project often publishes both.

## The shape of a repo

```
https://charts.example.com/
├── index.yaml
├── my-app-1.0.0.keramos.tgz
├── my-app-1.1.0.keramos.tgz
├── my-app-1.2.0.keramos.tgz
└── my-app-1.2.0.keramos.tgz.prov      # optional detached PGP signature
```

`index.yaml` lists every package, its versions, each archive's URL, and each
archive's SHA-256 digest.

## Produce a repo

### Package each release

```sh
keramos package ./my-app -d ./build
```

```
Successfully packaged to: ./build/my-app-1.2.3.keramos.tgz
```

`keramos package` reads the package's `keramos.yaml`, resolves layers, and writes a
single self-contained `<name>-<version>.keramos.tgz`. To sign at package time, add
`--sign` with a signer:

```sh
keramos package ./my-app -d ./build --sign --key author@example.com \
  --keyring ~/.gnupg/secring.gpg
```

```
Successfully packaged to: ./build/my-app-1.2.3.keramos.tgz
Signed: ./build/my-app-1.2.3.keramos.tgz.prov
```

This writes the archive plus a detached `.prov` signature. See
[Signing](signing.md) for the full story.

### Generate the index

```sh
keramos repo index ./build --url https://charts.example.com
```

```
Index generated at /home/you/build/index.yaml
```

`--url` is the base URL where the archives will be served; keramos writes
per-version absolute download URLs into the index and records each archive's
SHA-256. To add a new version without regenerating from scratch, use `--merge`;
to sign the index, add `--sign <private-key>` (writes `index.yaml.prov`):

```sh
keramos repo index ./build --url https://charts.example.com --merge --sign ./repo-key.asc
```

### Publish

The `./build/` directory is the entire repository — serve it from any static
HTTP host:

- **GitHub Pages** — commit `build/`, enable Pages, serve at
  `https://<user>.github.io/<repo>/`.
- **S3** — `aws s3 sync ./build s3://my-bucket/`, set `index.yaml`'s
  Content-Type to `application/yaml`.
- **nginx** — serve the directory; enable `autoindex` for a browsable view.

There is no special server; the repo is dumb static files.

## Consume a repo

Register the repo, refresh its index, then find and pull packages by name:

```sh
keramos repo add my-charts https://charts.example.com
```

```
"my-charts" has been added to your repositories
```

```sh
keramos repo update
```

```
...successfully got an update from "my-charts"
Update complete.
```

```sh
keramos repo list
```

```
NAME                 URL
my-charts            https://charts.example.com
```

```sh
keramos search repo my-app
```

`keramos repo add` records the name and URL in
`~/.config/keramos/repositories.yaml`; it does **not** fetch the index — that is
what `keramos repo update` does, caching each repo's `index.yaml` under
`~/.cache/keramos/indexes/` (30-minute TTL). Adding a name that already exists is
left untouched unless you pass `--force-update`.

### Pull a package

`keramos pull` fetches a named chart from a repo (`--repo`), with SemVer version
selection, and can unpack it. Note the flag is `--destination` — `keramos pull`
has no `-d` shorthand:

```sh
keramos pull my-app --repo https://charts.example.com --version "^1.2.0" \
  --destination ./pulled --untar
```

```
Pulled and extracted: ./pulled/my-app
```

Then install from the unpacked **directory** (`keramos install` takes a package
directory, not an archive or a URL):

```sh
keramos install my-app ./pulled/my-app -n default --create-namespace
```

## Authenticate to a private repo

Supply credentials when you add the repo:

```sh
keramos repo add private https://charts.example.com --username u --password p
```

For token-based auth (for example GitHub Pages behind a fine-grained token),
pass the token as the password — most providers accept it in place of a basic
password:

```sh
keramos repo add private https://charts.example.com \
  --username "$GITHUB_USER" --password "$GITHUB_TOKEN"
```

Credentials are stored in `~/.config/keramos/credentials.json`, keyed by host, and
reused automatically. To refresh a host's credential without re-adding the
repo, use `keramos login` — it is non-interactive and needs a credential flag
(`-u/--username` with `-p/--password`, or `--token`, or `--api-key`):

```sh
echo "$GITHUB_TOKEN" | keramos login charts.example.com -u "$GITHUB_USER" --password-stdin
```

```
Login succeeded for charts.example.com
```

`keramos logout charts.example.com` removes only the stored credential, not the
repo registration. To unregister the repo itself, `keramos repo remove private`.

## TLS options

`keramos repo add`, `keramos repo update`, and `keramos pull` honour:

- `--ca-file <path>` — trust an extra CA bundle for a self-signed server.
- `--cert-file <path>` / `--key-file <path>` — client certificate for mTLS.
- `--insecure-skip-tls-verify` — skip server-cert validation (`keramos repo add`
  only; do not use in production).

## Inspect a package

`keramos show` operates on an unpacked package **directory**:

```sh
keramos pull my-app --repo https://charts.example.com --version 1.2.3 \
  --destination ./pulled --untar
keramos show chart   ./pulled/my-app     # keramos.yaml metadata
keramos show values  ./pulled/my-app     # default values.yaml
keramos show readme  ./pulled/my-app     # README.md
keramos show crds    ./pulled/my-app     # declared CRDs
keramos show all     ./pulled/my-app     # everything in one document
```

To browse Artifact Hub without registering a repo:

```sh
keramos search hub my-app
keramos search hub my-app --endpoint https://artifacthub.io
```

`keramos search hub` queries the public Artifact Hub API directly.

## Verify provenance on pull

When an archive ships a `.prov` sidecar, verify the signature as you pull:

```sh
keramos pull my-app --repo https://charts.example.com --version 1.2.3 \
  --prov --verify --destination ./pulled
```

`--verify` fetches the `.prov` and checks it against your local keyring before
the archive is kept; a bad signature aborts the pull. See [Signing](signing.md).

## Troubleshooting

- **`digest mismatch`** — the archive on the server does not match the digest
  recorded in `index.yaml`. Common cause: an archive was re-published without
  regenerating the index (`keramos repo index ./build --url ...`).
- **`401 Unauthorized`** — no credential was sent, or it is wrong. Re-run
  `keramos login <host>`; check `~/.config/keramos/credentials.json`.
- **index not found** — the URL does not serve `index.yaml` at its root. Verify
  with `curl <url>/index.yaml`.
- **TLS handshake error** — the server certificate is not trusted; pass
  `--ca-file` for a self-signed setup.

## See also

- [`keramos repo add`](../cli/repo-add.md) · [`keramos repo index`](../cli/repo-index.md)
  · [`keramos repo update`](../cli/repo-update.md)
- [`keramos pull`](../cli/pull.md) — download a package from a repo or OCI
- [`keramos package`](../cli/package.md) — build the archives you host
- [OCI](oci.md) — registry-based distribution
- [Signing](signing.md) — sign and verify packages
{% endraw %}
