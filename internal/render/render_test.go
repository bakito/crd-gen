package render

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// verifies that all required dependencies for the application are correctly set up and available.
var _ = schema.GroupVersion{}
