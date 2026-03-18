<p align="center">
  <img src="logo.png" alt="Many Templates">
</p>

<h1 align="center">Many Templates</h1>

<p align="center">
  A swiss army-knife for Kubernetes manifests &mdash; multi-stage pipelines for fetching, processing, and rendering.
</p>

<p align="center">
  <a href="https://github.com/systemstart/many-templates/actions/workflows/ci.yml"><img src="https://github.com/systemstart/many-templates/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://codecov.io/gh/systemstart/many-templates"><img src="https://codecov.io/gh/systemstart/many-templates/branch/main/graph/badge.svg" alt="codecov"></a>
  <a href="https://goreportcard.com/report/github.com/systemstart/many-templates"><img src="https://goreportcard.com/badge/github.com/systemstart/many-templates" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-GPL--3.0-blue.svg" alt="License: GPL-3.0"></a>
  <a href="https://github.com/systemstart/many-templates/releases/latest"><img src="https://img.shields.io/github/v/release/systemstart/many-templates" alt="Release"></a>
</p>

---

`many` discovers `.many.yaml` pipeline definitions in a directory tree and executes them in-place.
Each pipeline composes steps --- **Go templating** (with [Sprig](https://masterminds.github.io/sprig/)),
**Kustomize** builds, **Helm** renders, **file generation**, and **YAML splitting** --- to turn a
template repository into ready-to-apply Kubernetes manifests.

Sources can live locally, in an OCI registry, behind an HTTPS URL, or in an
[OCM](https://ocm.software/) component --- `many` fetches and overlays them
transparently before running the pipeline.

<!-- TOC -->
  * [Installation](#installation)
  * [By Example](#by-example)
    * [1 --- Render and Split a Kustomization](#1----render-and-split-a-kustomization)
    * [2 --- Pull Templates from an OCI Registry](#2----pull-templates-from-an-oci-registry)
    * [3 --- Compose Multiple Sources](#3----compose-multiple-sources)
    * [4 --- Patch with Generate](#4----patch-with-generate)
    * [5 --- Helm Chart Rendering](#5----helm-chart-rendering)
    * [6 --- Multi-Instance Deployment](#6----multi-instance-deployment)
  * [The `pull` Subcommand](#the-pull-subcommand)
  * [CLI Reference](#cli-reference)
    * [Discovery Mode (default)](#discovery-mode-default)
    * [Single Pipeline Mode](#single-pipeline-mode)
    * [Instances Mode](#instances-mode)
  * [Pipeline Steps](#pipeline-steps)
    * [`template`](#template)
    * [`kustomize`](#kustomize)
    * [`helm`](#helm)
    * [`generate`](#generate)
    * [`split`](#split)
  * [Sources](#sources)
  * [Context](#context)
    * [Pipeline-Local Context](#pipeline-local-context)
    * [Global Context](#global-context)
    * [Context Merge Order](#context-merge-order)
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
make build   # produces ./many
```

## By Example

### 1 --- Render and Split a Kustomization

The most common pattern: template variables into Kustomize inputs, build, and split
the resulting multi-document YAML into individual files.

```
cert-manager/
├── .many.yaml
├── kustomization.yaml      # contains {{ .namespace }}
├── values.yaml             # contains {{ .domain }}
└── externalsecret.yaml
```

```yaml
# cert-manager/.many.yaml
context:
  namespace: cert-manager
  domain: "example.com"

pipeline:
  - name: render-templates
    type: template
    template:
      files:
        include: ["**/*.yaml"]
        exclude: [".many.yaml"]

  - name: build
    type: kustomize
    kustomize:
      enableHelm: true

  - name: split-output
    type: split
    split:
      input: build
      by: resource
      outputDir: manifests/
```

```bash
many -input ./cert-manager -output-directory ./output
```

The source tree is copied to `./output`, templates are rendered, `kustomize build`
runs, the output is split into one file per resource under `manifests/`, and
`.many.yaml` is removed.

### 2 --- Pull Templates from an OCI Registry

Instead of keeping templates in the local tree, pull them from an OCI image.
The `source` block fetches and overlays the image contents before the pipeline runs.

```yaml
# .many.yaml
source:
  oci: ghcr.io/myorg/cert-manager-templates:v1.0.0

context:
  namespace: cert-manager

pipeline:
  - name: render-templates
    type: template
    template:
      files:
        include: ["**/*.yaml"]
        exclude: [".many.yaml"]

  - name: build
    type: kustomize

  - name: split-output
    type: split
    split:
      input: build
      by: resource
      outputDir: manifests/
```

Sources can also use `https` (tarballs or single files), `file` (local paths),
or `ocm` (OCM component versions). See [Sources](#sources).

### 3 --- Compose Multiple Sources

A pipeline can overlay multiple sources. They are applied in order, so later
sources overwrite earlier ones --- perfect for base + patches workflows.

```yaml
source:
  - oci: ghcr.io/myorg/base-manifests:v2.0.0
  - oci: ghcr.io/myorg/env-patches:v1.1.0
    path: patches/          # overlay into a subdirectory
  - file: ./local-overrides

pipeline:
  - name: render
    type: template
    template:
      files:
        include: ["**/*.yaml"]
```

Sources can also be attached to individual steps rather than the whole pipeline:

```yaml
pipeline:
  - name: fetch-and-render
    type: template
    source:
      https: https://github.com/myorg/configs/archive/main.tar.gz
    template:
      files:
        include: ["**/*.yaml"]
```

### 4 --- Patch with Generate

The `generate` step creates files from inline Go templates. Use it to synthesize
Kustomize patches, ConfigMaps, or any manifest from context data alone --- no
placeholder files needed.

```yaml
context:
  app_name: my-api
  replicas: 3
  domain: "api.example.com"

pipeline:
  - name: gen-patch
    type: generate
    generate:
      output: patches/replicas.yaml
      template: |
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: {{ .app_name }}
        spec:
          replicas: {{ .replicas }}

  - name: gen-ingress
    type: generate
    generate:
      output: ingress.yaml
      template: |
        apiVersion: networking.k8s.io/v1
        kind: Ingress
        metadata:
          name: {{ .app_name }}
        spec:
          rules:
            - host: {{ .domain }}
              http:
                paths:
                  - path: /
                    pathType: Prefix
                    backend:
                      service:
                        name: {{ .app_name }}
                        port:
                          number: 8080

  - name: build
    type: kustomize

  - name: split
    type: split
    split:
      input: build
      by: resource
      outputDir: manifests/
```

### 5 --- Helm Chart Rendering

For Helm-based services, use the `helm` step to run `helm template` and pipe the
output into a split step.

```yaml
context:
  domain: "example.com"

pipeline:
  - name: render-values
    type: template
    template:
      files:
        include: ["values.yaml"]

  - name: render-chart
    type: helm
    helm:
      chart: ./charts/my-app
      releaseName: my-app
      namespace: my-app
      valuesFiles: ["values.yaml"]
      set:
        ingress.host: "app.example.com"

  - name: split
    type: split
    split:
      input: render-chart
      by: kind-dir
      outputDir: manifests/
```

### 6 --- Multi-Instance Deployment

The real power shows in **instances mode**: render the same template tree for
multiple environments, domains, or tenants in a single run.

The [`examples/many-sites/`](examples/many-sites/) directory demonstrates this
with two domains that each select a different subset of services from a shared pool:

```
examples/many-sites/
├── instances.yaml          # defines fediverse + development instances
├── context.yaml            # shared config (subdomains, storage, OIDC, ...)
└── services/
    ├── dex/                # Kustomize + Helm, templated values
    ├── lldap/
    ├── mastodon/
    ├── matrix/
    ├── forgejo/
    ├── harbor/
    └── ...
```

**instances.yaml** --- each instance picks services with `include` and sets
per-instance context:

```yaml
instances:
  - name: fediverse
    output: fediverse.example/
    include: [dex, eso, lldap, mastodon, matrix, mobilizon, pixelfed]
    context:
      domain: "fediverse.example"
      siteName: "fediverse"
      services: [dex, eso, lldap, mastodon, matrix, mobilizon, pixelfed]

  - name: development
    output: development.example/
    include: [dex, eso, lldap, forgejo, harbor, woodpecker]
    context:
      domain: "development.example"
      siteName: "development"
      services: [dex, eso, lldap, forgejo, harbor, woodpecker]
```

**context.yaml** --- shared configuration. Values can reference other context
keys via Go templates (single-pass interpolation):

```yaml
subdomains:
  dex: "auth"
  mastodon: "social"
  forgejo: "git"
  # ...

smtp:
  from: "noreply@{{ .domain }}"

eso:
  remotePathPrefix: "site/{{ .siteName }}"
```

**Each service** has a `.many.yaml` with the template-build-split pattern:

```yaml
# services/dex/.many.yaml
context:
  namespace: dex

pipeline:
  - name: render-templates
    type: template
    template:
      files:
        include: ["**/*.yaml"]
        exclude: [".many.yaml"]

  - name: build
    type: kustomize
    kustomize:
      enableHelm: true

  - name: split-output
    type: split
    split:
      input: build
      by: resource
      outputDir: manifests/
```

Run it:

```bash
many \
  -input examples/many-sites/services \
  -output-directory output \
  -instances examples/many-sites/instances.yaml \
  -context-file examples/many-sites/context.yaml
```

Output:

```
output/
├── fediverse.example/
│   ├── dex/manifests/...
│   ├── mastodon/manifests/...
│   └── ...
└── development.example/
    ├── dex/manifests/...
    ├── forgejo/manifests/...
    └── ...
```

> **Note:** This example demonstrates the tool's usage for a non-trivial project. Whether the
> resulting manifests provide a working setup is not in scope.

## The `pull` Subcommand

`many pull` fetches a source reference to a local directory without running any pipeline.
Useful for scripting --- no need to know whether to call `crane`, `curl`, or `ocm` for a
given URI.

```bash
many pull <ref> <dir>
```

Examples:

```bash
# Local path
many pull ./examples/many-sites/services /tmp/local-copy

# OCI image
many pull oci://ghcr.io/myorg/templates:latest /tmp/oci-pull

# HTTPS tarball
many pull https://github.com/myorg/templates/archive/main.tar.gz /tmp/https-pull

# OCM component
many pull ocm://ghcr.io/myorg/ocm//github.com/myorg/templates:v1.0.0 /tmp/ocm-pull
```

Prints nothing on success, exits non-zero with an error message on failure.

## CLI Reference

| Flag                          | Description                                                       | Default  |
|-------------------------------|-------------------------------------------------------------------|----------|
| `-input`, `-input-directory`  | Source directory (or remote URI) to process                       | required |
| `-output-directory`           | Destination for rendered output                                   | required |
| `-overwrite-output-directory` | Delete and recreate output directory                              | `false`  |
| `-context-file`               | Global context YAML file (removed from output if inside input)    | none     |
| `-max-depth`                  | Max directory recursion depth (`-1` = unlimited, `0` = root only) | `-1`     |
| `-processing`                 | Single `.many.yaml` to run (skips directory discovery)            | none     |
| `-instances`                  | Instances YAML file for matrix mode                               | none     |
| `-log-level`                  | `debug`, `info`, `warn`, `error`                                  | `info`   |
| `-logging-type`               | `json`, `text`, `tint`                                            | `tint`   |
| `-version`                    | Print version and exit                                            |          |

### Discovery Mode (default)

When neither `-processing` nor `-instances` is given, `many` walks the input directory
collecting all `.many.yaml` files, sorts them by depth (parents before children), and
executes each pipeline independently.

```bash
many \
  -input ./infrastructure \
  -output-directory ./output \
  -max-depth 2
```

### Single Pipeline Mode

Run one specific `.many.yaml` (must be within the input directory):

```bash
many \
  -processing ./infrastructure/cert-manager/.many.yaml \
  -input ./infrastructure \
  -output-directory ./output
```

### Instances Mode

Run the same input tree multiple times with different contexts, producing separate output
directories. Incompatible with `-processing`.

```bash
many \
  -input ./services \
  -output-directory ./output \
  -instances instances.yaml \
  -context-file global.yaml
```

Instance file format:

```yaml
instances:
  - name: prod-east
    output: prod-east/          # required --- subdirectory of -output-directory
    input: ""                   # optional --- subdirectory of -input (or remote URI)
    include: [api, frontend]    # optional --- filter immediate subdirectories (empty = all)
    context:                    # optional --- merged on top of global context
      region: us-east-1
      replicas: 3
```

For each instance, `many` copies the input tree (filtered by `include`), merges
global + instance context, discovers and runs pipelines, then removes `.many.yaml`
files. If an instance fails, remaining instances still run. The exit code is non-zero
if any instance failed.

## Pipeline Steps

Each step has a `name` (unique within the pipeline) and a `type`. Steps execute
sequentially. Steps that produce YAML output (`kustomize`, `helm`) store it by
name so later steps (e.g. `split`) can reference it.

### `template`

Renders files in-place using Go's [`text/template`](https://pkg.go.dev/text/template)
with [Sprig](https://masterminds.github.io/sprig/) functions.

```yaml
- name: render
  type: template
  template:
    files:
      include: ["**/*.yaml"]
      exclude: ["kustomization.yaml"]
```

| Field           | Description                         | Default    |
|-----------------|-------------------------------------|------------|
| `files.include` | Glob patterns for files to template | `["**/*"]` |
| `files.exclude` | Glob patterns for files to skip     | `[]`       |

Globs are relative to the pipeline directory and support `**` for recursive matching
via [doublestar](https://github.com/bmatcuk/doublestar).

### `kustomize`

Runs `kustomize build` and captures the multi-document YAML output. Requires
`kustomize` on `PATH`.

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
    valuesFiles: ["values.yaml"]
    set:
      image.tag: "v1.2.3"
```

| Field         | Description                               | Default     |
|---------------|-------------------------------------------|-------------|
| `chart`       | Path to chart directory or chart reference | required    |
| `releaseName` | Helm release name                         | required    |
| `namespace`   | Target namespace                          | `"default"` |
| `valuesFiles` | List of values files                      | `[]`        |
| `set`         | Map of `--set` overrides                  | `{}`        |

### `generate`

Creates a file from an inline Go template rendered against the pipeline context.
Unlike `template` (which renders existing files in-place), `generate` synthesizes
new files purely from context data.

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

Parent directories are created automatically.

### `split`

Takes a multi-document YAML stream from a previous `kustomize` or `helm` step and
splits it into individual files.

```yaml
- name: split-manifests
  type: split
  split:
    input: build
    by: kind
    outputDir: manifests/
```

| Field               | Description                                              | Default  |
|---------------------|----------------------------------------------------------|----------|
| `input`             | Name of a previous step whose output to split            | required |
| `by`                | Splitting strategy (see below)                           | `"kind"` |
| `outputDir`         | Directory to write split files into                      | `"."`    |
| `fileNameTemplate`  | Go template for file paths (only with `custom` strategy) | ---      |
| `canonicalKeyOrder` | Reorder keys: apiVersion, kind, metadata first           | `true`   |

**Splitting strategies:**

| Strategy    | Layout                                                                                             |
|-------------|----------------------------------------------------------------------------------------------------|
| `kind`      | One file per Kind (`deployment.yaml`, `service.yaml`). Multiple resources of the same Kind share a file. |
| `resource`  | One file per resource (`deployment-api.yaml`, `service-api.yaml`).                                 |
| `group`     | Directories per API group (`apps/deployment-api.yaml`, `core/service-api.yaml`).                   |
| `kind-dir`  | Directories per Kind, pluralized (`deployments/api.yaml`, `services/api.yaml`).                    |
| `custom`    | File paths from a Go template: `fileNameTemplate: "{{ .metadata.namespace }}/{{ .kind | lower }}-{{ .metadata.name }}.yaml"` |

## Sources

Sources fetch remote or local content and overlay it into the pipeline directory
before steps run. A source can be a single entry or a list (applied in order).

```yaml
# Single source
source:
  oci: ghcr.io/myorg/templates:v1.0.0

# Multiple sources (overlaid in order)
source:
  - oci: ghcr.io/myorg/base:v2.0.0
  - file: ./local-patches
    path: patches/
```

| Field       | Description                                           |
|-------------|-------------------------------------------------------|
| `oci`       | OCI image reference (requires `crane` on `PATH`)      |
| `https`     | URL to a `.tar.gz`/`.tgz` tarball or single file      |
| `file`      | Local filesystem path                                 |
| `ocm`       | OCM component reference (requires `ocm` on `PATH`)   |
| `path`      | Target subdirectory within the pipeline directory     |
| `recursive` | Download referenced sub-components (OCM only)         |

Exactly one of `oci`, `https`, `file`, or `ocm` must be set per entry.

Sources can also be attached to individual steps via the step-level `source` field,
using the same format.

## Context

### Pipeline-Local Context

Each `.many.yaml` can define a `context` block. These values are available as
template data in all `template` and `generate` steps:

```yaml
context:
  domain: "example.com"
  replicas: 3

pipeline:
  - name: render
    type: template
```

Templates reference values with `{{ .domain }}`, `{{ .replicas }}`, etc.

### Global Context

A global context file provided via `-context-file` applies to all pipelines:

```bash
many -input ./infra -output-directory ./output -context-file global.yaml
```

### Context Merge Order

Context is merged in layers (later layers override earlier ones, deep-merged for
nested maps):

1. `-context-file` (global)
2. Instance `context` (instances mode only)
3. `.many.yaml` `context` (pipeline-local)

```yaml
# global.yaml
domain: "default.example.com"
database:
  host: "db.default"

# .many.yaml --- domain overridden, database.host inherited, database.port added
context:
  domain: "app.example.com"
  database:
    port: 5432
```

### Context Value Interpolation

After merging, all string values are rendered as Go templates against the full
context (single pass). This lets context values reference other context values:

```yaml
domain: "example.com"
forgejo_url: "https://forgejo.{{ .domain }}"
smtp_from: "noreply@{{ .domain }}"
```

After interpolation, `forgejo_url` becomes `https://forgejo.example.com`.
[Sprig](https://masterminds.github.io/sprig/) functions are available
(e.g. `{{ .name | upper }}`). Non-string values (ints, bools) are left unchanged.

## Execution Model

1. The source tree is copied to the output directory.
2. `.many.yaml` files are discovered, sorted by directory depth (parents first).
3. Each pipeline executes in-place within the output tree.
4. `template` steps modify files in-place. `kustomize`/`helm` steps produce YAML
   in memory. `split` steps write that YAML to files.
5. `.many.yaml` files and the context file are removed from the output.

A failing step aborts its pipeline. Other pipelines continue. The exit code is
non-zero if any pipeline failed.

## Environment Variables

If a `.env` file exists in the working directory, it is loaded automatically
via [godotenv](https://github.com/joho/godotenv). These are available to
`kustomize` and `helm` steps via environment inheritance.
