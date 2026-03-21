<p align="center">
  <img src="logo.png" alt="Many Templates">
</p>

<h1 align="center">Many Templates</h1>

<p align="center">
  A swiss army knife for Kubernetes manifests &mdash; multi-stage pipelines for fetching, processing, and rendering.
</p>

<p align="center">
  <a href="https://github.com/systemstart/many-templates/actions/workflows/ci.yml"><img src="https://github.com/systemstart/many-templates/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://codecov.io/gh/systemstart/many-templates"><img src="https://codecov.io/gh/systemstart/many-templates/branch/main/graph/badge.svg" alt="codecov"></a>
  <a href="https://goreportcard.com/report/github.com/systemstart/many-templates"><img src="https://goreportcard.com/badge/github.com/systemstart/many-templates" alt="Go Report Card"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-GPL--3.0-blue.svg" alt="License: GPL-3.0"></a>
  <a href="https://github.com/systemstart/many-templates/releases/latest"><img src="https://img.shields.io/github/v/release/systemstart/many-templates" alt="Release"></a>
</p>

---

`many` is a GitOps tool to process [Kubernetes](https://kubernetes.io/) manifests from many sources in many formats.

Sources can live locally, in an OCI registry, behind an HTTPS URL, or in an
[OCM](https://ocm.software/) component --- `many` fetches and processes them in a pipeline.

`many` discovers `.many.yaml` pipeline definitions in a directory tree, executes them and writes the results to a output
directory.

Each pipeline composes steps --- **Go templating** (with [Sprig](https://masterminds.github.io/sprig/)),
**Kustomize** builds, **Helm** renders, **file generation**, and **YAML splitting** --- to turn a
template repository into ready-to-apply Kubernetes manifests.

`many` can fan out outputs to many instances, check the [examples](#examples).

<!-- TOC -->
  * [Installation](#installation)
  * [Examples](#examples)
    * [1 --- Fetch and Render Go Templates](#1-----fetch-and-render-go-templates)
    * [2 --- Fetch, Split, and Kustomize](#2-----fetch-split-and-kustomize)
    * [3 --- Instances](#3-----instances)
  * [CLI Reference](#cli-reference)
    * [Discovery Mode (default)](#discovery-mode-default)
    * [Single Pipeline Mode](#single-pipeline-mode)
    * [Instances Mode](#instances-mode)
  * [Pipeline Steps](#pipeline-steps)
    * [`template`](#template)
    * [`kustomize-build`](#kustomize-build)
    * [`kustomize-create`](#kustomize-create)
    * [`helm`](#helm)
    * [`generate`](#generate)
    * [`copy`](#copy)
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

## Examples

### 1 --- Fetch and Render Go Templates

[`examples/source-tarball/`](examples/source-tarball/) fetches a tarball from a URL
and renders Go templates from its contents.

```yaml
# examples/source-tarball/.many.yaml
pipeline:
  - name: fetch-and-render
    type: template
    source:
      # renovate: datasource=github-tags depName=myorg/configs
      https: https://github.com/myorg/configs/archive/main.tar.gz
      sha256: ""
    template:
      files:
        include: ["**/*.yaml"]
```

The `source` on the step fetches files into the working directory before the step
executes. Here an HTTPS tarball is downloaded and extracted, then all YAML files
are rendered as Go templates with [Sprig](https://masterminds.github.io/sprig/)
functions.

**SHA-256 verification** --- the `sha256` field pins a source to a known checksum.
When set `many` verifies the download matches
before proceeding. An empty string disables verification, useful during development. 
The checksum will be set if empty, unless `-no-sha256-update` is provided. Note: that doesn't
update existing checksum, to update they have to be emptied manually first.

**Renovate integration** --- the `# renovate:` comment is a
[Renovate](https://docs.renovatebot.com/) annotation. Renovate can be configured to update both the URL
and the `sha256` checksum.

```bash
many -input examples/source-tarball -output-directory ./output
```

### 2 --- Fetch, Split, and Kustomize

[`examples/kubevirt/`](examples/kubevirt/) downloads upstream release YAML files,
splits each into individual resources, and generates a kustomization.

```yaml
# examples/kubevirt/source/.many.yaml (abbreviated --- 4 similar split steps)
pipeline:
  - name: split-kubevirt-operator
    type: split
    source:
      # renovate: datasource=github-releases depName=kubevirt/kubevirt
      - https: "https://github.com/kubevirt/kubevirt/releases/download/v1.7.2/kubevirt-operator.yaml"
        sha256: "b74106509bbce107a01b228083b4030890b2278cf918fe5c2cf380fdc2a27aef"
        temporary: true
        path: build/
    split:
      input: build/kubevirt-operator.yaml
      by: resource
      outputDir: manifests/operator/
    exclude:
      - "build/**"

  # ... split-kubevirt-cr, split-cdi-operator, split-cdi-cr ...

  - name: create-kustomization
    type: kustomize-create
    kustomize-create:
      autodetect: true
      recursive: true
      dir: manifests/
```

Each `source` fetches a release YAML into `build/`. The `split` step breaks it
into one file per resource under `manifests/`, and `temporary: true` combined with
`exclude` cleans up the intermediate `build/` directory. The final `kustomize-create`
step generates a `kustomization.yaml` referencing all discovered manifests:

```
output/manifests/
├── kustomization.yaml
├── cdi/
│   ├── cdi-cdi.yaml
│   ├── deployment-cdi-operator.yaml
│   └── ...
└── operator/
    ├── kubevirt-kubevirt.yaml
    ├── deployment-virt-operator.yaml
    └── ...
```

```bash
many -input examples/kubevirt/source -output-directory ./output
```

### 3 --- Instances

[`examples/many-sites/`](examples/many-sites/) renders the same services for
multiple environments. Each instance selects a subset of services and provides
per-instance context values.

**instances.yaml** defines two instances with different service selections:

```yaml
instances:
  - name: fediverse
    input: "services"
    output: fediverse.example/
    include: [dex, eso, lldap, mastodon, matrix, mobilizon, pixelfed]
    context:
      domain: "fediverse.example"
      siteName: "fediverse"

  - name: development
    input: "services"
    output: development.example/
    include: [dex, eso, lldap, forgejo, harbor, woodpecker]
    context:
      domain: "development.example"
      siteName: "development"
```

**context.yaml** provides shared configuration. String values can reference other
context keys via Go template interpolation (single pass after merge):

```yaml
subdomains:
  mastodon: "social"
  forgejo: "git"
  # ...

smtp:
  from: "noreply@{{ .domain }}"

eso:
  remotePathPrefix: "site/{{ .siteName }}"
```

Services follow two patterns. **Helm-based services** (dex, mastodon, ...) use
`kustomize-build` with Helm and split the output:

```yaml
# services/dex/.many.yaml
context:
  namespace: dex

pipeline:
  - name: render-templates
    type: template
    source:
      - file: kustomization.yaml
      - file: values.yaml
      - file: externalsecret.yaml
    template:
      files:
        include: ["**/*.yaml"]

  - name: build
    type: kustomize-build
    kustomize-build:
      enableHelm: true
      outputFile: kustomize-output.yaml

  - name: split-output
    type: split
    exclude: ["kustomize-output.yaml"]
    split:
      input: kustomize-output.yaml
      by: resource
      outputDir: manifests/

  - name: create-kustomization
    type: kustomize-create
    kustomize-create:
      autodetect: true
      recursive: true
      dir: manifests/
```

**Raw-manifest services** (lldap, woodpecker, ...) render templates and generate a
kustomization:

```yaml
# services/lldap/.many.yaml
pipeline:
  - name: render-templates
    type: template
    source:
      file: manifests/
      path: manifests/
    template:
      files:
        include: ["**/*.yaml"]

  - name: create-kustomization
    type: kustomize-create
    kustomize-create:
      autodetect: true
      recursive: true
      namespace: lldap
```

Context is merged in layers --- global (`-context-file`) → instance
(`context` in instances.yaml) → pipeline-local (`context` in `.many.yaml`) ---
with later layers overriding earlier ones via deep merge.

```bash
many \
  -input examples/many-sites \
  -output-directory output \
  -instances examples/many-sites/instances.yaml \
  -context-file examples/many-sites/context.yaml
```

Output:

```
output/
├── fediverse.example/
│   ├── dex/
│   │   ├── kustomization.yaml
│   │   └── manifests/...
│   ├── mastodon/...
│   └── ...
└── development.example/
    ├── dex/...
    ├── forgejo/...
    └── ...
```

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
| `-env-file`                   | Load environment variables from the specified file                | none     |
| `-no-sha256-update`           | Disable sha256 writeback to `.many.yaml` files                   | `false`  |
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
    include: [ api, frontend ]    # optional --- filter immediate subdirectories (empty = all)
    context: # optional --- merged on top of global context
      region: us-east-1
      replicas: 3
```

For each instance, `many` copies the input tree (filtered by `include`), merges
global + instance context, discovers and runs pipelines, then removes `.many.yaml`
files. If an instance fails, remaining instances still run. The exit code is non-zero
if any instance failed.

## Pipeline Steps

Each step has a `name` (unique within the pipeline) and a `type`. Steps execute
sequentially.

### `template`

Renders files in-place using Go's [`text/template`](https://pkg.go.dev/text/template)
with [Sprig](https://masterminds.github.io/sprig/) functions.

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

Globs are relative to the pipeline directory and support `**` for recursive matching
via [doublestar](https://github.com/bmatcuk/doublestar).

### `kustomize-build`

Runs `kustomize build` and captures the multi-document YAML output. Requires
`kustomize` on `PATH`.

```yaml
- name: build
  type: kustomize-build
  kustomize-build:
    enableHelm: true
    outputFile: kustomize-output.yaml
```

| Field        | Description                               | Default |
|--------------|-------------------------------------------|---------|
| `dir`        | Directory containing `kustomization.yaml` | `"."`   |
| `enableHelm` | Pass `--enable-helm` to kustomize         | `false` |
| `outputFile` | File to write the build output to         | none    |

### `kustomize-create`

Runs `kustomize create` to auto-generate a `kustomization.yaml` file. Useful when
pulling sources from OCI/HTTPS and you want to avoid hand-writing the kustomization
file. Requires `kustomize` on `PATH`.

```yaml
- name: create-kustomization
  type: kustomize-create
  kustomize-create:
    dir: "."
    autodetect: true
    recursive: true
    resources:
      - deployment.yaml
      - ../base
    namespace: "staging"
    nameprefix: "acme-"
    namesuffix: "-v2"
    annotations:
      app.kubernetes.io/managed-by: many
    labels:
      env: production
```

| Field         | Description                                       | Default |
|---------------|---------------------------------------------------|---------|
| `dir`         | Directory in which to create `kustomization.yaml` | `"."`   |
| `autodetect`  | Pass `--autodetect` to discover resources         | `false` |
| `recursive`   | Pass `--recursive` (requires `autodetect`)        | `false` |
| `resources`   | Explicit list of resources to include             | `[]`    |
| `namespace`   | Set namespace in the generated kustomization      | none    |
| `nameprefix`  | Set name prefix                                   | none    |
| `namesuffix`  | Set name suffix                                   | none    |
| `annotations` | Map of annotations to add                         | `{}`    |
| `labels`      | Map of labels to add                              | `{}`    |

At least one of `autodetect` or `resources` must be set. `recursive` requires
`autodetect` to be enabled.

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
| `outputFile`  | File to write the rendered output to       | none        |

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

### `copy`

Copies files from the original source/input directory into the pipeline working
directory. Useful for pulling in static manifests, CRs, or other files that
don't need templating.

```yaml
- name: copy-cr-files
  type: copy
  copy:
    files:
      include: ["manifests/**/*.yaml"]
      exclude: ["manifests/tmp/**"]
    dest: manifests/
```

| Field           | Description                                              | Default    |
|-----------------|----------------------------------------------------------|------------|
| `files.include` | Glob patterns for files to copy from the source directory | `["**/*"]` |
| `files.exclude` | Glob patterns for files to skip                          | `[]`       |
| `dest`          | Destination subdirectory within the working directory    | `"."`      |

Files are copied preserving their relative directory structure. Globs use
[doublestar](https://github.com/bmatcuk/doublestar) syntax (`**` for recursive
matching).

### `split`

Takes a multi-document YAML file and splits it into individual files.

```yaml
- name: split-manifests
  type: split
  split:
    input: kustomize-output.yaml
    by: resource
    outputDir: manifests/
```

| Field               | Description                                              | Default  |
|---------------------|----------------------------------------------------------|----------|
| `input`             | File path to the multi-document YAML to split            | required |
| `by`                | Splitting strategy (see below)                           | `"kind"` |
| `outputDir`         | Directory to write split files into                      | `"."`    |
| `fileNameTemplate`  | Go template for file paths (only with `custom` strategy) | ---      |
| `canonicalKeyOrder` | Reorder keys: apiVersion, kind, metadata first           | `true`   |

**Splitting strategies:**

| Strategy   | Layout                                                                                                   |
|------------|----------------------------------------------------------------------------------------------------------|
| `kind`     | One file per Kind (`deployment.yaml`, `service.yaml`). Multiple resources of the same Kind share a file. |
| `resource` | One file per resource (`deployment-api.yaml`, `service-api.yaml`).                                       |
| `group`    | Directories per API group (`apps/deployment-api.yaml`, `core/service-api.yaml`).                         |
| `kind-dir` | Directories per Kind, pluralized (`deployments/api.yaml`, `services/api.yaml`).                          |
| `custom`   | File paths from a Go template: `fileNameTemplate: "{{ .metadata.namespace }}/{{ .kind                    | lower }}-{{ .metadata.name }}.yaml"` |

## Sources

Each step can declare a `source` to fetch files into the working directory before
execution. A source is a single entry or a list of entries, applied in order:

```yaml
source:
  https: https://example.com/archive.tar.gz
  sha256: "abc123..."
  path: subdir/
```

```yaml
source:
  - oci: ghcr.io/myorg/manifests:v1.0.0
  - file: ./local-overrides
    path: patches/
```

| Scheme  | Description                                        | Example                                  |
|---------|----------------------------------------------------|------------------------------------------|
| `file`  | Local file or directory (relative to `.many.yaml`) | `file: ../shared/postgres.yaml`          |
| `https` | URL to a single file or tarball (auto-extracted)   | `https: https://example.com/v1.tar.gz`   |
| `oci`   | OCI image reference                                | `oci: ghcr.io/myorg/config:v1`          |
| `ocm`   | OCM component version                              | `ocm: github.com/myorg/component//res`  |
| `helm`  | Helm chart (requires `repo`)                       | `helm: my-chart`                         |

| Option      | Description                                                  |
|-------------|--------------------------------------------------------------|
| `path`      | Target subdirectory to place fetched files into              |
| `sha256`    | SHA-256 checksum for `https` sources (see below)             |
| `temporary` | Remove fetched files after the step completes                |
| `recursive` | Recursively resolve OCM references (OCM only)                |
| `repo`      | Helm chart repository URL (Helm only)                        |
| `version`   | Helm chart version (Helm only)                               |

**SHA-256 verification** --- when `sha256` is set to a hex digest, `many` verifies
the downloaded content matches before proceeding. On the first run you can leave
it empty (`sha256: ""`) to disable verification; `many` computes the actual
checksum and writes it back to the `.many.yaml` file so you can pin it. Use
`-no-sha256-update` to disable this writeback. Once pinned, any mismatch
(e.g. an upstream release change) fails the pipeline. Combined with a
`# renovate:` comment, Renovate updates both the URL and the checksum
automatically.

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
4. `template` steps modify files in-place. `kustomize-build`/`helm` steps write their
   output to a file (`outputFile`). `split` steps read a multi-document YAML file and
   write individual files.
5. `.many.yaml` files and the context file are removed from the output.

A failing step aborts its pipeline. Other pipelines continue. The exit code is
non-zero if any pipeline failed.

## Environment Variables

Use `-env-file` to load environment variables from a file
via [godotenv](https://github.com/joho/godotenv):

```bash
many -env-file .env -input ./infra -output-directory ./output
```

Loaded variables are available to `kustomize-build`, `kustomize-create`, and
`helm` steps via environment inheritance.
