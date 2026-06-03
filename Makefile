# Include toolbox tasks
include ./.toolbox.mk

.PHONY: lint
lint: tb.golangci-lint
	$(TB_GOLANGCI_LINT) run --fix

# Run go mod tidy
.PHONY: tidy
tidy:
	go mod tidy

# Run tests
.PHONY: test
test: tidy lint test-ci

.PHONY: test-ci
test-ci:
	go test ./... -coverprofile=coverage.out

.PHONY: fuzz-compare
fuzz-compare: generate-compare
	go test -fuzz=Fuzz -fuzztime=30s ./cmd/compare-gen/test/...

.PHONY: generate-ci
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
	  --target tmp/apis/capsule --pointer \
		  --crd testdata/capsule.clastix.io_tenants.yaml
	go tool controller-gen object paths=./tmp/apis/capsule/v1beta2 --load-build-tags=
	go run ./cmd/generate-crd-api \
	  --target tmp/apis/argocd --pointer \
		  --crd testdata/applications.argoproj.io.yaml
	go tool controller-gen object paths=./tmp/apis/argocd/v1alpha1 --load-build-tags=
.PHONY: generate
generate: generate-ci
	rm -Rf tmp/apis

.PHONY: generate-compare
generate-compare:
	go run ./cmd/compare-gen/*.go \
		-input cmd/compare-gen/test/model.go \
		-output cmd/compare-gen/test/model_compare.go \
		-header=false
	echo "" >>  cmd/compare-gen/test/model_compare.go
	sed -i '1s;^;//nolint:staticcheck // S1008 "Simplify returning boolean expression" is hard to manage in generated code.\n;' cmd/compare-gen/test/model_compare.go