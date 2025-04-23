# Include toolbox tasks
include ./.toolbox.mk

lint: tb.golangci-lint
	$(TB_GOLANGCI_LINT) run --fix

# Run go mod tidy
tidy:
	go mod tidy

# Run tests
test: tidy lint
	go test ./... -coverprofile=coverage.out

generate:
	go generate