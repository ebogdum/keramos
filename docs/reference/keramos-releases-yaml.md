---
title: "keramos-releases.yaml"
parent: "Reference"
---
{% raw %}
# keramos-releases.yaml

Declares a set of separate releases and the order they must be applied in. The
`keramos releases` commands read it to install, upgrade, uninstall, plan, or check
the status of many releases — possibly of unrelated packages from different
sources — as one dependency-ordered operation. Unlike `layers` in `keramos.yaml`
(which merge into one release), every entry here is its own release.

## Minimal example

```yaml
releases:
  - name: cert-manager
    package: oci://registry.example.com/pkgs/cert-manager
  - name: ingress
    package: ./charts/ingress
    dependsOn: [cert-manager]
```

`keramos releases install` then installs `cert-manager` first, then `ingress`.

## Fields

The file has a single top-level key.

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `releases` | list | yes | — | The releases to manage. Each entry is one release; see below. |

### Release entry fields

| Field | Type | Required | Default | Description |
|---|---|---|---|---|
| `name` | string | yes | — | Release name, unique within the file. Used as the install target and referenced by other entries' `dependsOn`. |
| `package` | string | yes | — | Where the package comes from. Same forms as a layer source: local path, `oci://` URI, `https://` chart URL, or `git::` URL. |
| `namespace` | string | no | CLI `-n` | Namespace to install into. Falls back to the command's `--namespace`. |
| `profile` | string | no | — | Profile to activate when rendering this release. |
| `values` | string list | no | — | Value files to merge, passed through as repeated `-f`. Relative paths resolve against the current working directory, not the spec file. |
| `set` | string list | no | — | `--set`-style `key=value` overrides applied on top of `values`. |
| `dependsOn` | string list | no | — | Names of other entries in this file that must be applied before this one. References outside the file are rejected; cycles are reported as errors. |

Releases are applied in topological order of `dependsOn` (reverse order for
`uninstall`). Entries with no dependency relationship keep their declared order.
Every release installs with atomic rollback on failure.

## Full example

```yaml
# Platform bootstrap: four releases from four sources, strictly ordered.
releases:
  - name: cert-manager
    package: oci://registry.example.com/pkgs/cert-manager
    namespace: cert-manager
    set:
      - installCRDs=true

  - name: external-dns
    package: git::https://github.com/example/charts.git//external-dns
    namespace: dns

  - name: ingress-nginx
    package: ./local/ingress-nginx
    namespace: ingress
    profile: production
    values:
      - ./values/ingress-prod.yaml
    dependsOn: [cert-manager]         # needs cert-manager's webhooks first

  - name: monitoring
    package: oci://registry.example.com/pkgs/kube-prometheus
    namespace: monitoring
    dependsOn: [ingress-nginx, external-dns]
```

`keramos releases plan` prints the resulting order:

```
1. cert-manager (oci://…/cert-manager) ns=cert-manager
2. external-dns (git::…/external-dns) ns=dns
3. ingress-nginx (./local/ingress-nginx) ns=ingress
4. monitoring (oci://…/kube-prometheus) ns=monitoring
```

`keramos releases install` applies them in that order; `keramos releases uninstall`
removes them in reverse.

## See also

- [`keramos releases plan`](../cli/releases-plan.md) — preview the order.
- [`keramos releases install`](../cli/releases-install.md) / [`upgrade`](../cli/releases-upgrade.md) / [`uninstall`](../cli/releases-uninstall.md).
- [`keramos releases status`](../cli/releases-status.md) — show each release's revision.
- [keramos-workspace.yaml](keramos-workspace-yaml.md) — related packages from one repo.
- [Releases guide](../guides/releases.md).
{% endraw %}
