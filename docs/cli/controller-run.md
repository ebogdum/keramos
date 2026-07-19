---
title: "keramos controller run"
parent: "CLI"
---
{% raw %}
# keramos controller run

## Synopsis

`keramos controller run` starts the reconcile loop in the foreground. On a fixed
interval it lists every `KeramosRelease` in the cluster, and for each one installs
or upgrades the release its spec describes, then records the outcome on the
CR's `status`. This is the process you deploy as a Deployment (or run under
systemd) to make keramos operate as an in-cluster operator.

It runs until you stop it (Ctrl-C, or the pod is terminated). Stopping it halts
reconciliation but leaves every already-installed release in place.

## When to use it

- To run keramos as a Kubernetes-native operator driven by `KeramosRelease` CRs —
  typically behind a GitOps engine (Argo CD, Flux) that applies the CRs while
  the controller turns them into real releases.
- For one-off installs from a workstation, [`install`](install.md) and
  [`upgrade`](upgrade.md) are simpler; you don't need the controller.

## What happens

1. The loop starts and immediately runs a reconcile pass, then repeats every
   `--interval`.
2. Each pass lists every `KeramosRelease` in `--watch-namespace` (empty = all
   namespaces). A CR whose `resourceVersion` is unchanged since the last pass
   is skipped.
3. For each changed CR it reads `spec.package` and resolves it under
   `--package-root`. A package that resolves outside that root — an absolute
   path, a `..` sequence, or a symlink escape — is rejected and the CR is
   marked `Failed`.
4. It renders the package with the CR's `spec.values` and `spec.profile`, then
   installs or upgrades the release (named `spec.releaseName`, defaulting to the
   CR's name) into the CR's namespace, waiting for readiness and rolling back on
   failure.
5. It writes the result to the CR's `status`: `phase` (`Deployed` or `Failed`),
   `message`, `revision`, and `lastTransition`. Secret-shaped substrings in any
   error message are redacted before being stored.
6. While everything reconciles cleanly the process is quiet; a failed CR is
   logged to stderr as a `[WARN]` line and the loop continues with the next CR.

## Usage

```
keramos controller run [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--interval` | duration | `30s` | how often to re-list and reconcile every KeramosRelease; lower it for a tighter loop, raise it to reduce API load |
| `--package-root` | string | `/var/lib/keramos/packages` | directory that CR-supplied `spec.package` paths must resolve under; anything resolving outside it is rejected, so a namespaced tenant cannot point the controller at `/etc` or a secret mount |
| `--watch-namespace` | string | `""` | namespace to watch; empty watches every namespace |

Global flags also apply:

| Flag | Type | Default | Description |
|---|---|---|---|
| `--debug` | — | — | print debug output for each reconcile pass |
| `--kube-context` | string | (current) | which cluster to reconcile |
| `--kubeconfig` | string | (default) | path to the kubeconfig file |
| `-n, --namespace` | string | — | Kubernetes namespace |

## Worked example

Install the CRD, create one `KeramosRelease`, then start the loop:

```sh
keramos controller install-crd

kubectl apply -f - <<'EOF'
apiVersion: keramos.dev/v1
kind: KeramosRelease
metadata:
  name: web
  namespace: apps
spec:
  package: web          # a package pre-provisioned at /var/lib/keramos/packages/web
  values:
    replicas: 3
EOF

keramos controller run --watch-namespace apps
```

**Output:** with a valid `web` package, the loop stays silent — it installs the
release and records the result on the CR. Read the outcome from the object:

```sh
kubectl get keramosrelease web -n apps -o jsonpath='{.status}'
```

```
{"phase":"Deployed","message":"ok","revision":1,"lastTransition":"2026-07-18T14:05:33Z"}
```

If a CR points at a package that isn't under `--package-root`, that CR fails
and the loop logs it to stderr, then carries on:

```
[WARN] KeramosRelease apps/web: package "../../etc" does not exist under allowlisted root
```

and its status reads:

```
{"phase":"Failed","message":"package \"../../etc\" ...","revision":0,"lastTransition":"..."}
```

Tighten the loop on a dev cluster:

```sh
keramos controller run --interval 5s --debug
```

## See also

- [`controller install-crd`](controller-install-crd.md) — register the CRD first
- [`controller crd`](controller-crd.md) — inspect the CRD schema
- [`install`](install.md) / [`upgrade`](upgrade.md) — the operations each reconcile runs
- [`controller`](controller.md) — operator overview
{% endraw %}
