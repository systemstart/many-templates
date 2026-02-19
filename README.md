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
**Kustomize** builds, **Helm** renders, and **YAML stream splitting**.

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

## CLI Flags

| Flag                          | Description                                                       | Default  |
|-------------------------------|-------------------------------------------------------------------|----------|
| `-input-directory`            | Source directory to process                                       | required |
| `-output-directory`           | Destination for rendered output                                   | required |
| `-overwrite-output-directory` | Delete and recreate output directory                              | `false`  |
| `-context-file`               | Global context YAML file (removed from output if inside input)    | none     |
| `-max-depth`                  | Max directory recursion depth (`-1` = unlimited, `0` = root only) | `-1`     |
| `-processing`                 | Single `.many.yaml` to run (skips directory discovery)            | none     |
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

## Execution Model

1. The entire source tree is copied to the output directory.
2. `.many.yaml` files are discovered in the output tree, sorted by directory depth.
3. Each pipeline executes in-place within the output tree.
4. Template steps modify files in-place. Kustomize/Helm steps produce YAML streams in memory. Split steps write streams
   to files.
5. `.many.yaml` files and the context file are removed from the output after all pipelines complete.

A failing step aborts its pipeline. Other pipelines continue. A summary of failed pipelines is printed at the end, and
the exit code is non-zero if any pipeline failed.

## Examples

**WARNING: this should only demonstrate the usage for a non-trivial project. Whether the resulting manifests provide 
a working setup is not scope of this example.**


The [`examples/big-site/`](examples/big-site/) directory contains a complete, real-world deployment configuration
combining three stacks under shared infrastructure:

- **Infrastructure** --- Dex (OIDC), LLDAP, MinIO, ESO (External Secrets Operator), Argo CD
- **Fediverse** --- Mastodon, Matrix/Element, Mobilizon, Pixelfed
- **Office** --- ownCloud OCIS, Paperless-ngx, Stalwart mail
- **SDP** --- Forgejo, Woodpecker CI, Harbor

All services share Dex/LLDAP for SSO, use ExternalSecret resources for secrets management, and are deployed via an
ArgoCD app-of-apps pattern. The example demonstrates single-step templating for plain manifests, multi-step
Helm+Kustomize+split pipelines, and a global context file for shared configuration.

## Environment Variables

If a `.env` file exists in the working directory, it is loaded automatically
via [godotenv](https://github.com/joho/godotenv).
