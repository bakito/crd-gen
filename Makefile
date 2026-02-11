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
	go run ./cmd/generate-crd-api \
	  --target tmp/apis --pointer \
	  --crd testdata/capsule.clastix.io_tenants.yaml
generate: generate-ci
	rm -Rf tmp/apis
