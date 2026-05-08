# Hooks

Hooks are Job- or Pod-shaped resources that keramos runs at specific points in a release's lifecycle: before install, after install, before upgrade, before delete, on operator-invoked test, and so on. Hooks are how you express "run a database migration before the new replicas come up", "run a cluster smoke test after install completes", "back up the database before uninstalling".

Hooks are **not** part of the regular manifest — they're applied separately, in weight order, with their own lifecycle and cleanup policies. This guide covers when each hook fires, how to weight and clean them up, and the patterns that hold.

## Where hooks live

Hooks are YAML files under `hooks/` in a package. Each file is an ordinary Kubernetes manifest (typically a `Job`, but `Pod` works) plus one or more `$`-prefixed directives at the top of the document:

```yaml
# hooks/post-install-migrate.yaml
$hook: post-install
$weight: 5
$delete-policy: hook-succeeded

apiVersion: batch/v1
kind: Job
metadata:
  name: ${release.name}-migrate
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: migrate
          image: ${values.image.repository}:${values.image.tag}
          command: ["/migrate.sh"]
```

The directives are stripped before the manifest is applied to the cluster — `kubectl get job ${release.name}-migrate -o yaml` won't show them.

## Hook events

| Event | When it fires |
|---|---|
| `pre-install` | Before any `templates/` resource is applied during a fresh install. |
| `post-install` | After every `templates/` resource is applied and Ready (when `wait` is on). |
| `pre-upgrade` | Before an upgrade applies the new manifest. |
| `post-upgrade` | After an upgrade's apply completes. |
| `pre-rollback` | Before `keramos rollback` re-applies an old revision. |
| `post-rollback` | After a successful rollback. |
| `pre-delete` | Before `keramos uninstall` removes resources. |
| `post-delete` | After uninstall has removed resources. The release record is deleted *after* this hook completes. |
| `test` | When the operator runs `keramos test <release>`. Not run during install/upgrade. |

A single hook can fire on multiple events:

```yaml
$hook: pre-install,pre-upgrade
```

## Weights

`$weight` orders multiple hooks of the same event. Lower weight runs first. Default is `0`.

```yaml
# hooks/seed-config.yaml
$hook: pre-install
$weight: 1

# hooks/seed-data.yaml
$hook: pre-install
$weight: 2
```

Weight is per-event; a `pre-install` weight 5 has no relationship to a `post-install` weight 5.

Within the same event and weight, declared file order is honoured (lexically by file name).

## Delete policies

`$delete-policy` controls when keramos deletes a hook's resources after it finishes. Policies are comma-separated; multiple can apply.

| Policy | Meaning |
|---|---|
| `before-hook-creation` | Delete any prior instance of this hook before creating the new one. **Default.** Useful for `pre-install` hooks that re-run on retry. |
| `hook-succeeded` | Delete the hook resource after the hook completes successfully. Failed hooks stay for inspection. |
| `hook-failed` | Delete the hook resource even on failure. |
| `never` | Never delete (operator must clean up manually). |

```yaml
$hook: post-install
$weight: 5
$delete-policy: hook-succeeded,before-hook-creation
```

## Timeouts

`$timeout` sets a per-hook timeout. The default is keramos's overall `--timeout` (5 minutes when not set). Format is a Go duration: `30s`, `5m`, `1h`.

```yaml
$hook: post-install
$timeout: 10m
```

If a hook exceeds its timeout, keramos treats it as failed. Combined with `--atomic`, the install rolls back.

`keramos <command> --hook-timeout <duration>` overrides the per-hook timeout from the CLI.

## Hook directives summary

| Directive | Type | Description |
|---|---|---|
| `$hook` | string (comma list) | Events this hook participates in. Required. |
| `$weight` | integer | Ordering within an event. Default `0`. |
| `$delete-policy` | string (comma list) | When to delete the hook resource. Default `before-hook-creation`. |
| `$timeout` | duration string | Per-hook timeout. Default = `--timeout`. |
| `$preserve-on-uninstall` | bool | If `true`, hooks are not deleted when the release is uninstalled. Default `false` for non-test hooks, `true` for test hooks. |

## Hook bodies are templates

Hooks are rendered through the same engine as `templates/` — `${.Values}`, `${.Release}`, `${.Files}`, `${include}` all work. So you can parameterise the migration's image:

```yaml
spec:
  template:
    spec:
      containers:
        - name: migrate
          image: ${values.migrations.image | default (printf "%s:%s" .Values.image.repository .Values.image.tag)}
          env:
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: ${values.database.existingSecret}
                  key: url
```

## Persistent hooks for rollback

When a release rolls back to an older revision, keramos re-runs that revision's `pre-rollback` and `post-rollback` hooks — not the **current** hook templates. To make this work, keramos stores every revision's rendered hook manifests inside the release record:

```
release record (Secret keramos.v1.<release>.v<rev>)
├── manifest                  ← gzipped rendered manifest
├── values                    ← merged values used
├── hooks                     ← per-revision hook results
└── hookTemplates             ← per-revision rendered hook YAMLs (filename -> body)
```

This is invisible to the package author — the persistence happens automatically — but it means that if you ship a new hook in v3 and roll back to v2, the v2 hook (not the v3 hook) re-runs. Designing hooks to be idempotent and self-describing helps.

## Test hooks

Test hooks are like any other hook except they don't fire automatically. The operator runs:

```sh
keramos test my-app
```

which renders and applies every hook with `$hook: test`. The default `--delete-policy` for test hooks is `hook-succeeded` so the cluster doesn't accumulate Pods after a passing test run. `keramos test --keep` overrides this to leave artifacts for debugging.

```yaml
# tests/connection.yaml — equivalent to hooks/connection.yaml with $hook: test
$hook: test
$delete-policy: hook-succeeded

apiVersion: v1
kind: Pod
metadata:
  name: ${release.name}-connection-test
spec:
  restartPolicy: Never
  containers:
    - name: probe
      image: curlimages/curl:8.4.0
      command:
        - sh
        - -c
        - |
          curl -fsS http://${release.name}.${release.namespace}.svc.cluster.local/health
```

A non-zero exit from the test Pod's container is a test failure. `keramos test` exits non-zero when any test Pod fails.

`keramos test --parallel N` runs up to N tests concurrently; `keramos test --retries N` retries each failed test up to N times.

## Multi-document files

Hooks files can contain multiple YAML documents. The first document carries the `$hook:` directive; subsequent documents share it. This is useful for ConfigMaps the hook depends on.

```yaml
# hooks/migrate.yaml
$hook: pre-install
$weight: 5
$delete-policy: hook-succeeded

apiVersion: v1
kind: ConfigMap
metadata:
  name: ${release.name}-migrate-script
data:
  migrate.sh: |
    #!/bin/sh
    psql -f /migrations/0001.sql
---
apiVersion: batch/v1
kind: Job
metadata:
  name: ${release.name}-migrate
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: migrate
          image: postgres:16
          command: [/scripts/migrate.sh]
          volumeMounts:
            - name: scripts
              mountPath: /scripts
      volumes:
        - name: scripts
          configMap:
            name: ${release.name}-migrate-script
```

## Idioms

### Wait-for-CRDs

The `crds/` directory is the better choice (keramos waits for `Established=true` automatically). But if you're shipping a CRD instance through a hook, do this:

```yaml
$hook: post-install
$weight: 1
$delete-policy: hook-succeeded

apiVersion: batch/v1
kind: Job
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: wait
          image: bitnami/kubectl:1.28
          command:
            - sh
            - -c
            - |
              for i in $(seq 60); do
                kubectl get crd widgets.example.com >/dev/null 2>&1 && exit 0
                sleep 2
              done
              exit 1
```

### Backup-before-uninstall

```yaml
# hooks/pre-delete-backup.yaml
$hook: pre-delete
$weight: 1
$delete-policy: never           # keep the backup-job evidence
$timeout: 30m

apiVersion: batch/v1
kind: Job
metadata:
  name: ${release.name}-backup-${release.revision}
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: backup
          image: postgres:16
          command: ["sh", "-c", "pg_dump $DATABASE_URL > /backup/${HOSTNAME}.sql"]
```

`$delete-policy: never` means the backup Job sticks around after uninstall. The release record is deleted, the namespace is kept (assuming you didn't `--delete-namespaces`), and the backup PVC is reachable.

### Cross-namespace install gating

```yaml
# hooks/pre-install-check-deps.yaml
$hook: pre-install
$weight: 1
$delete-policy: before-hook-creation,hook-succeeded
$timeout: 2m

apiVersion: batch/v1
kind: Job
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
            - |
              kubectl -n cert-manager get deployment cert-manager -o jsonpath='{.status.readyReplicas}' | grep -q '^[1-9]' || exit 1
              kubectl -n external-dns get deployment external-dns -o jsonpath='{.status.readyReplicas}' | grep -q '^[1-9]' || exit 1
```

A quick sanity check that other releases this one depends on are actually ready before installing. Pairs naturally with `requires:` declarations in `keramos.yaml`.

## Inspecting hook results

```sh
keramos get hooks <release>                 # rendered hook manifests + last-run results
keramos get hooks <release> --revision 3    # for revision 3
keramos history <release>                   # hook outcomes per revision
```

`keramos status` includes a hook section showing the last run's outcome.

## Common errors

- **Hook timeout exceeded.** Increase `$timeout` or pass `--hook-timeout` from the CLI; verify your hook's command actually completes (long migrations need a real upper bound).
- **Hook still exists from a previous failed run.** Either set `$delete-policy: before-hook-creation` (the default) or run `keramos uninstall <release>` first; failed hooks otherwise stay for inspection.
- **`pre-install` hook can't reach the deployed Service** — install hooks fire before the manifest is applied. Use `post-install` if your hook needs the service.
