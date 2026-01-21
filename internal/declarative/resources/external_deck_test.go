package resources

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/require"
)

func TestExternalBlockValidateDeckRequires(t *testing.T) {
	selectorName := &ExternalSelector{MatchFields: map[string]string{"name": "svc"}}

	tests := []struct {
		name     string
		external *ExternalBlock
		wantErr  bool
	}{
		{
			name: "valid gateway placeholder",
			external: &ExternalBlock{
				Selector: selectorName,
				Requires: &ExternalRequires{
					Deck: []DeckStep{{Args: []string{"gateway", "{{kongctl.mode}}"}}},
				},
			},
		},
		{
			name: "requires deck rejects id",
			external: &ExternalBlock{
				ID:       "abc",
				Selector: selectorName,
				Requires: &ExternalRequires{
					Deck: []DeckStep{{Args: []string{"gateway", "sync"}}},
				},
			},
			wantErr: true,
		},
		{
			name: "requires selector name",
			external: &ExternalBlock{
				Requires: &ExternalRequires{
					Deck: []DeckStep{{Args: []string{"gateway", "sync"}}},
				},
			},
			wantErr: true,
		},
		{
			name: "selector must be name only",
			external: &ExternalBlock{
				Selector: &ExternalSelector{MatchFields: map[string]string{"id": "svc"}},
				Requires: &ExternalRequires{
					Deck: []DeckStep{{Args: []string{"gateway", "sync"}}},
				},
			},
			wantErr: true,
		},
		{
			name: "selector cannot include extra fields",
			external: &ExternalBlock{
				Selector: &ExternalSelector{MatchFields: map[string]string{"name": "svc", "env": "dev"}},
				Requires: &ExternalRequires{
					Deck: []DeckStep{{Args: []string{"gateway", "sync"}}},
				},
			},
			wantErr: true,
		},
		{
			name: "placeholder only allowed for gateway",
			external: &ExternalBlock{
				Selector: selectorName,
				Requires: &ExternalRequires{
					Deck: []DeckStep{{Args: []string{"file", "{{kongctl.mode}}"}}},
				},
			},
			wantErr: true,
		},
		{
			name: "gateway verb restricted",
			external: &ExternalBlock{
				Selector: selectorName,
				Requires: &ExternalRequires{
					Deck: []DeckStep{{Args: []string{"gateway", "diff"}}},
				},
			},
			wantErr: true,
		},
		{
			name: "placeholder must be gateway verb",
			external: &ExternalBlock{
				Selector: selectorName,
				Requires: &ExternalRequires{
					Deck: []DeckStep{{Args: []string{"gateway", "sync", "{{kongctl.mode}}"}}},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		err := tt.external.Validate()
		if tt.wantErr {
			require.Error(t, err, tt.name)
			continue
		}
		require.NoError(t, err, tt.name)
	}
}

func TestControlPlaneValidateRejectsDeckRequires(t *testing.T) {
	cp := ControlPlaneResource{
		CreateControlPlaneRequest: kkComps.CreateControlPlaneRequest{
			Name: "cp",
		},
		Ref: "cp",
		External: &ExternalBlock{
			Selector: &ExternalSelector{MatchFields: map[string]string{"name": "cp"}},
			Requires: &ExternalRequires{
				Deck: []DeckStep{{Args: []string{"gateway", "sync"}}},
			},
		},
	}

	require.ErrorContains(t, cp.Validate(), "_external.requires.deck")
}

func TestPortalValidateRejectsDeckRequires(t *testing.T) {
	portal := PortalResource{
		Ref: "portal",
		External: &ExternalBlock{
			Selector: &ExternalSelector{MatchFields: map[string]string{"name": "portal"}},
			Requires: &ExternalRequires{
				Deck: []DeckStep{{Args: []string{"gateway", "sync"}}},
			},
		},
	}

	require.ErrorContains(t, portal.Validate(), "_external.requires.deck")
}
