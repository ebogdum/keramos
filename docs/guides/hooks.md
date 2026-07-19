---
title: "Hooks"
parent: "Guides"
---
{% raw %}
# Hooks

Hooks are Job- or Pod-shaped resources that keramos runs at specific points in a
release's lifecycle: before install, after upgrade, before delete, and so on.
Use them to run a database migration before new replicas come up, back up a
database before uninstalling, or gate an install on a dependency being ready.

Hooks are applied separately from the regular manifest, in weight order, with
their own cleanup policies. This guide covers when each hook fires, how to order
and clean them up, and the patterns that hold.

## Where hooks live

Hooks are YAML files under `hooks/`. The file's **name** selects the lifecycle
event: `pre-install.yaml`, `post-upgrade.yaml`, or a descriptive name that
starts with the event (`post-install-migrate.yaml`). Each file is an ordinary
manifest — usually a `Job` — plus optional `$`-prefixed directives at the top of
the document:

```yaml
# hooks/pre-install.yaml
$hook: pre-install
$hookWeight: 1
$hookDeletePolicy: hook-succeeded

apiVersion: batch/v1
kind: Job
metadata:
  name: ${release.name}-preinstall
  namespace: ${release.namespace}
spec:
  backoffLimit: 0
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: pre
          image: ${values.image}
          command: ["sh", "-c", "echo pre-install ran"]
```

The directives are stripped before the manifest is applied — `kubectl get job`
won't show them.

> The directives are `$hook`, `$hookWeight`, `$hookDeletePolicy`, and
> `$hookTimeout`. Equivalently, use the bracket form
> `$hook: {phase: pre-install, weight: 1, deletePolicy: hook-succeeded, timeout: 10m}`.

## Hook events

| Event | When it fires |
|---|---|
| `pre-install` | Before any `templates/` resource is applied during a fresh install. |
| `post-install` | After every `templates/` resource is applied and ready. |
| `pre-upgrade` | Before an upgrade applies the new manifest. |
| `post-upgrade` | After an upgrade's apply completes. |
| `pre-rollback` | Before `keramos rollback` re-applies an old revision. |
| `post-rollback` | After a successful rollback. |
| `pre-delete` | Before `keramos uninstall` removes resources. |
| `post-delete` | After uninstall has removed resources. |

These eight events are the complete set. There is no `test` event — on-demand
tests live in `tests/` and are covered [below](#tests).

## Weights

`$hookWeight` orders multiple hooks of the same event; lower runs first. The
default is `0`.

```yaml
# hooks/pre-install-seed.yaml  →  $hook: pre-install, $hookWeight: 1
# hooks/pre-install-data.yaml  →  $hook: pre-install, $hookWeight: 2
```

Weight is per-event: a `pre-install` weight 5 has no relationship to a
`post-install` weight 5. Ties break by filename order.

## Delete policies

`$hookDeletePolicy` controls when keramos deletes a hook's resources after it
finishes:

| Policy | Meaning |
|---|---|
| `before-hook-creation` | Delete any prior instance of this hook before creating the new one. Use for hooks that re-run on retry. |
| `hook-succeeded` | Delete the hook resource after it completes successfully. Failed hooks stay for inspection. |
| `hook-failed` | Delete the hook resource on failure. |

When you set no policy, the hook resource is **kept** after it finishes. Combine
policies as a comma-separated list:

```yaml
$hookDeletePolicy: hook-succeeded,before-hook-creation
```

## Timeouts

`$hookTimeout` sets a per-hook timeout as a Go duration (`30s`, `5m`, `1h`); the
default is the operation's `--timeout` (5 minutes). A hook that exceeds its
timeout is treated as failed. The CLI `--hook-timeout` flag caps every hook's
timeout.

```yaml
$hook: post-install
$hookTimeout: 10m
```

## Hook bodies are templates

Hooks render through the same engine as `templates/`, with the same lowercase
namespaces — `${values.*}`, `${release.*}`, `${package.*}`, `${Files.*}`, and
`$include`:

```yaml
spec:
  template:
    spec:
      containers:
        - name: migrate
          image: ${values.image.repository}:${values.image.tag}
          env:
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: ${values.database.existingSecret}
                  key: url
```

## Multi-document files

A hook file can hold multiple YAML documents. The directives on the first
document apply to the file; later documents (e.g. a ConfigMap the Job mounts)
share the event:

```yaml
# hooks/pre-install-migrate.yaml
$hook: pre-install
$hookWeight: 5
$hookDeletePolicy: hook-succeeded

apiVersion: v1
kind: ConfigMap
metadata:
  name: ${release.name}-migrate-script
  namespace: ${release.namespace}
data:
  migrate.sh: |
    #!/bin/sh
    psql -f /migrations/0001.sql
---
apiVersion: batch/v1
kind: Job
metadata:
  name: ${release.name}-migrate
  namespace: ${release.namespace}
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: migrate
          image: postgres:16
          command: [/scripts/migrate.sh]
          volumeMounts:
            - { name: scripts, mountPath: /scripts }
      volumes:
        - name: scripts
          configMap: { name: ${release.name}-migrate-script }
```

## Inspecting hook results

Keramos stores each hook's rendered manifest and last-run outcome in the release
record:

```sh
keramos get hooks hello -n keramos-quickstart
```

```
NAME             KIND    STATUS
hk-preinstall    Job     succeeded
```

Add `--revision N` for a specific revision. See [`keramos get`](../cli/get.md).

## Persistent hooks for rollback

When a release rolls back to an older revision, keramos re-runs **that revision's**
hooks, not the current templates — it stores each revision's rendered hook
manifests in the release record. So if you ship a new hook in v3 and roll back
to v2, the v2 hook re-runs. Design hooks to be idempotent so a re-run is safe.

## Tests

Test manifests live in `tests/` — ordinary Pods, no `$hook` directive. They
never run during install or upgrade; keramos renders and stores them at
install/upgrade time and runs them on demand:

```yaml
# tests/connection.yaml
apiVersion: v1
kind: Pod
metadata:
  name: ${release.name}-connection-test
  namespace: ${release.namespace}
spec:
  restartPolicy: Never
  containers:
    - name: probe
      image: curlimages/curl:8.4.0
      command:
        - sh
        - -c
        - curl -fsS http://${release.name}.${release.namespace}.svc.cluster.local/health
```

```sh
keramos test hello -n keramos-quickstart
```

```
Running tests for release hello (revision 1)...
  TEST: connection.yaml
    PASS
All tests passed.
```

A non-zero exit from a test Pod is a failure; `keramos test` exits non-zero when any
test fails. `keramos test --parallel N` runs N tests concurrently, `--retries N`
retries failures, and `--logs` prints Pod logs. See [`keramos test`](../cli/test.md).

## Idioms

### Backup before uninstall

```yaml
# hooks/pre-delete-backup.yaml
$hook: pre-delete
$hookWeight: 1
$hookTimeout: 30m
# no delete policy → the backup Job is kept for evidence

apiVersion: batch/v1
kind: Job
metadata:
  name: ${release.name}-backup-${release.revision}
  namespace: ${release.namespace}
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: backup
          image: postgres:16
          command: ["sh", "-c", "pg_dump $DATABASE_URL > /backup/dump.sql"]
```

### Gate an install on dependencies

```yaml
# hooks/pre-install-check.yaml
$hook: pre-install
$hookWeight: 1
$hookDeletePolicy: before-hook-creation,hook-succeeded
$hookTimeout: 2m

apiVersion: batch/v1
kind: Job
metadata:
  name: ${release.name}-check
  namespace: ${release.namespace}
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: check
          image: bitnami/kubectl:1.28
          command:
            - sh
            - -c
            - kubectl -n cert-manager get deploy cert-manager -o jsonpath='{.status.readyReplicas}' | grep -q '^[1-9]'
```

Pairs naturally with `requires:` in `keramos.yaml`.

## Common errors

- **Hook timeout exceeded** — raise `$hookTimeout` or `--hook-timeout`, and make
  sure the command actually terminates.
- **A hook from a previous failed run is still there** — set
  `$hookDeletePolicy: before-hook-creation` so the next run clears it first.
- **A `pre-install` hook can't reach the Service** — install hooks fire before
  the manifest is applied. Use `post-install` if the hook needs the deployed
  resources.

## See also

- [Package anatomy](packages.md) — `hooks/` and `tests/` in context.
- [`keramos test`](../cli/test.md) and [`keramos get`](../cli/get.md).
- [Template expressions](../templates/expressions.md) — the hook body language.
{% endraw %}
