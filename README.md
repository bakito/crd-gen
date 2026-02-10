# crd-gen

This repository provides three powerful tools for working with Kubernetes Custom Resource Definitions (CRDs):

- [`generate-crd-api`](./cmd/generate-crd-api): Generate Go API types from CRD YAML files.
- [`extract-crd-api`](./cmd/extract-crd-api): Extract Go API types from existing modules for selected CRDs.
- [`flatten-crd-api`](./cmd/flatten-crd-api): Flatten Go API types by extracting specific types and their dependencies into a single file.

Below you’ll find documentation and usage examples for these tools.

---

## generate-crd-api

### Purpose

`generate-crd-api` is used to automatically generate Go API types from one or more Kubernetes CRD YAML files. This simplifies integration testing, code generation, and documentation.

### Installation

Requires **Go 1.24+**.

```bash
go get -tool github.com/bakito/crd-gen/cmd/generate-crd-api@latest
```

### Usage

You can use `go generate` directives to invoke the tool. Example:

```go
//go:build generate
// +build generate

//go:generate go tool generate-crd-api --target .testdata/ \
    --crd testdata/certificates.cert-manager.io.yaml \
    --crd testdata/certificaterequests.cert-manager.io.yaml \
    --crd testdata/clusterissuers.cert-manager.io.yaml
```

#### Flags

- `--target <dir>`: Directory to write generated Go files to.
- `--crd <file>`: Path to a CRD YAML file. Can be specified multiple times.

---

## extract-crd-api

### Purpose

`extract-crd-api` extracts Go API types for CRDs from existing Go modules. It enables you to reuse and synchronize CRD types from upstream or third-party providers.

### Installation

Requires **Go 1.24+**.

```bash
go get -tool github.com/bakito/crd-gen/cmd/extract-crd-api@latest
```

### Usage

Use `go generate` to invoke the tool and extract API types from modules.

```go
//go:build generate
// +build generate

//go:generate go tool extract-crd-api \
    --module "github.com/upbound/provider-vault@v2.1.1" \
    --use-git --clear \
    --path apis/vault/v1alpha1      --target ./apis/vault211/vault      --exclude .*\.managed.go
//go:generate go tool extract-crd-api \
    --module "github.com/upbound/provider-vault@v2.1.1" \
    --use-git --clear \
    --path apis/kubernetes/v1alpha1 --target ./apis/vault211/kubernetes --exclude .*\.managed.go
```

#### Flags

- `--module <module@version>`: Go module and version to extract API types from.
- `--use-git`: Use git to fetch the module.
- `--clear`: Clear the target directory before extraction.
- `--path <dir>`: Path inside the module to extract API types from.
- `--target <dir>`: Target directory for extracted files.
- `--exclude <pattern>`: Regex pattern for files to exclude.

---

## flatten-crd-api

### Purpose

`flatten-crd-api` extracts specific Go types and their internal dependencies from a package and flattens them into a single Go file. It preserves references to standard library types and Kubernetes `metav1` types while bringing all other required struct definitions into the output file. This is particularly useful for creating lightweight, self-contained versions of complex API types.

### Installation

Requires **Go 1.24+**.

```bash
go get -tool github.com/bakito/crd-gen/cmd/flatten-crd-api@latest
```

### Usage

Example of flattening an `ExternalSecret` type from the `external-secrets` project:

```go
//go:build generate
// +build generate

//go:generate go tool flatten-crd-api \
    --src github.com/external-secrets/external-secrets/apis/externalsecrets/v1beta1 \
    --type ExternalSecret \
    --out ./apis/externalsecrets/v1beta1/zz_generated.flattened.go \
    --pkg v1beta1
```

#### Flags

- `--src <path>`: Source package path or file.
- `--type <names>`: Comma-separated list of struct names to extract.
- `--out <file>`: Output file path.
- `--pkg <name>`: Output package name (default: `generated`).
