---
title: "keramos test"
parent: "CLI"
---
{% raw %}
# keramos test

`keramos test` runs a deployed release's stored test manifests against the live
cluster and reports which passed.

## When to use it

- As a post-install or post-upgrade smoke test, in a pipeline or by hand.
- To confirm a release actually serves traffic, not just that its pods exist.
- With `-o junit` to feed CI test reporting.

## What happens

1. Loads the latest revision of `<release-name>` and requires its status to
   be `deployed`; anything else is an error.
2. Reads the test manifests stored on that revision — the package's `tests/`
   directory, rendered and saved at install/upgrade time. With no stored
   tests it prints a notice and exits 0.
3. Selects tests by name, keeping only those whose filename contains a
   `--filter` substring (all of them if no filter is given).
4. Applies each test, then waits up to `--timeout` for its Pods to reach
   `Succeeded` and its Jobs to complete; `--parallel` runs run concurrently
   and `--retries` re-runs a failed test.
5. Deletes each test's resources afterward, and with `--logs` prints the
   pod logs first.
6. Prints results in the `--output` format and exits non-zero if any test
   failed.

## Usage

```
keramos test <release-name> [flags]
```

The argument is a release name, not a package path — the tests come from the
recorded release, so the package directory need not be present.

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--filter` | stringArray | — | run only tests whose filename contains this substring, skipping the rest (repeatable) |
| `--timeout` | duration | 5m0s | fail a test if its pod does not finish within this window |
| `--parallel` | int | 1 | run this many tests at once; `1` runs them sequentially |
| `--retries` | int | 0 | re-run a failed test up to this many times before recording it as failed |
| `--logs` | — | false | print each test pod's logs after it finishes |
| `-o, --output` | string | "human" | result format: `human`, `junit` (XML), or `json` |

## Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `-n, --namespace` | string | namespace holding the release |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `--debug` | — | enable debug output |

## Worked example

**INPUT — the test fixture `tests/http-probe.yaml` in the package.** It runs
one Pod that fetches the service. keramos renders it at install time and stores
the result on the release:

```yaml
# web/tests/http-probe.yaml
apiVersion: v1
kind: Pod
metadata:
  name: "${release.name}-http-probe"
spec:
  restartPolicy: Never
  containers:
    - name: probe
      image: "${values.image.repository}:${values.image.tag}"
      command: ["wget", "-q", "http://${release.name}"]
```

Installed as release `web-prod`, that fixture renders and is stored as:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: web-prod-http-probe
spec:
  restartPolicy: Never
  containers:
    - name: probe
      image: nginx:1.27
      command: ["wget", "-q", "http://web-prod"]
```

**Run it:**

```sh
keramos test web-prod -n prod
```

**OUTPUT — the probe pod exits 0 (`Succeeded`), so the test passes:**

```
Running tests for release web-prod (revision 3)...
  TEST: http-probe.yaml
    PASS
All tests passed.
```

Read it back against the input: `TEST: http-probe.yaml` is the fixture's
filename, the stored `web-prod-http-probe` Pod reached `Succeeded`, so the
line reads `PASS` and the command exits 0.

**When the service is unreachable**, `wget` exits non-zero, the Pod ends in
`Failed`, and the run fails — with a non-zero exit code for CI:

```
Running tests for release web-prod (revision 3)...
  TEST: http-probe.yaml
    FAIL
Error: one or more tests failed for release web-prod
```

Add `--logs` to see the probe's own output under the `FAIL` line while you
debug.

## See also

- [`install`](install.md) — install a release and store its tests
- [`upgrade`](upgrade.md) — upgrade a release and refresh its stored tests
- [`lint`](lint.md) — validate a package before deploying
- [`get`](get.md) — inspect a release's stored manifests and hooks
{% endraw %}
