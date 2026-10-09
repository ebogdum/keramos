---
title: "Capabilities"
nav_order: 6
parent: "Templates"
---
{% raw %}
# Capabilities

The `capabilities` namespace exposes what the template knows about the cluster
it renders against. The two useful pieces are the Kubernetes version (for gating
version-specific fields) and the `lookup` function (for reading live resources).

## `capabilities.kubeVersion`

Keramos stores the cluster version under `capabilities.kubeVersion`. The keys keep
their Go-struct casing (capitalised), unlike the lowercase root namespaces.

| Path | Meaning |
|---|---|
| `capabilities.kubeVersion.Version` | full version string, e.g. `1.30.2` |
| `capabilities.kubeVersion.GitVersion` | server's reported git version |
| `capabilities.kubeVersion.Major` | major, e.g. `1` — live cluster only |
| `capabilities.kubeVersion.Minor` | minor, e.g. `30` — live cluster only |

Against a live cluster (`keramos install` / `upgrade` / `diff`), all four are
populated from the API server. Under `keramos template`, only `Version` and
`GitVersion` are set — both to the `--kube-version` value — and `Major` / `Minor`
are always null.

There is **no default** version. If you run `keramos template` without
`--kube-version`, `capabilities.kubeVersion` is unset and any version gate
errors. Pass the flag when a template branches on the version:

**Template:**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: caps
data:
  version: "${capabilities.kubeVersion.Version}"
  major:   "${capabilities.kubeVersion.Major}"
```

**`keramos template ./pkg --kube-version 1.30.2`:**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: caps
data:
  version: "1.30.2"
  major: null
```

### Branching on the version

Use `semverCompare` with the version on the **left** of the pipe and the
constraint as a literal argument. The pipeline form is required — a function
argument is a literal, not a path (see
[Expressions](expressions.md#arguments-are-literals-not-paths)), so only the
left side is resolved:

```yaml
$if: ${capabilities.kubeVersion.Version | semverCompare '>=1.27.0-0'}
spec:
  template:
    spec:
      schedulingGates:
        - name: hold
```

With `--kube-version 1.30.2`, `${... | semverCompare '>=1.27.0-0'}` renders
`true`.

### Refusing an unsupported cluster

```yaml
$if: ${capabilities.kubeVersion.Version | semverCompare '<1.25.0-0'}
data: ${'this package requires Kubernetes 1.25 or newer' | fail}
```

`fail` aborts the render with your message.

## `capabilities.apiVersions`

The set of group/versions the cluster has registered. Keramos fills it from API
discovery on live operations, and from `--api-versions` (repeatable) when
running offline. The value is a map keyed by group/version.

Ask whether a group/version is registered with the `Capabilities.APIVersions.Has`
function — the same name Helm charts use, so a migrated chart keeps working:

```yaml
apiVersion: ${Capabilities.APIVersions.Has "policy/v1" | ternary "policy/v1" "policy/v1beta1"}
kind: PodDisruptionBudget
```

```yaml
$if: ${Capabilities.APIVersions.Has "monitoring.coreos.com/v1"}
$then:
  apiVersion: monitoring.coreos.com/v1
  kind: ServiceMonitor
```

It returns `true` when the exact group/version is present (`apps/v1`), and also
when a kind is given in Helm's `group/version/Kind` form and the group/version
part is registered.

Paths into the map do not work — a slash splits the path and there is no
bracket or method syntax:

```yaml
${capabilities.apiVersions.apps/v1}          # slash splits the path
${capabilities.apiVersions['apps/v1']}       # no bracket form
```

For version-driven branching, `kubeVersion` + `semverCompare` is usually enough.

## `lookup` — reading cluster state

`lookup` pulls a live resource into the render:

```
lookup(apiVersion, kind, namespace, name)
```

All four arguments are literals. It returns the resource as a map, or a falsy
value when the resource is absent — `nil` under `keramos template` (no cluster),
an empty map against a live cluster. Pass an empty `name` to list a kind.

Guard on the result so a missing resource simply drops the block:

**Template:**

```yaml
$if: ${lookup 'v1' 'ConfigMap' 'kube-system' 'cluster-info'}
data:
  found: "yes"
```

Under `keramos template` this renders nothing — `lookup` returns nil, `$if` is
falsy.

To read a field out of the result, pipe it through `get` or `dig` rather than a
dotted sub-path (the engine can't index the result of a call inline):

```yaml
data:
  ca: ${lookup 'v1' 'ConfigMap' 'kube-system' 'cluster-info' | get 'data' | get 'ca.crt'}
```

`dig` walks several keys and takes a fallback as its last argument:

```yaml
data:
  ca: ${lookup 'v1' 'ConfigMap' 'kube-system' 'cluster-info' | dig 'data' 'ca.crt' ''}
```

Keramos caches `lookup` results within a single render, so repeated lookups of the
same tuple hit the API server once.

## Offline behaviour

Under `keramos template` (no kubeconfig):

- `capabilities.kubeVersion.Version` / `.GitVersion` come from `--kube-version`;
  omit the flag and they are unset (nil).
- `capabilities.kubeVersion.Major` / `.Minor` are always null.
- `capabilities.apiVersions` contains exactly the `--api-versions` entries.
- `lookup` returns nil for everything.

Templates that depend on `lookup` therefore take their empty/false branches
under `keramos template`. To preview against a real cluster's view, use `keramos diff`
or `keramos plan`, which read live capabilities and lookups.

## See also

- [Expressions](expressions.md) — why `semverCompare` needs the pipeline form
- [Function reference](functions.md) — `semverCompare`, `fail`, `get`, `dig`
- [Control flow](control-flow.md) — `$if` gating on capabilities and `lookup`
{% endraw %}
