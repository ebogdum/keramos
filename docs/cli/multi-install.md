# keramos multi-install

Install one package as the same release into several clusters in a single
invocation.

## When to use it

- Rolling the same release out to many clusters with identical parameters —
  per-region copies of a platform service, or a fleet of edge clusters.
- Fan-out installs from CI where you want one command, one result table, and an
  optional all-or-nothing guarantee across clusters.

Each target is a kubeconfig context named in `--to`. For a single cluster use
[`keramos install`](install.md); to drive many releases across contexts from a
workspace file, see [`keramos workspace`](workspace.md).

## What happens

1. Reads the target contexts from `--to` (comma-separated). At least one is
   required.
2. Installs `<release-name>` from `<package-path>` into each context in
   parallel, up to `--parallel` at a time, applying the same values, `--set`
   overrides, `--profile`, and `--env` to every target. All installs use the
   namespace from `-n/--namespace`.
3. Waits for each cluster's resources to become ready (up to `--timeout`) unless
   `--no-wait` is set.
4. Prints one result row per context. With `--atomic-cross-cluster`, if any
   context fails, the successful installs are rolled back and the command exits
   non-zero; without it, each install stands on its own (eventual consistency).

This mutates every target cluster and requires all named contexts to be
reachable.

## Usage

```
keramos multi-install <release-name> <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--to` | strings | — | kubeconfig contexts to install into (comma-separated); required |
| `--atomic-cross-cluster` | — | false | if any context fails, roll back the successful ones and fail |
| `--parallel` | int | 4 | number of contexts to install into concurrently |
| `--timeout` | duration | 5m0s | per-cluster wait for resources to become ready |
| `--no-wait` | — | false | do not wait for resources to be ready in any cluster |
| `--profile` | string | — | profile to apply |
| `--env` | string | — | environment name declared in `keramos.yaml`'s `environments:` section |
| `-f, --values` | stringArray | — | values file override (repeatable) |
| `--set` | stringArray | — | key=value override (repeatable) |
| `--set-string` | stringArray | — | key=value forced as string (repeatable) |
| `--set-file` | stringArray | — | key=path; value read from path (repeatable) |
| `--set-json` | stringArray | — | key=<json>; value parsed as JSON (repeatable) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Worked example

**INPUT — install `web` into three regional clusters:**

```sh
keramos multi-install web ./web \
  --to ctx-eu,ctx-us,ctx-ap \
  -n prod --set image.tag=1.5.0
```

**OUTPUT — one row per target context:**

```
CONTEXT   STATUS   DETAIL
ctx-eu    ok       revision 1, status deployed
ctx-us    ok       revision 1, status deployed
ctx-ap    ok       revision 1, status deployed
```

**With `--atomic-cross-cluster`, one bad cluster rolls back the rest:**

```sh
keramos multi-install web ./web \
  --to ctx-eu,ctx-us,ctx-ap \
  -n prod --atomic-cross-cluster
```

```
CONTEXT   STATUS   DETAIL
ctx-eu    ok       revision 1, status deployed
ctx-us    ok       revision 1, status deployed
ctx-ap    FAIL     wait timed out after 5m0s

Atomic cross-cluster install failed — rolling back successful installs.
```

The command exits non-zero, and `ctx-eu` and `ctx-us` are uninstalled so no
cluster is left in a partial state.

## See also

- [`install`](install.md) — install one release into a single cluster
- [`workspace`](workspace.md) — drive many releases across contexts from a workspace file
- [`upgrade`](upgrade.md) — upgrade an existing release
