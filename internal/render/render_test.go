package render

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// verifies that all required dependencies for the application are correctly set up and available.
	_ = scheme.Builder{}
	_ = schema.GroupVersion{}
)
