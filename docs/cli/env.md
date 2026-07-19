# keramos env

`keramos env` prints the resolved paths and settings keramos uses, one
`KEY="value"` per line.

## When to use it

- Confirm which cache, config, and data directories keramos reads before you
  debug a path or plugin problem.
- Check the namespace and kubeconfig keramos resolves from your flags and
  environment.

## What happens

1. Resolves each value from its environment variable, falling back to the OS
   default when unset (for example `KERAMOS_CACHE_HOME` defaults to the user cache
   directory, and derived paths like `KERAMOS_REPOSITORY_CACHE` sit under it).
2. Resolves the namespace in order: `--namespace` flag → `KERAMOS_NAMESPACE` →
   `HELM_NAMESPACE` → `default`.
3. Sorts the keys and prints them as quoted `KEY="value"` lines.

No cluster is contacted.

## Usage

```
keramos env [flags]
```

## Flags

Inherits the global flags. `-n, --namespace` changes the reported
`KERAMOS_NAMESPACE`.

## Worked example

**INPUT** — no overrides set. Run `keramos env`:

```
KERAMOS_BIN="/usr/local/bin/keramos"
KERAMOS_CACHE_HOME="/Users/you/Library/Caches/keramos"
KERAMOS_CONFIG_HOME="/Users/you/Library/Application Support/keramos"
KERAMOS_DATA_HOME="/Users/you/.local/share/keramos"
KERAMOS_KUBECONFIG=""
KERAMOS_KUBECONTEXT=""
KERAMOS_NAMESPACE="default"
KERAMOS_PLUGINS="/Users/you/.local/share/keramos/plugins"
KERAMOS_REGISTRY_CONFIG="/Users/you/Library/Application Support/keramos/registry.json"
KERAMOS_REPOSITORY_CACHE="/Users/you/Library/Caches/keramos/repository"
KERAMOS_REPOSITORY_CONFIG="/Users/you/Library/Application Support/keramos/repositories.yaml"
```

**Now set values and watch the output follow.** With `KERAMOS_CACHE_HOME` set, the
derived repository cache moves with it (`KERAMOS_CACHE_HOME=/data/cache keramos env`):

```
KERAMOS_CACHE_HOME="/data/cache"
KERAMOS_REPOSITORY_CACHE="/data/cache/repository"
```

Pass `-n staging` and the resolved namespace changes
(`keramos -n staging env`):

```
KERAMOS_NAMESPACE="staging"
```

Each printed line reflects exactly what you set, so you can verify keramos reads
the values you expect.

## See also

- [`config`](config.md) — build a values file interactively
- [`version`](version.md) — print the keramos build version
