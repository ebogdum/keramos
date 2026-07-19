---
title: "keramos dev"
parent: "CLI"
---
{% raw %}
# keramos dev

`keramos dev` watches a package directory and re-renders it on every file
change — the live inner loop for authoring templates.

## When to use it

- You're editing templates or values and want to see the rendered manifest
  update as you save.
- You want to catch render errors the moment they're introduced.
- It renders locally only (a client-side dry-run) — it never touches a
  cluster. To preview against stored state use [`keramos plan`](plan.md).

## What happens

1. Walks every file under `<package-path>` and polls it every `--interval`
   (default 500ms), detecting changes by file size and modification time.
2. On the first render it prints `--- initial render ---` followed by the
   full rendered manifest.
3. On each later change it prints `--- diff ---` and a line-by-line diff
   (`-` old, `+` new) against the previous render — only what changed.
4. If a render fails it prints `--- render error ---` and the message, then
   keeps watching so the next save can fix it.

Press Ctrl-C to stop. `--profile`, `-f`, and `--set` are applied to every
render, exactly as an install would resolve them.

## Usage

```
keramos dev <package-path> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--interval` | duration | 500ms | how often to poll the directory for changes; raise it for large packages |
| `--profile` | string | — | profile to apply on every render |
| `-f, --values` | stringArray | — | values file applied on every render (repeatable) |
| `--set` | stringArray | — | `key=value` applied on every render (repeatable) |

## Worked example

Start watching the package, then edit a value in another terminal. With
`keramos dev ./myapp` running, change `replicaCount: 1` to `replicaCount: 3` in
`myapp/values.yaml` and save.

**What the watch loop prints — first the initial render:**

```
--- initial render ---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: myapp
  name: myapp
spec:
  replicas: 1
  ...
```

**Then, the instant you save the edit, only the change:**

```
--- diff ---
-   replicas: 1
+   replicas: 3
```

The `-`/`+` pair maps straight back to your edit: `replicaCount: 3` flows
through the template to `spec.replicas: 3`. A save that renders identically
prints nothing new; a broken edit prints `--- render error ---` instead.

## See also

- [`template`](template.md) — one-shot render to stdout
- [`debug`](debug.md) — render with a resolution trace
- [`lint`](lint.md) — validate the package
- [`values`](values.md) — how values files and `--set` overrides merge
{% endraw %}
