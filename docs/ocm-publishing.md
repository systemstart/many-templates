# OCM Component Publishing Workflow

This document describes how to publish template trees as OCM components and consume them with `many`.

## 1. Create a Component Constructor

Create a `component-constructor.yaml` that packages your template directory as a `directoryTree` resource:

```yaml
components:
  - name: github.com/myorg/my-templates
    version: v1.0.0
    provider:
      name: myorg
    resources:
      - name: templates
        type: directoryTree
        input:
          type: dir
          path: ./templates
```

## 2. Build and Publish

Build a Common Transport Format (CTF) archive, then transfer it to an OCI registry:

```bash
# Build the CTF archive
ocm add componentversions --create ctf-archive component-constructor.yaml

# Transfer to an OCI registry
ocm transfer ctf ctf-archive ghcr.io/myorg/ocm
```

This publishes the component as `ghcr.io/myorg/ocm//github.com/myorg/my-templates:v1.0.0`.

## 3. Consume with many

Use the published component as an input source:

```bash
many -input ocm://ghcr.io/myorg/ocm//github.com/myorg/my-templates:v1.0.0 \
     -output-directory output/
```

Or reference it in a `.many.yaml` pipeline:

```yaml
source:
  ocm: ghcr.io/myorg/ocm//github.com/myorg/my-templates:v1.0.0
pipeline:
  - name: render
    type: template
    template:
      files:
        include: ["**/*.yaml"]
```

### Recursive Components

If your component has references to other components, use `recursive: true` to download all referenced resources:

```yaml
source:
  ocm: ghcr.io/myorg/ocm//github.com/myorg/my-templates:v1.0.0
  recursive: true
pipeline:
  - name: render
    type: template
    template:
      files:
        include: ["**/*.yaml"]
```

### Remote Instance Input

Instances can reference remote OCM components directly:

```yaml
instances:
  - name: prod
    input: ocm://ghcr.io/myorg/ocm//github.com/myorg/my-templates:v1.0.0
    output: prod/
    context:
      env: production
```
