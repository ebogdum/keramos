---
title: "keramos policy check"
parent: "CLI"
---
{% raw %}
# keramos policy check

## Synopsis

`keramos policy check` reads a rendered manifest and evaluates every rule under
`<package-path>/policies/` against it. It prints one line per violation and
exits non-zero when any `deny`-severity rule is broken, so you can wire it
straight into CI.

## When to use it

- As a deny-gate in CI: render the package, pipe it through `policy check`,
  and fail the build on a violation.
- Locally before you commit or apply, to catch a bad image tag or a missing
  resource limit without touching a cluster.

## What happens

1. Loads every rule from `<package-path>/policies/*.yaml`. If the package has
   no policies, it prints `no policies/ rules found` and exits 0.
2. Reads the manifest from `--manifest <file>`, or from stdin when the flag is
   absent. An empty manifest is an error.
3. Evaluates each rule against each resource in the manifest.
4. If nothing is violated, prints `ok — N rule(s) passed` and exits 0.
5. Otherwise prints each violation as `[SEVERITY] rule — apiVersion/Kind/name
   in ns namespace: detail`, then exits non-zero if any violation is `deny`
   (a `warn`-only run still exits 0).

## Usage

```
keramos policy check <package-path> [flags]
```

## Flags

| Flag | Cause → effect |
|---|---|
| `--manifest <file>` | Read the manifest from this file instead of stdin. |

Also inherits the global flags.

## Worked example

**INPUT — the rule** in `mychart/policies/images.yaml`:

```yaml
name: no-latest-tag
severity: deny
match:
  kinds: [Deployment]
require:
  imageNotTagged: true
message: pin images to an explicit tag
```

**INPUT — the rendered manifest.** The package still points at a floating tag:

```yaml
# keramos template ./mychart  →  the Deployment it produces
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: default
spec:
  template:
    spec:
      containers:
        - name: web
          image: nginx:latest        # ← floating tag, forbidden by the rule
```

**Run it:**

```sh
keramos template ./mychart | keramos policy check ./mychart
```

**OUTPUT — it fails, naming the rule and the offending resource:**

```
[DENY] no-latest-tag — apps/v1/Deployment/web in ns default: pin images to an explicit tag (container image "nginx:latest" uses :latest or no tag)
Error: policy violations exist
```

The command exits non-zero, so the pipeline stops.

**Now pin the image** (`image: nginx:1.27.1`) and run it again:

```sh
keramos template ./mychart | keramos policy check ./mychart
```

**OUTPUT — it passes:**

```
ok — 1 rule(s) passed
```

Now the command exits 0 and the build proceeds.

## See also

- [`policy`](policy.md) — the parent command
- [`policy list`](policy-list.md) — show what rules would run
- [`template`](template.md) — render the manifest to pipe in
- [`install`](install.md) — apply the package once it passes
{% endraw %}
