---
title: "Quickstart"
parent: "Guides"
---
{% raw %}
# Quickstart

By the end of this guide you will have scaffolded a package, installed it as a
release, upgraded it, previewed and reverted changes, inspected drift, and
uninstalled it — all with the real commands and their real output. It takes
about ten minutes and assumes a working `kubectl` against any cluster (`kind`,
`k3d`, `k3s`, `minikube`, or a managed service).

## Prerequisites

- A Kubernetes cluster reachable through your current `kubectl` context.
- The `keramos` binary on your `$PATH`. See
  [Quick install](../../README.md#quick-install) if you do not have it yet.
- Permission to create namespaces, Deployments, and Services in the cluster.

Sanity check:

```sh
keramos version
kubectl get nodes
```

## 1. Scaffold a package

```sh
keramos create hello
cd hello
```

`keramos create` writes:

```
hello/
├── .keramosignore
├── keramos.yaml
├── values.yaml
└── templates/
    ├── _helpers.yaml
    ├── deployment.yaml
    ├── notes.yaml
    └── service.yaml
```

The scaffold is a minimal nginx Deployment plus a Service. `keramos.yaml` declares
the package; `values.yaml` is the configurable surface; `templates/` holds the
manifests, written with `${...}` expressions. `_helpers.yaml` (leading
underscore) is a *partial* — a bag of reusable snippets, never emitted as a
standalone manifest. `templates/notes.yaml` is a document with a single
`message:` key; keramos treats any such document as the release notes rather than a
manifest.

The default `values.yaml`:

```yaml
name: hello
replicaCount: 1
image:
  repository: nginx
  tag: latest
service:
  port: 80
```

## 2. Lint it

```sh
keramos lint .
```

```
lint passed
```

`keramos lint` checks that `keramos.yaml` and `values.yaml` parse, that
`values.schema.json` (if present) is valid JSON, and that every template
renders. It does **not** validate values against the schema — that happens at
render time (step 3). See [`keramos lint`](../cli/lint.md).

## 3. Render templates locally

```sh
keramos template .
```

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: hello
  name: hello
spec:
  replicas: 1
  selector:
    matchLabels:
      app: hello
  template:
    metadata:
      labels:
        app: hello
    spec:
      containers:
        - image: nginx:latest
          name: hello
          ports:
            - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  labels:
    app: hello
  name: hello
spec:
  ports:
    - port: 80
      protocol: TCP
      targetPort: 80
  selector:
    app: hello
  type: ClusterIP
```

`keramos template` renders every template against `values.yaml` and prints the
manifests to stdout. **Nothing has touched the cluster.** Override a value with
`--set`:

```sh
keramos template . --set replicaCount=3
```

or with a values file:

```sh
keramos template . -f overrides.yaml
```

See [`keramos template`](../cli/template.md).

## 4. Install

```sh
keramos install hello . -n keramos-quickstart --create-namespace
```

```
NOTES:
hello has been installed successfully.
Namespace: keramos-quickstart
Run "kubectl get deployments" to verify.
```

Keramos renders the package, stamps `managedBy=keramos` on every resource,
server-side-applies the manifest, waits for the resources to become ready, and
stores the release record as a labelled Secret in the namespace. Check what
landed:

```sh
kubectl -n keramos-quickstart get all -l managedBy=keramos
```

```
NAME                        READY   STATUS    RESTARTS   AGE
pod/hello-bc94584c5-lfgvr   1/1     Running   0          24s

NAME            TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)   AGE
service/hello   ClusterIP   10.43.65.214   <none>        80/TCP    24s

NAME                    READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/hello   1/1     1            1           25s
```

See [`keramos install`](../cli/install.md).

## 5. List, status, manifest

```sh
keramos list -n keramos-quickstart
```

```
NAME     NAMESPACE          REVISION    STATUS      PACKAGE    VERSION    UPDATED
hello    keramos-quickstart    1           deployed    hello      0.1.0      2026-07-18 22:06:02
```

```sh
keramos status hello -n keramos-quickstart
```

```
NAME:       hello
NAMESPACE:  keramos-quickstart
STATUS:     deployed
REVISION:   1
PACKAGE:    hello-0.1.0
UPDATED:    2026-07-18 22:06:02

NOTES:
hello has been installed successfully.
...
```

`keramos get manifest hello -n keramos-quickstart` prints the exact YAML keramos stored
for the revision; `keramos get values hello -n keramos-quickstart` prints the merged
values it used. Use `keramos list -A` to see releases across all namespaces. See
[`keramos list`](../cli/list.md), [`keramos status`](../cli/status.md), and
[`keramos get`](../cli/get.md).

## 6. Upgrade

Edit `values.yaml` or a template, then re-apply. Here, bump the replica count:

```sh
keramos upgrade hello . -n keramos-quickstart --set replicaCount=3
```

Each upgrade increments the revision counter, stores the new manifest, and
server-side-applies it. `keramos history` now shows two revisions:

```sh
keramos history hello -n keramos-quickstart
```

```
REVISION    STATUS        PACKAGE        UPDATED                DESCRIPTION
1           superseded    hello-0.1.0    2026-07-18 22:06:02
2           deployed      hello-0.1.0    2026-07-18 22:07:32
```

See [`keramos upgrade`](../cli/upgrade.md) and [`keramos history`](../cli/history.md).

## 7. Preview changes before applying

`keramos diff` compares local inputs only — it never reads the cluster. Render the
package two ways and diff them:

```sh
keramos diff . --to-set replicaCount=5
```

```
diff: from → to

~ update  Deployment/hello
      ~ spec.replicas
          - 1
          + 5

Summary: 0 added, 1 changed, 0 removed.
```

To compare the package against what keramos last recorded for the release, use
`keramos plan` instead:

```sh
keramos plan . -r hello -n keramos-quickstart --action upgrade
```

```
keramos plan: update  hello / keramos-quickstart  (package .)

~ update  Deployment/hello
      from: deployment.yaml
      ~ spec.replicas
          - 3   (state)
          + 1   ← package-default (values.yaml)

Plan: 0 to add, 1 to change, 0 to destroy.
```

See [`keramos diff`](../cli/diff.md) and [`keramos plan`](../cli/plan.md).

## 8. Plan and apply

For change-management workflows, separate "what keramos would do" from "do it".
`keramos plan --out` writes a self-contained JSON artifact (rendered manifest plus
a sha256 integrity digest, bound to the release name and namespace):

```sh
keramos plan . -r hello -n keramos-quickstart --action upgrade --set replicaCount=5 --out plan.json
```

```
plan written to plan.json
```

```sh
keramos apply --plan plan.json -n keramos-quickstart
```

```
applied upgrade for hello revision 3
```

`keramos apply` executes exactly what the plan captured. See
[`keramos apply`](../cli/apply.md).

## 9. Roll back

```sh
keramos rollback hello 1 -n keramos-quickstart
```

Keramos re-applies revision 1's stored manifest and records a new revision. The
audit trail records every action:

```sh
keramos audit hello -n keramos-quickstart
```

```
REVISION    ACTION     USER      STATUS        TIMESTAMP
1           install    ada    superseded    2026-07-18 22:06:02
2           upgrade    ada    deployed      2026-07-18 22:07:32
```

See [`keramos rollback`](../cli/rollback.md) and [`keramos audit`](../cli/audit.md).

## 10. Detect and reconcile drift

`keramos drift` compares three views — the package as it renders now, the recorded
state, and the live cluster. It locates each live object by name **and
namespace**, so the resource templates must carry their namespace. Add one line
under `metadata` in `templates/deployment.yaml` and `templates/service.yaml`:

```yaml
metadata:
  name: "${values.name}"
  namespace: ${release.namespace}
```

Apply the edit, then change the cluster out of band and compare:

```sh
keramos upgrade hello . -n keramos-quickstart
kubectl -n keramos-quickstart scale deploy hello --replicas=7
keramos drift . -r hello -n keramos-quickstart
```

```
drift: package ↔ state ↔ running   (release hello)

~ differs                Deployment/hello  (namespace keramos-quickstart)
      spec.replicas  ⚠ cluster drift
          package: 1
          state:   1
          running: 7

1 cluster-drift, 0 pending-apply, 0 orphan, 0 missing, 0 to-create.
```

Push the recorded state back onto the cluster:

```sh
keramos reconcile hello -n keramos-quickstart
```

```
Reconciled 2 resource(s):
  - Deployment/hello
  - Service/hello
```

See [`keramos drift`](../cli/drift.md) and [`keramos reconcile`](../cli/reconcile.md).

## 11. Uninstall

```sh
keramos uninstall hello -n keramos-quickstart
```

Keramos deletes the release's resources. History is **kept** by default (so
`keramos audit` and `keramos rollback` still work); pass `--purge` to delete the
release record too. List kept-history releases with:

```sh
keramos list --uninstalled -n keramos-quickstart
```

Remove the namespace when you are done:

```sh
kubectl delete ns keramos-quickstart
```

See [`keramos uninstall`](../cli/uninstall.md).

## Next steps

- [Package anatomy](packages.md) — every file in a package.
- [Values](values.md) — how `values.yaml`, layers, environments, profiles, and
  CLI flags merge.
- [Layers](layers.md) — composing a package from reusable building blocks.
- [Schema validation](schema-validation.md) — `values.schema.json` patterns.
- [Hooks](hooks.md) — lifecycle Jobs and Pods.
- [Workspaces](workspaces.md) — orchestrating many packages at once.
- [Template expressions](../templates/expressions.md) and
  [function reference](../templates/functions.md) — the `${...}` language.
- [CLI reference](../cli/README.md) — every command and flag.
{% endraw %}
