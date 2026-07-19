# keramos get manifest

`keramos get manifest` prints the rendered Kubernetes manifest that keramos stored for
a release — the exact YAML it applied at that revision.

## When to use it

- You want to see what keramos applied, to diff it against the live cluster.
- You want to capture a revision's manifest to a file for offline comparison.

## What happens

It loads the stored release record for `<release>` (the latest revision, or the
one named by `--revision`) and prints the record's `manifest` field verbatim to
stdout. The manifest is the fully-resolved YAML from the last install or
upgrade; nothing is re-rendered and the cluster's live objects are not read.

## Flags

| Flag | Cause | Effect |
|---|---|---|
| `--revision <n>` | you name a stored revision | prints that revision's manifest instead of the latest |
| `-o, --output <fmt>` | you pass `raw`, `json`, or `yaml` | accepted for parity with sibling commands, but ignored — the manifest is always printed raw |

Inherits the global flags (`-n/--namespace`, `--kube-context`, `--kubeconfig`,
`--debug`).

## Usage

```
keramos get manifest <release> [flags]
```

## Worked example

Stored record for `hello` (revision 4), its `manifest` field:

```yaml
# what keramos recorded at the last upgrade
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello
  namespace: prod
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: hello
          image: registry/hello:1.5.0
```

Run it:

```sh
keramos get manifest hello -n prod
```

Output — the stored manifest, printed raw and unchanged:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello
  namespace: prod
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: hello
          image: registry/hello:1.5.0
```

## See also

- [`get`](get.md) — the parent command
- [`get all`](get-all.md) — the full record, not just the manifest
- [`drift`](drift.md) — compare the stored manifest against the live cluster
