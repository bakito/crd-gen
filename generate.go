//go:build generate
// +build generate

//go:generate go run ./cmd/extract-crd-api/main.go --module "github.com/upbound/provider-vault@v2.1.1" --use-git --clear --path apis/vault/v1alpha1      --target testdata/apis/vault      --exclude .*\.managed.go --exclude .*\.managedlist.go --exclude .*_terraformed.go --exclude .*\.conversion_hubs.go --exclude .*\.resolvers.go
//go:generate go run ./cmd/extract-crd-api/main.go --module "github.com/upbound/provider-vault@v2.1.1" --use-git --clear --path apis/kubernetes/v1alpha1 --target testdata/apis/kubernetes --exclude .*\.managed.go --exclude .*\.managedlist.go --exclude .*_terraformed.go --exclude .*\.conversion_hubs.go --exclude .*\.resolvers.go

// run the generator
//go:generate go run ./cmd/generate-crd-api --target testdata/apis --crd testdata/certificates.cert-manager.io.yaml --crd testdata/certificaterequests.cert-manager.io.yaml --crd testdata/clusterissuers.cert-manager.io.yaml

// Generate deepcopy methodsets and CRD manifests
//go:generate go tool controller-gen object paths=./testdata/v1

package gen
