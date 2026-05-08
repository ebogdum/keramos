# Capabilities

The `capabilities` root namespace exposes information about the cluster the template is rendering against. The two practically-useful pieces of information are the cluster's Kubernetes version (used to gate version-specific fields) and the `lookup` function (used to read live resources).

## `capabilities.kubeVersion`

Keramos stores the Kubernetes version the template is rendering against under `capabilities.kubeVersion`. The keys preserve their Go-struct casing (capitalised), unlike the lowercase root namespaces (`values`, `release`, `package`, `capabilities`).

| Path | Type | Description |
|---|---|---|
| `capabilities.kubeVersion.Version` | string | full version string, e.g. `"1.28.5"`. Always populated. |
| `capabilities.kubeVersion.GitVersion` | string | server's reported `gitVersion`, e.g. `"v1.28.5+k3s1"`. |
| `capabilities.kubeVersion.Major` | string | major version, e.g. `"1"`. May be empty when `--kube-version` is used without a full server probe. |
| `capabilities.kubeVersion.Minor` | string | minor version, e.g. `"28"`. May be empty in offline `keramos template` mode. |

When running against a real cluster (`keramos install`/`keramos upgrade`/`keramos diff`), all four fields are populated from the API server's `/version` endpoint. When running offline (`keramos template`), only `Version` and `GitVersion` are populated from the `--kube-version` flag (or a default of `1.28.0` if not specified).

### Branching on the Kubernetes version

Use `semverCompare` with the version on the left of the pipe and the constraint as the literal argument:

```yaml
$if: ${capabilities.kubeVersion.Version | semverCompare '>=1.27.0-0'}
spec:
  template:
    spec:
      schedulingGates:
        - name: hold
```

The pipeline form is required because keramos function arguments are parsed as literal tokens — only the value to the left of `|` is a path lookup. So `${semverCompare '>=1.27.0-0' capabilities.kubeVersion.Version}` does **not** work; the second argument would be the literal string `"capabilities.kubeVersion.Version"` rather than the resolved version.

### Detecting Pod-spec fields available in a given version

```yaml
spec:
  template:
    spec:
      containers:
        - name: app
          image: ${values.image}
      $if: ${capabilities.kubeVersion.Version | semverCompare '>=1.26.0-0'}
      hostUsers: false                      # 1.26+ user-namespace field
```

### Refusing to render against unsupported clusters

```yaml
$if: ${capabilities.kubeVersion.Version | semverCompare '<1.25.0-0'}
data: ${'this package requires Kubernetes 1.25 or newer' | fail}
```

`fail` aborts the render with the supplied message.

## `capabilities.apiVersions`

The set of group/versions the cluster has registered. Keramos populates it from the API server's discovery during cluster-bound operations and from the `--api-versions` flag when running offline.

The map is exposed at the path `capabilities.apiVersions` and yields a `key: bool` map when serialised (e.g. via `toYaml`):

```yaml
apps/v1: true
networking.k8s.io/v1: true
monitoring.coreos.com/v1: true
```

### Limitation: keys with slashes are not directly addressable

Keramos's path engine cannot traverse map keys that contain `/` or `.` mid-segment, and there is no method-call syntax. So neither of these compile:

```yaml
${capabilities.apiVersions.apps/v1}            # ✗ slash splits the path
${capabilities.apiVersions.has 'apps/v1'}      # ✗ no method call
${capabilities.apiVersions['apps/v1']}         # ✗ bracket form not supported
```

This means there is no clean template-time check for "is `monitoring.coreos.com/v1` registered?" today. The practical patterns are:

1. **Trust the operator** — assume the apiVersion is present if the user supplied it via `--api-versions` (CI gates) or because the package's `kubeVersion:` constraint covers it.
2. **Render the resource unconditionally** and let the API server's apply error if the kind is missing. Combined with `--cleanup-on-fail`, the failure rolls back cleanly.
3. **Gate at install time, not template time** — a `pre-install` hook can run `kubectl get crd ... || exit 1` to verify a CRD is present before the install proceeds.

For most version-driven branching, `kubeVersion` + `semverCompare` is sufficient (the API server's GVK availability tracks the version closely).

## `lookup` — read-side cluster state

The `lookup` function pulls a live resource into the render. Signature:

```
lookup(apiVersion, kind, namespace, name)
```

All four arguments are literals. Returns the resource as a map, `nil` if missing, or a list (`{items: [...]}`) when `name` is empty.

**Input — read a ConfigMap that may not exist yet:**

```yaml
$if: ${lookup 'v1' 'ConfigMap' 'kube-system' 'cluster-info'}
data:
  publicCAFile: ${(lookup 'v1' 'ConfigMap' 'kube-system' 'cluster-info').data.kubeconfig | quote}
```

When the ConfigMap doesn't exist, `lookup` returns nil; `$if` evaluates falsy; the document is dropped.

**Input — list pods in a namespace:**

```yaml
$if: ${lookup 'v1' 'Pod' 'default' ''}
data:
  count: ${(lookup 'v1' 'Pod' 'default' '').items | len}
```

### Caching

Keramos caches `lookup` results within a single render so repeated lookups for the same (apiVersion, kind, namespace, name) tuple don't re-hit the API server. Across renders, lookups re-run.

## Offline mode behaviour

`keramos template` runs without kubeconfig. In that mode:

- `capabilities.kubeVersion.Version` returns the value of `--kube-version <ver>` if supplied, else `"1.28.0"`.
- `capabilities.kubeVersion.Major`/`.Minor` are typically empty (only populated by a real server probe).
- `capabilities.apiVersions` contains exactly the entries supplied via `--api-versions` (repeatable). Without that flag, the map is empty.
- `lookup` returns `nil` for everything.

This means templates that depend on `lookup` for required data **will** render to empty/false branches under `keramos template`. To preview against a specific cluster's view, run `keramos diff` or `keramos plan` (both use the live API for capabilities and lookup) instead.
