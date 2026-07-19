---
title: "Hooks in templates"
parent: "Templates"
---
{% raw %}
# Hooks in templates

A hook is a Job- or Pod-shaped manifest that keramos runs at a specific lifecycle
point. Hooks live in a package's `hooks/` directory and are rendered by the same
engine as `templates/` — every `${...}` expression and control-flow directive
works. What makes a file a hook is its location and its lifecycle phase, plus
optional `$`-prefixed directives that keramos strips before applying.

For when each event fires and how ordering and delete policies play out at
runtime, see the [Hooks guide](../guides/hooks.md).

## The phase comes from the filename

Name a hook file after its lifecycle event and keramos picks the phase up from the
name — no directive required:

```
hooks/pre-install.yaml
hooks/post-upgrade.yaml
hooks/pre-install-10.yaml   # a numeric suffix is allowed
```

The valid phases are:

| Phase | Fires |
|---|---|
| `pre-install` / `post-install` | around install |
| `pre-upgrade` / `post-upgrade` | around upgrade |
| `pre-delete` / `post-delete` | around uninstall |
| `pre-rollback` / `post-rollback` | around rollback |

Each file targets **one** phase. To run the same work before install *and*
before upgrade, ship two files (or share the body through an `$include`
partial).

## The `$hook` directive

Add a `$hook` directive only when you need to set weight, delete policy, or
timeout, or when the filename doesn't convey the phase. Two forms:

**Map form** — everything in one place:

```yaml
$hook:
  phase: pre-install
  weight: 5
  deletePolicy: hook-succeeded
  timeout: 5m
```

**String form** with flat siblings:

```yaml
$hook: pre-install
$hookWeight: 5
$hookDeletePolicy: hook-succeeded
$hookTimeout: 5m
```

| Field | Type | Default | Meaning |
|---|---|---|---|
| `phase` | string | filename | one lifecycle event |
| `weight` | int | `0` | order within a phase; lower runs first |
| `deletePolicy` | string | none | when to delete the hook resource |
| `timeout` | duration | `5m` | how long to wait for the hook |

`deletePolicy` takes **one** value (not a list):

- `before-hook-creation` — delete a prior instance before creating this one.
- `hook-succeeded` — delete after the hook succeeds.
- `hook-failed` — delete after the hook fails.

With no policy set, keramos keeps the hook resource. Directives are stripped before
the manifest is applied.

## A migration hook

**`hooks/post-install.yaml`:**

```yaml
$hook: post-install
$hookWeight: 5
$hookDeletePolicy: hook-succeeded

apiVersion: batch/v1
kind: Job
metadata:
  name: ${release.name}-migrate-${release.revision}
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: migrate
          image: ${values.image.repository}:${values.image.tag}
          command: ["/app/migrate", "--idempotent"]
```

**Rendered body** (`--release-name hello`, `image: {repository: myapp, tag: 1.4.2}`):

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: hello-migrate-1
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: migrate
          image: myapp:1.4.2
          command: ["/app/migrate", "--idempotent"]
```

At install time keramos renders the file, strips the directives, applies the Job,
waits for it to complete (or hit `timeout`), then deletes it per
`hook-succeeded`.

## Multi-document hook files

A hook file can hold several YAML documents. Put the `$hook` directive on one
document; all documents in the file share the phase and run together.

```yaml
$hook: pre-install
$hookDeletePolicy: hook-succeeded

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
```

## Rendering inside hooks

Hooks use the full engine — `${...}`, `$if`, `$each`, `$switch`, and `$include`
all behave as in `templates/`:

```yaml
$hook: pre-install
$hookWeight: 1
$hookDeletePolicy: hook-succeeded

apiVersion: batch/v1
kind: Job
metadata:
  name: ${release.name}-precheck
  labels:
    $include: common.labels
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        $each: ${values.precheck.containers}
        $yield:
          name: ${$item.name}
          image: ${$item.image}
```

## Tests

Tests are a separate mechanism from hooks. Test manifests live in the `tests/`
directory, are rendered and stored when you install or upgrade, and run only
when you invoke `keramos test <release>` — never during install or upgrade. A test
is a plain Pod or Job manifest; no `$hook` directive is involved.

**`tests/connection.yaml`:**

```yaml
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
        - curl -fsS http://${release.name}.${release.namespace}.svc.cluster.local/health
```

`keramos test <release>` applies each stored test, waits for its Pod/Job to finish,
and reports pass or fail from the exit status. Test resources are deleted after
each run. `keramos test` exits non-zero if any test fails.

```sh
keramos test my-app
keramos test my-app --logs              # print pod logs
keramos test my-app --filter connection # run tests whose name contains "connection"
keramos test my-app --parallel 4        # run up to 4 at once
keramos test my-app -o junit            # human | junit | json
```

## Rollback re-runs a revision's hooks

Keramos stores each revision's rendered hooks in that revision's release record.
Rolling back re-applies the old revision *and* re-runs its rollback hooks as
they were when that revision was installed. Design hooks to be **idempotent**
and **self-contained** — a rollback to revision 3 replays revision 3's hooks, so
they must not assume state that only a later revision created.

## Inspecting hooks

```sh
keramos get hooks <release>            # rendered hook manifests + last results
keramos get hooks <release> --revision 3
keramos history <release>              # per-revision hook outcomes
keramos status <release>              # current hook section
```

## See also

- [Hooks guide](../guides/hooks.md) — event ordering, delete policies at runtime
- [Control flow](control-flow.md) — the directives hook bodies use
- [Expressions](expressions.md) — `${...}` inside hook manifests
{% endraw %}
