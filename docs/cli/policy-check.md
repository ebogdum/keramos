# keramos policy check

## Synopsis

`keramos policy check` evaluates every policy under `<package-path>/policies/` against a rendered Kubernetes manifest, returning a non-zero exit code when any rule reports a violation. The manifest comes from stdin (the typical pattern is `keramos template ... | keramos policy check ./pkg`) or from `--manifest <file>`. Policies can be either declarative keramos-policy YAML or full Rego (`.rego`) files for OPA-style logic.

## When to use it

Use as a deny-gate in CI: render the package, pipe through policy check, and fail the build on any violation. Also useful locally before committing — catch missing-resource-limits or missing-security-context issues without leaving your editor.

## What happens when you run it

1. Loads every `.yaml` and `.rego` file under `<package-path>/policies/`.
2. Reads the manifest from stdin (or from `--manifest <file>`).
3. Evaluates each policy rule against each resource in the manifest.
4. Prints a summary of pass/fail per rule.
5. Exits 0 on no violations, non-zero on any violation.

## Usage

```
keramos policy check <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for check |
| `--manifest` | string | "" | rendered manifest file (default: stdin) |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Pipe a freshly-rendered manifest through policy check:

```sh
keramos template hello ./my-app | keramos policy check ./my-app
```

Run against a saved manifest file:

```sh
keramos template hello ./my-app > rendered.yaml
keramos policy check ./my-app --manifest rendered.yaml
```

CI gate — fail the build on any violation:

```sh
keramos template hello ./my-app | keramos policy check ./my-app || exit 1
```

## See also

- [`policy`](policy.md)
- [`policy list`](policy-list.md) — show what rules are declared
- [`lint`](lint.md) — broader package validation
- [Package anatomy: policies](../guides/packages.md)
