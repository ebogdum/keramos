# keramos policy

## Synopsis

`keramos policy` evaluates the guardrail rules a package ships in its
`policies/` directory against a rendered Kubernetes manifest. Use it to catch
disallowed images, missing resource limits, absent labels, and similar
mistakes before the manifests reach a cluster.

The rules live in `<package-path>/policies/*.yaml`. Each rule has a `name`, a
`severity` (`deny` fails the check, `warn` only prints), an optional `match`
selector (`kinds`, `namespaces`, `names`, `apiVersion`), and `require` /
`forbid` predicates such as `imageRegistries`, `imageNotTagged`,
`resourceLimits`, `minReplicas`, `labelKeys`, and `fields`.

## Subcommands

| Command | What it does |
|---|---|
| [`check`](policy-check.md) | Evaluate the package's rules against a manifest and pass or fail. |
| [`list`](policy-list.md) | Print the rules a package declares, with their severities. |

## Usage

```
keramos policy [command]
```

Render a package and check it in one pipeline:

```sh
keramos template ./mychart | keramos policy check ./mychart
```

List the rules a package enforces:

```sh
keramos policy list ./mychart
```

## See also

- [`policy check`](policy-check.md) — run the rules against a manifest
- [`policy list`](policy-list.md) — show the declared rules
- [`template`](template.md) — render the manifest you pipe into `check`
- [`install`](install.md) — apply the package once it passes
- [`package verify`](package-verify.md) — verify a package's signature
