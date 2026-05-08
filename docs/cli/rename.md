# keramos rename

## Synopsis

`keramos rename` changes a release's name in place. Every revision Secret stored in the cluster (`keramos.v1.<old>.v<rev>`) is copied to a new Secret carrying the new name (`keramos.v1.<new>.v<rev>`); the originals are then deleted (unless `--keep-old` is passed). The cluster resources the release manages — Deployments, Services, ConfigMaps, etc. — are *not* renamed; they continue to belong to the (now-renamed) release.

## When to use it

Use when a release was originally named badly (a typo, a stale convention, an environment label that no longer applies) and you want to correct the name without uninstalling and reinstalling. Common cases:

- The release was created with `keramos install backend-prod ./api` and you want to rename it to `api`.
- A re-platforming changed conventions, e.g. `app-team-a-redis` → `redis`.
- You're consolidating two namespaces' release names.

Note: resources whose names embed `${release.name}` in their templates will keep the *old* name until the next upgrade. To rename both the release and its resource names, follow `keramos rename` with `keramos upgrade <new-name> <package-path>` so the templates re-render with the new release name.

## What happens when you run it

1. Locates every revision Secret labelled `name=<old>` in the namespace.
2. Copies each revision to a new Secret with the canonical keramos name pattern (`keramos.v1.<new>.v<rev>`) and the `name=<new>` label.
3. Updates each new Secret's stored release record so its internal `name` field matches the new name.
4. Deletes the original Secrets unless `--keep-old` is set.
5. Prints the new release name and revision count on success.

## Usage

```
keramos rename <old> <new> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-h, --help` | bool | false | help for rename |
| `--keep-old` | bool | false | leave the old release revisions in place after copying |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | bool | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Examples

Rename a release in place:

```sh
keramos rename backend-prod api -n prod
```

Rename, keeping the old revisions for a safety period (you can manually `keramos uninstall <old> --keep-history` later):

```sh
keramos rename backend-prod api --keep-old -n prod
```

Rename, then re-render templates so resource names also reflect the new release name:

```sh
keramos rename backend-prod api -n prod
keramos upgrade api ./packages/api -n prod
```

## See also

- [`list`](list.md) — verify the release appears under its new name
- [`history`](history.md) — confirm every revision came across
- [`upgrade`](upgrade.md) — re-render so resource names follow the new release name
