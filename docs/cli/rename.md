---
title: "keramos rename"
parent: "CLI"
---
{% raw %}
# keramos rename

Give an existing release a new name, carrying its full revision history across.

## When to use it

- A release was named badly — a typo, a stale convention, or an environment
  label that no longer applies — and you want to fix the name without
  uninstalling and reinstalling.
- You are consolidating naming across namespaces, e.g. `backend-prod` → `api`.

Renaming touches only keramos's stored record of the release. The live resources it
manages — Deployments, Services, ConfigMaps — keep running untouched and still
belong to the release under its new name. To make resource names that embed the
release name follow along, run [`keramos upgrade`](upgrade.md) with the new name
afterward so the templates re-render.

## What happens

1. Reads every stored revision of `<old>` from the namespace's release storage.
2. Refuses if `<new>` already has revisions, or if `<old>` and `<new>` are the
   same name.
3. Copies each revision to `<new>`, rewriting the recorded name. If any copy
   fails, the already-copied revisions are rolled back so `<old>` stays intact.
4. Deletes the original `<old>` revisions — unless `--keep-old` is set.

This mutates keramos's release storage in the cluster, so a reachable cluster is
required. The Kubernetes workloads themselves are not modified.

## Usage

```
keramos rename <old> <new> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--keep-old` | — | false | leave the old release revisions in place after copying, instead of deleting them |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Worked example

**INPUT — a release named `backend-prod` with three revisions:**

```sh
keramos list -n prod
```

```
NAME           NAMESPACE   REVISION   STATUS     UPDATED
backend-prod   prod        3          deployed   2026-07-18 08:22:10
```

**Rename it to `api`:**

```sh
keramos rename backend-prod api -n prod
```

```
copied 3 revisions from backend-prod to api
deleted 3 revisions of backend-prod
```

**OUTPUT — the release now answers to `api`, history preserved:**

```sh
keramos list -n prod
```

```
NAME   NAMESPACE   REVISION   STATUS     UPDATED
api    prod        3          deployed   2026-07-18 08:22:10
```

```sh
keramos history api -n prod
```

```
REVISION   STATUS       UPDATED               DESCRIPTION
1          superseded   2026-07-17 14:03:55   Install complete
2          superseded   2026-07-18 07:55:41   Upgrade complete
3          deployed     2026-07-18 08:22:10   Upgrade complete
```

Pass `--keep-old` to leave `backend-prod` in place as well, then remove it
yourself once you are satisfied the rename is correct.

## See also

- [`list`](list.md) — confirm the release appears under its new name
- [`history`](history.md) — verify every revision came across
- [`upgrade`](upgrade.md) — re-render so resource names follow the new name
{% endraw %}
