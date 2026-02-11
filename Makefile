# Include toolbox tasks
include ./.toolbox.mk

lint: tb.golangci-lint
	$(TB_GOLANGCI_LINT) run --fix

# Run go mod tidy
tidy:
	go mod tidy

# Run tests
test: tidy lint test-ci

test-ci:
	go test ./... -coverprofile=coverage.out

generate-ci:
	rm -Rf tmp/apis
	go run ./cmd/extract-crd-api/main.go --module "github.com/upbound/provider-vault@v2.1.1" \
	  --use-git --clear --path apis/vault/v1alpha1 \
	  --target tmp/apis/vault \
	  --exclude .*\.managed.go \
	  --exclude .*\.managedlist.go \
	  --exclude .*_terraformed.go \
	  --exclude .*\.conversion_hubs.go \
	  --exclude .*\.resolvers.go
	go run ./cmd/generate-crd-api \
	  --target tmp/apis --pointer \
		  --crd testdata/capsule.clastix.io_tenants.yaml
	go tool controller-gen object paths=./tmp/apis/v1beta2
generate: generate-ci
	rm -Rf tmp/apis
