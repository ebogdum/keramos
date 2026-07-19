---
title: "Docs home"
nav_exclude: true
---
{% raw %}
# keramos documentation

keramos is a Kubernetes package manager built around expression-based templating,
layered composition, and dependency-aware orchestration. This documentation is
organized the way you'll actually use it.

## Start here

New to keramos? Follow the [Quickstart](guides/quickstart.md) — it takes you from an
empty directory to a deployed, upgraded, and rolled-back release.

## How-to guides

Task-focused walkthroughs for getting real work done.

| Guide | You'll learn to |
|---|---|
| [Quickstart](guides/quickstart.md) | Install, upgrade, and roll back your first release |
| [Packages](guides/packages.md) | Structure a keramos package and its templates |
| [Values](guides/values.md) | Layer and override configuration values |
| [Schema validation](guides/schema-validation.md) | Validate values with `values.schema.json` |
| [Layers](guides/layers.md) | Compose packages from reusable layers |
| [Hooks](guides/hooks.md) | Run jobs at install/upgrade/delete phases |
| [Releases](guides/releases.md) | Orchestrate several releases together |
| [Workspaces](guides/workspaces.md) | Manage a multi-package workspace |
| [Repositories](guides/repositories.md) | Publish and consume package repositories |
| [OCI](guides/oci.md) | Push and pull packages via OCI registries |
| [Signing](guides/signing.md) | Sign packages and verify provenance |
| [Plugins](guides/plugins.md) | Build a plugin that adds a `keramos` command |
| [Migration](guides/migration.md) | Convert a Helm chart to a keramos package |

## Reference

Exact, look-it-up detail.

| Reference | Covers |
|---|---|
| [CLI](cli/README.md) | Every command, flag, and option — with worked input→output examples |
| [Template functions](templates/functions.md) | ~200 built-in functions, each with input→output |
| [Expressions](templates/expressions.md) | The `${...}` syntax, operators, and pipelines |
| [Control flow](templates/control-flow.md) | `$if` / `$each` / `$switch` directives |
| [Layers](templates/layers.md) | Layer merge and override semantics |
| [Hooks](templates/hooks.md) | Hook directives and lifecycle phases |
| [Capabilities](templates/capabilities.md) | The cluster capability object |
| [`keramos.yaml`](reference/keramos-yaml.md) | Package manifest, field by field |
| [`values.yaml`](reference/values-yaml.md) | Values file and override precedence |
| [`values.schema.json`](reference/values-schema-json.md) | Supported JSON Schema keywords |
| [`keramos-releases.yaml`](reference/keramos-releases-yaml.md) | Cross-release definition |
| [`keramos-workspace.yaml`](reference/keramos-workspace-yaml.md) | Workspace definition |

## Understanding keramos

Background and orientation.

| Page | About |
|---|---|
| [Comparison](comparison.md) | keramos vs Helm, Kustomize, and others |
| [Use cases](use-cases.md) | Common scenarios mapped to commands |
| [FAQ](faq.md) | Frequent questions |
| [Glossary](glossary.md) | Terms used throughout these docs |
| [Troubleshooting](troubleshooting.md) | Symptoms, causes, and fixes |

## How these docs are written

Every reference example shows **input → output**: the file you write or the
command you run, and the exact result it produces. When a command reads hidden
state (a stored release, the live cluster), the docs show that state and trace
each output line back to its cause. See [`keramos drift`](cli/drift.md) for the
fullest example of this style.
{% endraw %}
