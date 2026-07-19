---
title: "keramos controller"
parent: "CLI"
---
{% raw %}
# keramos controller

## Synopsis

`keramos controller` runs keramos as an in-cluster operator. Instead of you typing
`keramos install` / `keramos upgrade` from a workstation, you declare what you want
as `KeramosRelease` custom resources, and a long-running controller process
reconciles the cluster to match them.

The workflow has three parts, one per subcommand:

```
controller crd          — print the KeramosRelease CRD definition
controller install-crd  — register that CRD in the cluster
controller run          — start the reconcile loop that acts on KeramosReleases
```

Once the CRD is installed and the loop is running, you (or a GitOps engine)
create a `KeramosRelease` object like this:

```yaml
apiVersion: keramos.dev/v1
kind: KeramosRelease
metadata:
  name: web
  namespace: apps
spec:
  package: web            # path under the controller's --package-root
  releaseName: web        # optional; defaults to metadata.name
  profile: prod           # optional
  values:                 # optional inline values
    replicas: 3
```

On its next tick the controller renders that package, installs or upgrades the
release, and writes the result back to the object's `status` (phase, message,
revision). Edit the `KeramosRelease` and the controller reconciles again; a CR it
has already applied and that has not changed is skipped.

## Subcommands

| Command | Description |
|---|---|
| [`crd`](controller-crd.md) | Print the KeramosRelease CRD YAML to stdout |
| [`install-crd`](controller-install-crd.md) | Apply the KeramosRelease CRD to the cluster |
| [`run`](controller-run.md) | Run the reconcile loop in the foreground |

## Usage

```
keramos controller [command]
```

Bring the operator up in the usual order:

```sh
keramos controller install-crd      # once per cluster
keramos controller run              # long-running; deploy it as a Deployment
```

## See also

- [`install`](install.md) — the one-shot install the controller runs for you
- [`upgrade`](upgrade.md) — the one-shot upgrade the controller runs for you
- [`reconcile`](reconcile.md) — manually converge a single release
{% endraw %}
