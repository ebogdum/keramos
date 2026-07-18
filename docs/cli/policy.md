# keramos policy

## Synopsis

`keramos policy` evaluates package-defined policy rules (under `policies/`) against the rendered manifest. Rules are Keramos policy YAML (declarative match-and-require).

## When to use it

Use as a CI gate to enforce organisation-wide rules: "every Pod sets runAsNonRoot", "every Service has a selector", "no hostNetwork", etc. Policies live with the package so they ship together.

## Usage

```
keramos policy [command]
```

## Subcommands

- [`keramos policy list`](policy-list.md) — List policy rules declared in the package

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | — | — | help for policy |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Writing a policy

Policies are **your own YAML files** under `<package>/policies/`. Each file may
hold one or more rules (separated by `---`). A rule has:

| Field | Meaning |
|---|---|
| `name` | rule name (required) |
| `severity` | `deny` (fails the check, default) or `warn` (prints only) |
| `match` | selector — `kinds`, `namespaces`, `names`, `apiVersion`; empty = any |
| `require` | predicates every matched resource must satisfy |
| `forbid` | predicates that must NOT hold |
| `message` | custom text prepended to the violation detail |

Predicates available under `require` / `forbid`: `fields`, `labelKeys`,
`annotationKeys`, `imageRegistries` (allowed image prefixes, matched at a host
boundary), `imageNotTagged` (bans `:latest`/untagged; a digest pins an image),
`resourceRequests` / `resourceLimits` (every container must declare them),
`minReplicas`.

`fields` takes dotted paths that traverse arrays. Semantics differ by block:

- `require.fields`: the path must be present and non-empty at **every** position
  it reaches — including **every element** when it crosses an array (so a rule
  on `spec.template.spec.containers.securityContext.runAsNonRoot` requires *all*
  containers to set it, not just one). A value that is explicitly present counts
  even when it is a zero value (`false`, `0`) — require checks presence, not
  truthiness.
- `forbid.fields`: a violation if the field is present (non-zero) at **any**
  position — one offending array element is enough to fail.

`severity` must be `deny` (default) or `warn`; any other value is rejected at
load time so a typo cannot silently disable a gate.

Example `./my-app/policies/security.yaml`:

```yaml
name: min-replicas-3
severity: deny
match:
  kinds: [Deployment]
require:
  minReplicas: 3
message: production deployments need at least 3 replicas
---
name: no-latest-tag
severity: deny
match: { kinds: [Deployment] }
require: { imageNotTagged: true }
```

## Examples

Gate a rendered package in CI (in → out). Failing render:

```sh
keramos template ./my-app | keramos policy check ./my-app
```

```
[DENY] min-replicas-3 — apps/v1/Deployment/app : spec.replicas (1) is below minReplicas 3
[DENY] no-latest-tag  — apps/v1/Deployment/app : container image "nginx:latest" uses :latest or no tag
Error: policy violations exist        # exit code 1
```

Passing render (fixed values):

```sh
keramos template ./my-app --set replicas=5 --set-string image=nginx:1.25 | keramos policy check ./my-app
```

```
ok — 2 rule(s) passed                 # exit code 0
```

List the rules a package ships with (no evaluation):

```sh
keramos policy list ./my-app
```

```
min-replicas-3 [deny]
no-latest-tag [deny]
```

These same policies are enforced automatically on `keramos install` and
`keramos upgrade` for the package — a `deny` violation aborts the operation.

## See also

- [`policy check`](policy-check.md)
- [`policy list`](policy-list.md)
- [`lint`](lint.md)
