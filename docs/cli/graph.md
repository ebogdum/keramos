# keramos graph

`keramos graph` emits a dependency graph of a release's resources and lifecycle
hooks in Mermaid, DOT, or plain ASCII.

## When to use it

- To document a release visually — which resources it contains, how config and
  secrets feed the workloads, and the order its hooks fire in.
- To paste a diagram straight into a Markdown doc (Mermaid) or render a PNG
  with Graphviz (DOT).
- To eyeball the structure in a terminal with no extra tools (ASCII).

## What happens

1. You name a release. keramos reads its stored record — the latest revision, or
   the one you pass to `--revision`.
2. It builds a graph: one node per Kubernetes resource (labelled `Kind/name`),
   one node per lifecycle hook, and edges from ConfigMaps/Secrets to the
   workloads that mount them.
3. Hook nodes are ordered by phase (pre-install, post-install, …) and weight,
   with edges between them, so the implicit run order becomes explicit.
4. It prints the graph in the chosen `--format` to stdout.

## Usage

```
keramos graph <release-name> [flags]
```

## Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `-f, --format` | string | `mermaid` | output dialect: `mermaid` (Markdown), `dot` (Graphviz), or `ascii` (terminal) |
| `--revision` | int | `0` | graph this revision; `0` uses the latest |

### Persistent flags inherited from `keramos`

| Flag | Type | Description |
|---|---|---|
| `--debug` | — | enable debug output |
| `--kube-context` | string | Kubernetes context to use |
| `--kubeconfig` | string | path to kubeconfig file |
| `-n, --namespace` | string | Kubernetes namespace |

## Worked example

Get the default Mermaid graph for the `web-api` release:

```sh
keramos graph web-api -n prod
```

Output (paste it into a Markdown ```` ```mermaid ```` block):

```
%% keramos graph: web-api
flowchart TD
  deployment_web_api[Deployment/web-api (prod)]
  service_web_api[Service/web-api (prod)]
  configmap_web_api_config[ConfigMap/web-api-config (prod)]
  hook_pre_install_w0((pre-install w=0))
  hook_post_install_w0((post-install w=0))
  configmap_web_api_config --> deployment_web_api
  hook_pre_install_w0 --> hook_post_install_w0
```

Render a PNG with Graphviz:

```sh
keramos graph web-api -n prod --format dot | dot -Tpng > web-api.png
```

That DOT output looks like:

```
digraph "web-api" {
  rankdir=TD;
  deployment_web_api [label="Deployment/web-api (prod)" shape=box];
  service_web_api [label="Service/web-api (prod)" shape=box];
  configmap_web_api_config [label="ConfigMap/web-api-config (prod)" shape=box];
  hook_pre_install_w0 [label="pre-install w=0" shape=ellipse];
  configmap_web_api_config -> deployment_web_api;
}
```

Quick ASCII view in the terminal:

```sh
keramos graph web-api -n prod --format ascii
```

```
· Deployment/web-api (prod)
· Service/web-api (prod)
· ConfigMap/web-api-config (prod)
○ pre-install w=0
○ post-install w=0

edges:
  configmap_web_api_config → deployment_web_api
  hook_pre_install_w0 → hook_post_install_w0
```

## See also

- [`get manifest`](get-manifest.md) — the resources the graph is built from
- [`get hooks`](get-hooks.md) — hook manifests and their execution results
