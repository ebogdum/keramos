# keramos init

`keramos init` scaffolds a new package from a built-in template chosen with
`-t` — a curated starting point for a class of workload.

## When to use it

- You know the shape you're packaging and want defaults tuned for it.
- You want a specific starter: `webapp` (Deployment + Service + ConfigMap),
  `batch` (a CronJob worker), `operator` (a CRD + controller), or `blank`
  (the smallest valid package).
- For the fixed nginx starter instead of a template menu, use
  [`keramos create`](create.md).

## What happens

1. Creates `<name>/` under `--dest` (fails if the target already exists or
   the template name is unknown).
2. Copies the chosen template's files, substituting the package name where
   the template references it. Every template writes `keramos.yaml`,
   `values.yaml`, and a `templates/` directory; richer templates also add
   `values.schema.json`, a `tests/` directory, or a `crds/` directory.
3. Prints `Initialised <template> package at <path>` followed by the next
   commands to run.

## Usage

```
keramos init <name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-t, --template` | string | "blank" | which starter to scaffold: `webapp`, `batch`, `operator`, or `blank` |
| `--dest` | string | "." | parent directory to create `<name>/` in |

## Worked example

Scaffold a web-app package:

```sh
keramos init webui -t webapp
```

**OUTPUT:**

```
Initialised webapp package at webui
Next:
  cd webui
  keramos lint .
  keramos template . -o yaml
```

**What you now have on disk:**

```
webui/
├── keramos.yaml
├── values.yaml
├── values.schema.json
├── templates/
│   ├── configmap.yaml
│   ├── deployment.yaml
│   └── service.yaml
└── tests/
    └── connection.yaml
```

The `blank` template is leaner — `keramos init mini` gives only `keramos.yaml`,
`values.yaml`, and `templates/configmap.yaml`. Follow the printed steps to
lint and render whichever template you picked.

## See also

- [`create`](create.md) — the fixed nginx starter (no template choice)
- [`lint`](lint.md) — validate the scaffolded package
- [`template`](template.md) — render it to manifests
- [`install`](install.md) — install the package as a release
