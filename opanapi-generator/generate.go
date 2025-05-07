//go:build generate
// +build generate

// run the generator
//go:generate go run main.go mycrd.yaml v1alpha1/certificate.go

// Generate deepcopy methodsets and CRD manifests
//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen object paths=./v1alpha1

package main

import (
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen" //nolint:typecheck
)
