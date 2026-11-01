package loader

import (
	"encoding/json"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/require"
)

func TestPrincipalMetadataModesValidateAndPreserveEmptyMaps(t *testing.T) {
	for _, mode := range []string{"merge", "sync"} {
		input := `identity:
  directories:
    - ref: people
      _external: {id: 4078f559-063b-4f10-9098-9e6553b21d9b}
      principals:
        - ref: worker
          metadata_mode: ` + mode + `
          metadata: {}
`
		rs, err := New().LoadFile(writeLoaderTestFile(t, input))
		require.NoError(t, err)
		value := rs.Identity.Principals[0]
		require.NotNil(t, value.Metadata)
		data, err := json.Marshal(value)
		require.NoError(t, err)
		var saved resources.IdentityPrincipalResource
		require.NoError(t, json.Unmarshal(data, &saved))
		require.NoError(t, saved.Validate())
		require.NotNil(t, saved.Metadata)
		require.Equal(t, mode, saved.MetadataMode)
	}
	for _, fields := range []string{
		"metadata_mode: invalid\n          metadata: {}",
		"metadata_mode: null\n          metadata: {}",
		"metadata_mode: merge",
		"metadata_mode: sync\n          metadata: null",
		"metadata_mode: merge\n          metadata: {}\n          _external: {id: 5078f559-063b-4f10-9098-9e6553b21d9b}",
	} {
		input := `identity:
  directories:
    - ref: people
      _external: {id: 4078f559-063b-4f10-9098-9e6553b21d9b}
      principals:
        - ref: worker
          ` + fields + "\n"
		_, err := New().LoadFile(writeLoaderTestFile(t, input))
		require.Error(t, err)
	}
}
