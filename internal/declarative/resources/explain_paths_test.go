package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExplainResourcePathsAllResolve(t *testing.T) {
	paths := ExplainResourcePaths()
	require.NotEmpty(t, paths)
	assert.IsIncreasing(t, paths)

	for _, path := range paths {
		_, err := ResolveExplainSubject(path)
		assert.NoErrorf(t, err, "listed path %q must resolve", path)
	}
}

func TestExplainResourcePathsOmitsGroupingRoots(t *testing.T) {
	// organization and analytics only resolve with a child segment, so listing
	// them bare would advertise a path that errors.
	paths := ExplainResourcePaths()
	assert.NotContains(t, paths, "organization")
	assert.NotContains(t, paths, "analytics")
	assert.Contains(t, paths, "api")
}
