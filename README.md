## generate-crd-api

## Usage

go 1.24+

```bash
go get -tool github.com/bakito/crd-gen/cmd/generate-crd-api@latest
```

```go
//go:build generate
// +build generate

//go:generate go tool generate-crd-api -target . -crd certificates.cert-manager.io.yaml -crd certificaterequests.cert-manager.io.yaml -crd clusterissuers.cert-manager.io.yaml

package crd_gen
```

## extract-crd-api

## Usage

go 1.24+

```bash
go get -tool github.com/bakito/crd-gen/cmd/extract-crd-api@latest
```

```go
//go:build generate
// +build generate

//go:generate go tool extract-crd-api -module "github.com/upbound/provider-vault@v2.1.1" -use-git -clear -path apis/vault/v1alpha1      -target ./apis/vault211/vault/v1alpha1     -exclude .*\.managed.go -exclude .*\.managedlist.go -exclude .*_terraformed.go -exclude .*\.conversion_hubs.go -exclude .*\.resolvers.go
//go:generate go tool extract-crd-api -module "github.com/upbound/provider-vault@v2.1.1" -use-git -clear -path apis/kubernetes/v1alpha1 -target ./apis/vault211/kubernetes/v1alpha1 -exclude .*\.managed.go -exclude .*\.managedlist.go -exclude .*_terraformed.go -exclude .*\.conversion_hubs.go -exclude .*\.resolvers.go

package crd_gen
```
