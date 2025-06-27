//go:build integration
// +build integration

package declarative_test

import (
	"testing"
)

func TestPlanGeneration_CrossResourceReferences(t *testing.T) {
	t.Skip("Skipping cross-resource reference test until auth strategy support is implemented")
}

func TestPlanGeneration_UnresolvedReferences(t *testing.T) {
	t.Skip("Skipping unresolved reference test until auth strategy support is implemented")
}

func TestPlanGeneration_InvalidReference(t *testing.T) {
	t.Skip("Skipping invalid reference test until auth strategy support is implemented")
}