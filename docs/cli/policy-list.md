---
title: "keramos policy list"
parent: "CLI"
---
{% raw %}
# keramos policy list

## Synopsis

`keramos policy list` prints the policy rules a package declares under
`<package-path>/policies/`, one per line, with each rule's severity. It does
not evaluate anything or read a manifest.

## When to use it

- To audit what guardrails a package self-imposes before you consume it —
  especially an upstream package you did not write.
- To confirm which rules `keramos policy check` will run.

## What happens

1. Loads every rule from `<package-path>/policies/*.yaml`.
2. Prints each rule as `name [severity]`, in declaration order.
3. Reads only local files — no manifest, no cluster, no network.

## Usage

```
keramos policy list <package-path> [flags]
```

## Flags

Inherits the global flags.

## Worked example

**INPUT — two rules** under `mychart/policies/`:

```yaml
# mychart/policies/images.yaml
name: no-latest-tag
severity: deny
match: { kinds: [Deployment] }
require: { imageNotTagged: true }
---
# mychart/policies/scale.yaml
name: min-replicas-3
severity: warn
match: { kinds: [Deployment] }
require: { minReplicas: 3 }
```

**Run it:**

```sh
keramos policy list ./mychart
```

**OUTPUT — each rule and its severity:**

```
no-latest-tag [deny]
min-replicas-3 [warn]
```

`deny` rules fail `keramos policy check`; `warn` rules only print.

## See also

- [`policy`](policy.md) — the parent command
- [`policy check`](policy-check.md) — actually evaluate the rules
- [`install`](install.md) — apply the package once it passes
{% endraw %}
