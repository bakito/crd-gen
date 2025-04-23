//go:build generate
// +build generate

// Remove existing and generate new input manifests
//go:generate go run cmd/extract-crd-api/main.go -module "github.com/upbound/provider-vault@v2.1.1" -use-git -clear -path apis/vault/v1alpha1      -target ./apis/vault211/vault      -exclude .*\.managed.go -exclude .*\.managedlist.go -exclude .*_terraformed.go -exclude .*\.conversion_hubs.go -exclude .*\.resolvers.go
//go:generate go run cmd/extract-crd-api/main.go -module "github.com/upbound/provider-vault@v2.1.1" -use-git -clear -path apis/kubernetes/v1alpha1 -target ./apis/vault211/kubernetes -exclude .*\.managed.go -exclude .*\.managedlist.go -exclude .*_terraformed.go -exclude .*\.conversion_hubs.go -exclude .*\.resolvers.go

package crd_gen
