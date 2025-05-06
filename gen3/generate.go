//go:build generate
// +build generate

// Generate deepcopy methodsets and CRD manifests
//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen object paths=./v1alpha1

package v1alpha1

import (
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen" //nolint:typecheck
)
