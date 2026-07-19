# keramos version

`keramos version` prints the version, git commit, and build date of the `keramos`
binary you are running.

## When to use it

- To confirm which build is installed before filing a bug or following
  version-specific docs.
- To capture the exact commit in CI logs so a run is reproducible.

## What happens

1. keramos prints a single line: its version, the git commit it was built from,
   and the date it was built.
2. The values come from the binary itself (stamped in at build time), so no
   cluster or network access is needed.

## Usage

```
keramos version [flags]
```

## Flags

Inherits the global flags (`--debug`, `--kube-context`, `--kubeconfig`,
`-n/--namespace`); version prints locally, so they have no effect.

## Worked example

```sh
keramos version
```

Output:

```
keramos version 1.4.0 (commit a1b2c3d, built 2026-07-15T10:04:22Z)
```

A build from source without stamped values prints placeholders instead:

```
keramos version dev (commit unknown, built unknown)
```

## See also

- [`env`](env.md) — the resolved paths and environment keramos is using
