---
title: "keramos controller crd"
parent: "CLI"
---
{% raw %}
# keramos controller crd

## Synopsis

`keramos controller crd` prints the `KeramosRelease` CustomResourceDefinition as YAML
to stdout. This is the exact definition the controller reconciles, and the same
YAML that [`controller install-crd`](controller-install-crd.md) applies — this
command only prints it, it never touches the cluster.

## When to use it

- To read the CRD schema before you register it.
- To commit the CRD into a GitOps repo so your delivery tool applies it.
- To pipe it into `kubectl` from a machine that has keramos but where you'd rather
  apply through your own tooling.

## What happens

1. Keramos writes the embedded CRD definition (group `keramos.dev`, kind
   `KeramosRelease`) to stdout.
2. Nothing else — no cluster contact, no files written, exit 0.

## Usage

```
keramos controller crd
```

## Flags

Inherits the global flags.

| Flag | Type | Default | Description |
|---|---|---|---|
| `--debug` | — | — | print debug output while running |
| `--kube-context` | string | (current) | Kubernetes context to use |
| `--kubeconfig` | string | (default) | path to the kubeconfig file |
| `-n, --namespace` | string | — | Kubernetes namespace |

(This command reads none of them; they are accepted because every keramos command
carries them.)

## Worked example

Print the CRD and save it for review:

```sh
keramos controller crd > keramosrelease-crd.yaml
```

**Output** (`keramosrelease-crd.yaml`):

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: keramosreleases.keramos.dev
spec:
  group: keramos.dev
  scope: Namespaced
  names:
    plural: keramosreleases
    singular: keramosrelease
    kind: KeramosRelease
    shortNames: [hr]
  versions:
    - name: v1
      served: true
      storage: true
      subresources:
        status: {}
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              required: [package]
              properties:
                releaseName: { type: string }
                package:     { type: string }
                version:     { type: string }
                profile:     { type: string }
                values:
                  type: object
                  x-kubernetes-preserve-unknown-fields: true
            status:
              type: object
              properties:
                phase:          { type: string }
                message:        { type: string }
                revision:       { type: integer }
                lastTransition: { type: string }
```

Apply it through kubectl instead of `install-crd`:

```sh
keramos controller crd | kubectl apply -f -
```

## See also

- [`controller install-crd`](controller-install-crd.md) — apply this CRD directly
- [`controller run`](controller-run.md) — start the reconciler once the CRD exists
- [`controller`](controller.md) — operator overview
{% endraw %}
