# keramos show all

## Synopsis

`keramos show all` prints the package's metadata (`keramos.yaml`), default values (`values.yaml`), and README in one combined document. Convenient when surveying an unfamiliar package: you see identity, configuration surface, and human documentation without running three separate commands.

## When to use it

Use as a one-shot inspection of a package directory you don't yet know — typically a freshly-pulled package or one a teammate handed you. For just one slice (chart only, values only, README only), use the dedicated subcommand.

## What happens when you run it

1. Reads `<package-path>/keramos.yaml`, `<package-path>/values.yaml`, and `<package-path>/README.md`.
2. Concatenates them with `---` separators.
3. Prints to stdout.
4. No cluster contact, no network access.

## Usage

```
keramos show all <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for all |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Inspect a local package directory:

```sh
keramos show all ./my-app
```

Pull a package and inspect it before installing:

```sh
keramos pull my-app --repo https://charts.example.com --version 1.2.3 -d ./pulled --untar
keramos show all ./pulled/my-app
```

Pipe to a pager for browsing:

```sh
keramos show all ./my-app | less
```

## See also

- [`show`](show.md)
- [`show chart`](show-chart.md)
- [`show values`](show-values.md)
- [`show readme`](show-readme.md)
- [`show crds`](show-crds.md)
