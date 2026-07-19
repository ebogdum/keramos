# keramos create

`keramos create` scaffolds a ready-to-edit keramos package — a working
Deployment + Service with values already wired in — in a new directory.

## When to use it

- You want a working starter you can render and edit immediately, not a
  menu of layouts.
- You want the batteries-included set: a helpers partial, notes, and a
  `.keramosignore` already in place.
- For a choice of workload shapes (webapp, batch, operator, blank) use
  [`keramos init`](init.md) instead.

## What happens

1. Creates a directory named after `<name>` (fails if it already exists).
2. Writes `keramos.yaml` (`name`, `version`, `description`) and a `values.yaml`
   seeded with `replicaCount`, `image`, and `service.port`.
3. Writes a `templates/` directory: `deployment.yaml`, `service.yaml`, a
   `_helpers.yaml` partial, and `notes.yaml`.
4. Writes a `.keramosignore` listing patterns to skip when packaging.
5. Prints `created package <name>/`.

The templates read the seeded values via `${values.*}`, so the package
renders and lints as-is. Edit the files, then run [`keramos lint`](lint.md) and
[`keramos template`](template.md).

## Usage

```
keramos create <name> [flags]
```

## Flags

Inherits the global flags.

## Worked example

Scaffold a package called `myapp`:

```sh
keramos create myapp
```

**OUTPUT:**

```
created package myapp/
```

**What you now have on disk:**

```
myapp/
├── .keramosignore
├── keramos.yaml
├── values.yaml
└── templates/
    ├── _helpers.yaml
    ├── deployment.yaml
    ├── notes.yaml
    └── service.yaml
```

`values.yaml` sets `replicaCount: 1`, `image.repository: nginx`, and
`service.port: 80`, and the templates reference those values, so the package
renders straight away:

```sh
cd myapp
keramos template .
```

Change `replicaCount` or the image in `values.yaml` and re-render to watch
the manifest update.

## See also

- [`init`](init.md) — scaffold from a chosen built-in template
- [`lint`](lint.md) — validate the scaffolded package
- [`template`](template.md) — render it to manifests
- [`values`](values.md) — how values files and `--set` overrides merge
