# Hooks in templates

A hook is a Job- or Pod-shaped manifest that keramos runs at a specific lifecycle point. Hooks live under `hooks/` (or `tests/`) in a package; the YAML files use the same template engine as `templates/` — every `${...}` expression and every YAML control-flow directive works. The difference from a regular template is that the manifest carries `$`-prefixed directives at the top level that keramos strips before applying.

For the broader picture (when each event fires, weight ordering, delete policies), see the [Hooks guide](../guides/hooks.md).

## Top-level directives

| Directive | Type | Required | Description |
|---|---|---|---|
| `$hook` | string (comma list) | yes | Events to participate in. |
| `$weight` | integer | no | Order within an event. Default `0`. Lower runs first. |
| `$delete-policy` | string (comma list) | no | When to delete the hook resource. Default `before-hook-creation`. |
| `$timeout` | duration string | no | Per-hook timeout. Default = `--timeout`. |
| `$preserve-on-uninstall` | bool | no | Keep the hook after release uninstall. Defaults to `true` for tests. |

Directives sit at the top of the YAML document; they are stripped before the manifest is sent to the cluster.

## Minimal hook

**Input — `hooks/post-install.yaml`:**

```yaml
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

**What keramos does at install time:**

1. Render `templates/` and apply.
2. After the manifest is Ready, render `hooks/post-install.yaml`.
3. Strip `$hook`, `$weight`, `$delete-policy`.
4. Apply the resulting `Job`.
5. Wait for the Job to complete (or hit `$timeout`).
6. On success: per `hook-succeeded`, delete the Job.

## Multiple events on one hook

```yaml
$hook: pre-install,pre-upgrade
```

Same hook fires before install AND before upgrade. The body is identical. Useful for "always-run-before-mutation" hooks.

## Multi-document files

A hook file can have multiple YAML documents. The first document carries the directives; subsequent docs share them.

**`hooks/migrate.yaml`:**

```yaml
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

Both the ConfigMap and the Job are treated as part of the same hook; the ConfigMap is applied first (because keramos's resource ordering puts it before workloads), the Job runs against it, then both are deleted on success.

## Test hooks

Test hooks fire only when the operator runs `keramos test <release>` — not during install or upgrade. The convention is `tests/<name>.yaml`, which is shorthand for `hooks/<name>.yaml` with `$hook: test`.

**Input — `tests/connection.yaml`:**

```yaml
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

**Operator runs:**

```sh
keramos test my-app
# → renders hooks/*.yaml + tests/*.yaml with $hook: test
# → applies each in weight order
# → waits for Pod completion
# → succeed/fail based on container exit code
```

A non-zero exit from the test container is a test failure. `keramos test` exits non-zero if any test fails.

## Per-revision hooks for rollback

When a rollback re-applies an old revision's manifest, it also re-runs that revision's `pre-rollback` and `post-rollback` hooks — using the hook templates as they were when that revision was installed, not the current ones.

Keramos persists the rendered hook manifests inside each revision's release record:

```
keramos.v1.<release>.v<rev> Secret
├── manifest          ← gzipped + base64 rendered manifest
├── values            ← merged values used
├── hookTemplates     ← per-revision rendered hook YAMLs (filename → body)
└── hooks             ← previous run's hook results
```

This persistence is automatic — package authors don't write any extra YAML — but it shapes how hooks should be designed: **idempotent** (rolling back to v3 will re-run v3's hooks), **self-describing** (hooks should not assume v4's CRDs are installed), and **scoped to their revision** (no global state across revisions).

## Rendering inside hooks

Hook files are rendered by the same engine as `templates/`. So:

```yaml
$hook: pre-install
$weight: 1
$delete-policy: hook-succeeded
$timeout: 5m

apiVersion: batch/v1
kind: Job
metadata:
  name: ${release.name}-${release.revision}-precheck
  labels:
    $include: common.labels         # YAML-level partial
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        $each: ${values.precheck.containers}
        $yield:
          name: ${$item.name}
          image: ${$item.image}
          command: ${$item.command}
```

`${...}` expressions, `$if`, `$each`, `$switch`, `$include` — all of them work the same way as in regular templates.

## Idioms

### Single-shot DB migration with idempotent script

```yaml
$hook: post-install,post-upgrade
$weight: 5
$delete-policy: hook-succeeded,before-hook-creation
$timeout: 30m

apiVersion: batch/v1
kind: Job
metadata:
  name: ${release.name}-migrate-${release.revision}
spec:
  backoffLimit: 0
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: migrate
          image: ${values.image.repository}:${values.image.tag}
          command: ["/app/migrate", "--idempotent"]
          envFrom:
            - secretRef:
                name: ${values.database.existingSecret}
```

The `${release.revision}` in the Job name keeps every revision's migration as its own discoverable artifact (it's deleted by `hook-succeeded`, but if it fails it stays for inspection with the revision number visible).

### Wait-for-prerequisite hook

```yaml
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
        - name: wait
          image: bitnami/kubectl:1.28
          command:
            - sh
            - -c
            - |
              for i in $(seq 60); do
                kubectl -n cert-manager get deployment cert-manager -o jsonpath='{.status.readyReplicas}' | grep -q '^[1-9]' && exit 0
                sleep 2
              done
              exit 1
```

### Backup-before-uninstall (preserved)

```yaml
$hook: pre-delete
$weight: 1
$delete-policy: never               # preserve evidence
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
          command:
            - sh
            - -c
            - |
              pg_dump $DATABASE_URL > /backup/${release.name}-${HOSTNAME}.sql
          envFrom:
            - secretRef:
                name: ${values.database.existingSecret}
          volumeMounts:
            - name: backup
              mountPath: /backup
      volumes:
        - name: backup
          persistentVolumeClaim:
            claimName: ${release.name}-backup
```

`$delete-policy: never` keeps the Job (and the backup PVC) after uninstall — the backup outlives the release.

## Inspecting hooks

```sh
keramos get hooks <release>                  # rendered hook manifests + last results
keramos get hooks <release> --revision 3
keramos history <release>                    # hook outcomes per revision
keramos status <release>                     # current hook section
```

`keramos test --keep <release>` overrides `hook-succeeded` to leave the test artifacts in the cluster for debugging.
