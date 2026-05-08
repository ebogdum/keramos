# Quickstart

This guide takes you from zero to a deployed, upgraded, and rolled-back release in about ten minutes. It assumes you already have a working `kubectl` against some Kubernetes cluster — anything from `kind`, `k3d`, `k3s` and `minikube` to a managed service will work.

## Prerequisites

- A Kubernetes cluster reachable through your current `kubectl` context.
- The `keramos` binary on your `$PATH`. See [Installing Keramos](../../README.md#installing-keramos) if not yet installed.
- Permission to create namespaces, Deployments, Services, ConfigMaps, and Secrets in the target cluster.

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
├── README.md
├── keramos.yaml
├── values.yaml
└── templates/
    ├── _helpers.yaml
    ├── configmap.yaml
    ├── deployment.yaml
    └── service.yaml
```

The scaffolded package is a small "hello-world" web server. Open the files and look around — `keramos.yaml` declares the package; `values.yaml` is the configurable surface; `templates/` is where YAML manifests with `${...}` expressions live. The `_helpers.yaml` file (note the leading underscore) is a *partial*: included from other templates, never rendered as a standalone manifest.

## 2. Lint it

```sh
keramos lint .
```

Lint runs YAML parsing, schema validation (if `values.schema.json` is present), template rendering, and a static set of best-practice checks (e.g. resources without limits, hostPort usage, image tags pinned to `latest`). A clean lint exits 0.

## 3. Render templates locally

```sh
keramos template .
```

This renders every template against the package's `values.yaml`, with the release name `hello`, and prints the resulting Kubernetes manifests to stdout. **Nothing has touched the cluster yet.** Use `keramos template` whenever you want to inspect what keramos would apply.

To override a value:

```sh
keramos template . --set replicas=3
```

Or with a values file:

```sh
keramos template . -f overrides.yaml
```

## 4. Install

```sh
keramos install hello . -n keramos-quickstart --create-namespace
```

What happens:

1. The package is rendered the same way `keramos template` rendered it.
2. Keramos stamps `managedBy=keramos` on every resource and on the namespace it creates.
3. Pre-install hooks (if any) run.
4. Server-side apply pushes the rendered manifest into the cluster.
5. Post-install hooks run.
6. The release record is stored as a labelled Secret in the install namespace.
7. Keramos waits for the rendered resources to become Ready (Deployment available, Pod Ready, etc.) and prints a summary.

Check what landed:

```sh
kubectl -n keramos-quickstart get deploy,svc,cm
kubectl -n keramos-quickstart get all -l managedBy=keramos
```

## 5. List, status, manifest

```sh
keramos list                          # releases in the current namespace
keramos list -A                       # every release everywhere
keramos status hello -n keramos-quickstart
keramos get manifest hello -n keramos-quickstart
keramos get values hello -n keramos-quickstart
```

`keramos status` shows the current revision, package, and per-resource readiness. `keramos get manifest` prints the exact YAML keramos stored for this revision (gzipped + base64 inside the release Secret; keramos decompresses it on the fly).

## 6. Upgrade

Edit `values.yaml` (e.g. bump replicas) or any template, then:

```sh
keramos upgrade hello . -n keramos-quickstart
```

Each upgrade increments the release's revision counter and stores the new manifest. The cluster sees server-side apply with the new fields. Hooks tagged `pre-upgrade` and `post-upgrade` run before and after the apply.

## 7. Diff before applying

`keramos diff` shows what would change without applying. It always uses server-side dry-run, so the cluster's defaulters and webhooks contribute to the comparison — you see the diff the cluster would actually compute.

```sh
keramos diff hello . -n keramos-quickstart
```

## 8. Plan and apply

For change-management workflows, separate "what keramos would do" from "do it":

```sh
keramos plan hello . -n keramos-quickstart -o hello.plan
# review the plan...
keramos apply hello.plan
```

`keramos plan` produces a self-contained plan file (rendered manifest + parameters + hash). `keramos apply` consumes one and executes the upgrade exactly as planned. The plan binds to the release name and namespace so you cannot accidentally apply it elsewhere.

## 9. History and rollback

```sh
keramos history hello -n keramos-quickstart
```

prints every revision with its timestamp, status, and audit data (who installed it, with what flags). To roll back:

```sh
keramos rollback hello 1 -n keramos-quickstart
```

Keramos re-applies revision 1's stored manifest and re-runs revision 1's `pre-rollback` and `post-rollback` hooks (which are persisted alongside the manifest, so a rollback to an old revision uses the hooks that revision originally shipped, not the current ones).

## 10. Drift detection

After an install, the cluster might be edited out-of-band — somebody runs `kubectl edit deploy hello`, or another operator picks up a field. `keramos drift` compares the live state against the release's stored manifest:

```sh
keramos drift hello -n keramos-quickstart
```

Drift output is per-resource, per-field. To re-converge to the stored manifest:

```sh
keramos reconcile hello -n keramos-quickstart
```

This re-applies the stored manifest, taking ownership of any drifted fields.

## 11. Audit

```sh
keramos audit hello -n keramos-quickstart
```

prints the full audit trail: every revision, the action (`install`, `upgrade`, `rollback`, `uninstall`), who initiated it, the kubeconfig context, the keramos version, the flags as passed, and any value files supplied. This is signed metadata stored in the release record.

## 12. Uninstall

```sh
keramos uninstall hello -n keramos-quickstart
```

Pre-delete hooks run, the manifest's resources are deleted, post-delete hooks run, and the release record is removed. Pass `--keep-history` to keep the release record for forensic purposes; `keramos list --filter status=uninstalled` shows kept-history releases.

To clean up the namespace too:

```sh
kubectl delete ns keramos-quickstart
```

## Where next

- [Package anatomy](packages.md) — every file in a package, in detail.
- [Values](values.md) — how `values.yaml`, layers, environments, and CLI flags merge.
- [Layers](layers.md) — composing a package from reusable building blocks.
- [Hooks](hooks.md) — Job-based lifecycle hooks.
- [Workspaces](workspaces.md) — orchestrating many packages with one command.
- [Template expressions](../templates/expressions.md) — the `${...}` syntax.
- [Template functions](../templates/functions.md) — every built-in, with input/output examples.
