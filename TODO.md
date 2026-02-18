# TODO

## Core Refactoring

- [x] Rename module and binary from `enver` to `many` (binary), `many-templates` (module)
- [x] Rename `cmd/enver/` to `cmd/many/`
- [x] Rename `Processing` type to `Pipeline` with new schema (pipeline steps, context)
- [x] Update `.many.yaml` as the config filename (replaces `processing.yaml`)
- [x] Remove legacy `processing.Process()` and `processing.LoadProcessing()` code

## `.many.yaml` Schema & Parsing

- [x] Define Go types for `.many.yaml`: Pipeline, Step, TemplateConfig, KustomizeConfig, HelmConfig, SplitConfig
- [x] Parse and validate `.many.yaml` files (unknown step types, duplicate step names, missing required fields)
- [x] Load and merge context: CLI `--context-file` as base, pipeline-local context overrides top-level keys

## Directory Discovery

- [x] Walk input directory tree, collect all `.many.yaml` file paths
- [x] `--max-depth` flag to limit recursion depth
- [x] Sort discovered pipelines by path depth (parents before children)
- [x] Copy source tree to output directory before processing
- [x] Remove `.many.yaml` files from output after all pipelines complete

## Pipeline Execution Engine

- [x] Sequential step executor: iterate steps, dispatch by type, pass step outputs forward
- [x] Step output registry: steps that produce streams (kustomize, helm) store output keyed by step name
- [x] Step `input` resolution: validate referenced step exists and has output
- [x] Per-pipeline error handling: abort pipeline on step failure, continue other pipelines
- [x] Summary at end: list succeeded/failed pipelines

## Step: `template`

- [x] Refactor existing `processFile` into a step implementation
- [x] Accept `files.include` / `files.exclude` from step config
- [x] Operate in-place within the output directory (current behaviour, just wired into step interface)

## Step: `kustomize`

- [x] Shell out to `kustomize build <dir>`, capture stdout as multi-document YAML stream
- [x] Support `enableHelm` flag (`--enable-helm`)
- [x] Capture stderr for error reporting
- [x] Store output stream in step output registry

## Step: `helm`

- [x] Shell out to `helm template <releaseName> <chart>` with `--namespace`, `--values`, `--set` flags
- [x] Support multiple `valuesFiles`
- [x] Capture stderr for error reporting
- [x] Store output stream in step output registry

## Step: `split`

- [x] Parse multi-document YAML stream into individual manifests (handle `---` separators)
- [x] Extract `apiVersion`, `kind`, `metadata.name`, `metadata.namespace` from each manifest
- [x] Implement splitting strategies:
  - [x] `kind` — group by Kind, one file per Kind
  - [x] `resource` — one file per resource, `<kind>-<name>.yaml`
  - [x] `group` — directories per API group, files per resource
  - [x] `kind-dir` — directories per Kind, files per resource name
  - [x] `custom` — file path from Go template with full manifest data
- [x] Write split files to `outputDir`, creating directories as needed
- [x] Handle naming collisions (e.g. same kind+name in different namespaces for `resource` strategy)

## CLI Updates

- [x] Add `--context-file` flag
- [x] Add `--max-depth` flag
- [x] Keep `--processing` for single-file backward-compatible mode
- [x] When no `--processing` given, run in directory discovery mode

## Testing

- [x] Unit tests for YAML stream splitting and each split strategy
- [x] Unit tests for `.many.yaml` parsing and validation
- [x] Unit tests for context merging (global + local)
- [x] Integration test: template step with testdata
- [x] Integration test: full pipeline (template -> kustomize -> split) with testdata
- [ ] Add testdata `.many.yaml` examples for each step type
