package helpers_test

import (
	"errors"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/konnect/helpers"
	fixture "github.com/kong/kongctl/test/konnect"
	"github.com/stretchr/testify/require"
)

func TestIdentityDirectoryPaginationDoesNotFilterOrReturnPartialResults(t *testing.T) {
	for _, failure := range []string{"", "error", "repeated", "missing"} {
		t.Run(failure, func(t *testing.T) {
			api := &fixture.DirectoryAPI{}
			api.List = func(page *kkComps.CursorPageParameters) (*kkOps.ListKongDirectoriesResponse, error) {
				require.Equal(t, int64(100), *page.Size)
				if page.After == nil {
					return &kkOps.ListKongDirectoriesResponse{ListKongDirectories: &kkComps.ListKongDirectories{
						Meta: kkComps.CursorMeta{Page: kkComps.CursorMetaPage{
							Next: new("https://example.test/v2/directories?page%5Bafter%5D=cursor-two"),
						}},
					}}, nil // Sparse pages may have a continuation.
				}
				require.Equal(t, "cursor-two", *page.After)
				switch failure {
				case "error":
					return nil, errors.New("read failed")
				case "missing":
					return nil, nil
				}
				next := (*string)(nil)
				if failure == "repeated" {
					next = new("cursor-two")
				}
				return &kkOps.ListKongDirectoriesResponse{ListKongDirectories: &kkComps.ListKongDirectories{
					Data: []kkComps.KongDirectory{fixture.Directory("default", "", false)},
					Meta: kkComps.CursorMeta{Page: kkComps.CursorMetaPage{Next: next}},
				}}, nil
			}
			values, err := helpers.ListIdentityDirectories(t.Context(), api, 0)
			if failure == "" {
				require.NoError(t, err)
				require.Len(t, values, 1)
			} else {
				require.Error(t, err)
				require.Nil(t, values)
			}
		})
	}
}

func TestIdentityDirectoryNameMatchingAndFreshResolution(t *testing.T) {
	current := fixture.Directory("default", "", false)
	api := &fixture.DirectoryAPI{Directories: []kkComps.KongDirectory{current}, Current: current}
	resolved, err := helpers.ResolveIdentityDirectory(t.Context(), api, "default", 10)
	require.NoError(t, err)
	require.Equal(t, current.ID, resolved.ID)
	require.Equal(t, 1, api.Gets)
	_, err = helpers.ResolveIdentityDirectory(t.Context(), api, "Default", 10)
	require.ErrorContains(t, err, "not found")
	_, err = helpers.FindIdentityDirectory([]kkComps.KongDirectory{current, current}, "default")
	require.ErrorContains(t, err, "multiple")
	current.TTLSecs = nil
	_, err = helpers.IdentityDirectoryReplacement(current)
	require.ErrorContains(t, err, "not fully observable")
}
