<p align="center">
  <img src="logo.png" alt="Many Templates">
</p>

<h1 align="center">Many Templates</h1>

<p align="center">
  A pipeline-based CLI tool for processing Kubernetes manifests and configuration files.
</p>

<p align="center">
  <a href="https://github.com/systemstart/many-templates/actions/workflows/ci.yml"><img src="https://github.com/systemstart/many-templates/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://codecov.io/gh/systemstart/many-templates"><img src="https://codecov.io/gh/systemstart/many-templates/branch/main/graph/badge.svg" alt="codecov"></a>
  <a href="https://goreportcard.com/report/github.com/systemstart/many-templates"><img src="https://goreportcard.com/badge/github.com/systemstart/many-templates" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-GPL--3.0-blue.svg" alt="License: GPL-3.0"></a>
  <a href="https://github.com/systemstart/many-templates/releases/latest"><img src="https://img.shields.io/github/v/release/systemstart/many-templates" alt="Release"></a>
</p>

---

Many Templates recursively discovers `.many.yaml` pipeline definitions in a directory tree, copies the source to an
output directory, and executes each pipeline in-place. Pipelines compose steps: **Go templating** (with Sprig),
**Kustomize** builds, **Helm** renders, **file generation** from inline templates, and **YAML stream splitting**.

<!-- TOC -->
  * [Installation](#installation)
  * [Quick Start](#quick-start)
  * [Examples](#examples)
  * [CLI Flags](#cli-flags)
    * [Discovery Mode (default)](#discovery-mode-default)
    * [Single Pipeline Mode](#single-pipeline-mode)
    * [Instances Mode](#instances-mode)
  * [`.many.yaml` Schema](#manyyaml-schema)
  * [Pipeline Steps](#pipeline-steps)
    * [`template`](#template)
    * [`kustomize`](#kustomize)
    * [`helm`](#helm)
    * [`generate`](#generate)
    * [`split`](#split)
      * [Splitting Strategies](#splitting-strategies)
  * [Context](#context)
    * [Pipeline-Local Context](#pipeline-local-context)
    * [Global Context](#global-context)
    * [Context Value Interpolation](#context-value-interpolation)
  * [Execution Model](#execution-model)
  * [Environment Variables](#environment-variables)
<!-- TOC -->

## Installation

```bash
go install github.com/systemstart/many-templates/cmd/many@latest
```

Or build from source:

```bash
go build -o many ./cmd/many
```

## Quick Start

Given a directory tree with `.many.yaml` pipeline definitions:

```
infrastructure/
├── .many.yaml
├── ingress.yaml
└── values.yaml
```

Where `.many.yaml` contains:

```yaml
context:
  domain: "example.com"

pipeline:
  - name: render
    type: template
    template:
      files:
        include: [ "**/*.yaml" ]
```

Run:

```bash
many \
  -input-directory ./infrastructure \
  -output-directory ./output \
  -overwrite-output-directory
```

The source tree is copied to `./output`, templates are rendered in-place using the context variables, and `.many.yaml`
files are removed from the output.

## Examples

**WARNING: this should only demonstrate the usage for a non-trivial project. Whether the resulting manifests provide
a working setup is not scope of this example.**

The [`examples/many-sites/`](examples/many-sites/) directory demonstrates instances mode with a real-world deployment
configuration. A shared set of service templates is rendered for two domains --- `fediverse.example` and
`development.example` --- each selecting a different subset of services:

- **fediverse.example** --- Mastodon, Matrix/Element, Mobilizon, Pixelfed
- **development.example** --- Forgejo, Woodpecker CI, Harbor

Both instances share infrastructure services (Dex, LLDAP, ESO), a global context file for common
configuration, and per-instance context for the domain name. External S3 and SMTP are configured globally.
The `instances.yaml` file defines the two instances with their `include` filters and context overrides.

```bash
many \
  -input-directory examples/many-sites/services \
  -output-directory output \
  -instances examples/many-sites/instances.yaml \
  -context-file examples/many-sites/context.yaml
```

## CLI Flags

| Flag                          | Description                                                       | Default  |
|-------------------------------|-------------------------------------------------------------------|----------|
| `-input-directory`            | Source directory to process                                       | required |
| `-output-directory`           | Destination for rendered output                                   | required |
| `-overwrite-output-directory` | Delete and recreate output directory                              | `false`  |
| `-context-file`               | Global context YAML file (removed from output if inside input)    | none     |
| `-max-depth`                  | Max directory recursion depth (`-1` = unlimited, `0` = root only) | `-1`     |
| `-processing`                 | Single `.many.yaml` to run (skips directory discovery)            | none     |
| `-instances`                  | Instances YAML file for matrix mode                               | none     |
| `-log-level`                  | `debug`, `info`, `warn`, `error`                                  | `info`   |
| `-logging-type`               | `json`, `text`, `tint`                                            | `tint`   |

### Discovery Mode (default)

When no `-processing` flag is given, `many` walks the input directory tree collecting all `.many.yaml` files. Pipelines
are sorted by directory depth (parents before children) and executed independently.

```bash
many \
  -input-directory ./infrastructure \
  -output-directory ./output \
  -max-depth 2
```

### Single Pipeline Mode

When `-processing` points to a specific `.many.yaml` (must be within the input directory), only that pipeline runs:

```bash
many \
  -processing ./infrastructure/cert-manager/.many.yaml \
  -input-directory ./infrastructure \
  -output-directory ./output
```

### Instances Mode

When `-instances` points to an instances YAML file, `many` runs the same input tree (or a filtered subset) multiple
times with different contexts, producing separate output directories. This is useful when you have a shared set of
templates that need to be rendered for multiple environments, domains, or tenants.

`-instances` is incompatible with `-processing`.

```bash
many \
  -input-directory ./applications \
  -output-directory ./output \
  -instances instances.yaml \
  -context-file global.yaml
```

The instances file defines a list of instances, each with a name, output directory, optional input subdirectory,
optional include filter, and optional context:

```yaml
instances:
  - name: prod-east
    output: prod-east/
    include:
      - api
      - frontend
    context:
      region: us-east-1
      replicas: 3

  - name: staging
    input: staging-apps/
    output: staging/
    context:
      region: us-west-2
      replicas: 1
```

| Field     | Description                                                           | Default     |
|-----------|-----------------------------------------------------------------------|-------------|
| `name`    | Unique identifier for the instance                                    | required    |
| `output`  | Output subdirectory (relative to `-output-directory`)                 | required    |
| `input`   | Input subdirectory (relative to `-input-directory`)                   | `""` (root) |
| `include` | List of immediate subdirectory names to include (empty = include all) | `[]`        |
| `context` | Additional context merged on top of global context for this instance  | `{}`        |

**Context merge order**: `-context-file` global context -> instance `context` -> per-directory `.many.yaml` `context`.
Instance context acts as an additional global layer for that run.

**Include filtering**: When `include` is specified, only the listed immediate subdirectories of the input directory are
copied to the output. Root-level files are always copied. When `include` is empty or absent, the entire input tree is
copied.

For each instance, `many`:

1. Copies the input tree (filtered by `include`) to the instance output directory
2. Merges global context with instance context
3. Discovers and executes pipelines within the instance output
4. Removes `.many.yaml` files from the instance output

If an instance fails, remaining instances still run. A summary of failed instances is reported at the end.

## `.many.yaml` Schema

Each `.many.yaml` defines a pipeline scoped to its directory.

```yaml
# Context variables available to template steps.
context:
  domain: "example.com"
  certManager:
    installCRDs: true

# Ordered list of steps. Executed sequentially.
pipeline:
  - name: template-configs
    type: template
    template:
      files:
        include: [ "kustomization.yaml", "values.yaml", "patches/**/*.yaml" ]

  - name: build
    type: kustomize
    kustomize:
      dir: "."

  - name: split-manifests
    type: split
    split:
      input: build
      by: kind
      outputDir: manifests/

  - name: template-output
    type: template
    template:
      files:
        include: [ "manifests/**/*.yaml" ]
```

## Pipeline Steps

Each step has a `name` (unique within the pipeline), a `type`, and type-specific configuration. Steps execute
sequentially. Steps that produce output (kustomize, helm) store their result keyed by name, which subsequent steps can
reference.

### `template`

Renders files in-place using Go's [`text/template`](https://pkg.go.dev/text/template)
with [Sprig](https://masterminds.github.io/sprig/) functions. Context variables from the pipeline's `context` block (
merged with any global context) are passed as template data.

```yaml
- name: render
  type: template
  template:
    files:
      include: [ "**/*.yaml" ]
      exclude: [ "kustomization.yaml" ]
```

| Field           | Description                         | Default    |
|-----------------|-------------------------------------|------------|
| `files.include` | Glob patterns for files to template | `["**/*"]` |
| `files.exclude` | Glob patterns for files to skip     | `[]`       |

Globs are matched relative to the pipeline directory. Patterns support `**` for recursive matching
via [doublestar](https://github.com/bmatcuk/doublestar).

### `kustomize`

Runs `kustomize build` and captures the multi-document YAML output. Requires `kustomize` on `PATH`.

```yaml
- name: build
  type: kustomize
  kustomize:
    dir: "."
    enableHelm: true
```

| Field        | Description                               | Default |
|--------------|-------------------------------------------|---------|
| `dir`        | Directory containing `kustomization.yaml` | `"."`   |
| `enableHelm` | Pass `--enable-helm` to kustomize         | `false` |

### `helm`

Runs `helm template` to render a chart. Requires `helm` on `PATH`.

```yaml
- name: render-chart
  type: helm
  helm:
    chart: ./charts/my-app
    releaseName: my-app
    namespace: default
    valuesFiles: [ "values.yaml" ]
    set:
      image.tag: "v1.2.3"
```

| Field         | Description                                | Default     |
|---------------|--------------------------------------------|-------------|
| `chart`       | Path to chart directory or chart reference | required    |
| `releaseName` | Helm release name                          | required    |
| `namespace`   | Target namespace                           | `"default"` |
| `valuesFiles` | List of values files                       | `[]`        |
| `set`         | Map of `--set` overrides                   | `{}`        |

### `generate`

Creates a file from an inline Go template rendered with [Sprig](https://masterminds.github.io/sprig/) functions against
the pipeline context. Unlike `template` (which renders existing files in-place), `generate` synthesizes new files purely
from context data, removing the need for placeholder files in the source tree.

```yaml
- name: gen-config
  type: generate
  generate:
    output: manifests/config.yaml
    template: |
      apiVersion: v1
      kind: ConfigMap
      metadata:
        name: {{ .app_name }}
      data:
        domain: {{ .domain }}
```

| Field      | Description                                         | Default  |
|------------|-----------------------------------------------------|----------|
| `output`   | Output file path relative to the pipeline directory | required |
| `template` | Inline Go template string                           | required |

Parent directories for the output path are created automatically.

### `split`

Takes a multi-document YAML stream from a previous `kustomize` or `helm` step and splits it into individual files.

```yaml
- name: split-manifests
  type: split
  split:
    input: build
    by: kind
    outputDir: manifests/
```

| Field              | Description                                              | Default  |
|--------------------|----------------------------------------------------------|----------|
| `input`            | Name of a previous step whose output to split            | required |
| `by`               | Splitting strategy                                       | `"kind"` |
| `outputDir`        | Directory to write split files into                      | `"."`    |
| `fileNameTemplate` | Go template for file paths (only with `custom` strategy) | ---      |

#### Splitting Strategies

**`kind`** --- One file per Kind. Multiple resources of the same Kind share a file as multi-document YAML.

```
manifests/
├── deployment.yaml
├── service.yaml
└── ingress.yaml
```

**`resource`** --- One file per resource: `<kind>-<name>.yaml`.

```
manifests/
├── deployment-api.yaml
├── deployment-worker.yaml
├── service-api.yaml
└── configmap-app-config.yaml
```

**`group`** --- Directories per API group, files per resource.

```
manifests/
├── apps/
│   └── deployment-api.yaml
├── core/
│   └── service-api.yaml
└── networking.k8s.io/
    └── ingress-main.yaml
```

**`kind-dir`** --- Directories per Kind (pluralized), files per resource name.

```
manifests/
├── deployments/
│   ├── api.yaml
│   └── worker.yaml
├── services/
│   └── api.yaml
└── ingresses/
    └── main.yaml
```

**`custom`** --- File paths determined by a Go template. The template receives the full manifest as a map.

```yaml
split:
  input: build
  by: custom
  outputDir: manifests/
  fileNameTemplate: "{{ .metadata.namespace | default \"cluster\" }}/{{ .kind | lower }}-{{ .metadata.name }}.yaml"
```

## Context

### Pipeline-Local Context

Each `.many.yaml` can define a `context` block. These values are available to all `template` steps in that pipeline:

```yaml
context:
  domain: "example.com"
  replicas: 3
```

Templates reference values with `{{ .domain }}`, `{{ .replicas }}`, etc.

### Global Context

A global context file can be provided via `-context-file`. It applies to all pipelines. Pipeline-local context overrides
global values (shallow merge at top-level keys):

```bash
many \
  -input-directory ./infra \
  -output-directory ./output \
  -context-file global.yaml
```

```yaml
# global.yaml
domain: "default.example.com"
environment: production
```

```yaml
# infra/app/.many.yaml — domain is overridden, environment is inherited
context:
  domain: "app.example.com"
pipeline:
  - name: render
    type: template
    template:
      files:
        include: [ "**/*.yaml" ]
```

### Context Value Interpolation

After merging global and pipeline-local context, all string values in the context map are rendered as Go templates
against the full context. This lets context values reference other context values:

```yaml
# global.yaml
domain: "example.com"
forgejo_url: "https://forgejo.{{ .domain }}"
app_url: "https://app.{{ .domain }}"
```

After interpolation, `forgejo_url` becomes `https://forgejo.example.com` and `app_url` becomes
`https://app.example.com`. The same [Sprig](https://masterminds.github.io/sprig/) functions available in template steps
can be used in context values (e.g. `{{ .name | upper }}`).

Interpolation is a **single pass** --- values can reference plain context keys but not other interpolated values.
Strings inside nested maps and slices are also interpolated. Non-string values (ints, bools) are left unchanged. If a
context value fails to parse or execute as a template, the pipeline aborts with an error.

## Execution Model

1. The entire source tree is copied to the output directory.
2. `.many.yaml` files are discovered in the output tree, sorted by directory depth.
3. Each pipeline executes in-place within the output tree.
4. Template steps modify files in-place. Kustomize/Helm steps produce YAML streams in memory. Split steps write streams
   to files.
5. `.many.yaml` files and the context file are removed from the output after all pipelines complete.

A failing step aborts its pipeline. Other pipelines continue. A summary of failed pipelines is printed at the end, and
the exit code is non-zero if any pipeline failed.

## Environment Variables

If a `.env` file exists in the working directory, it is loaded automatically
via [godotenv](https://github.com/joho/godotenv).
