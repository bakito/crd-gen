package openapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_newUniqFieldName(t *testing.T) {
	cr := &CustomResource{
		Kind: "TestCase",
	}

	r := &CustomResources{
		structNames: make(map[string]bool),
	}
	un := r.newUniqFieldName(cr, "Spec", true, "TestCase")
	assert.Equal(t, "TestCaseSpec", un)
	un = r.newUniqFieldName(cr, "Status", true, "TestCase")
	assert.Equal(t, "TestCaseStatus", un)
	un = r.newUniqFieldName(cr, "Foo", false, "TestCase.Spec")
	assert.Equal(t, "Foo", un)
	un = r.newUniqFieldName(cr, "Foo", false, "TestCase.Status")
	assert.Equal(t, "TestCaseFoo", un)
	un = r.newUniqFieldName(cr, "Foo", false, "TestCase.Spec.Bar")
	assert.Equal(t, "BarFoo", un)
	un = r.newUniqFieldName(cr, "Foo", false, "TestCase.Status.Bar")
	assert.Equal(t, "StatusBarFoo", un)
	un = r.newUniqFieldName(cr, "Foo", false, "TestCase.Status.Bar")
	assert.Equal(t, "TestCaseStatusBarFoo", un)
	un = r.newUniqFieldName(cr, "Foo", false, "TestCase.Status.Bar")
	assert.Equal(t, "Foo_f8559662a4db3e0bf226e9df87cdcfb1", un)
}
