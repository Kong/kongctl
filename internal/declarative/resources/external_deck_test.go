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
			name: "valid requires deck",
			external: &ExternalBlock{
				Selector: selectorName,
				Requires: &ExternalRequires{
					Deck: &DeckRequires{Files: []string{"gateway-service.yaml"}},
				},
			},
		},
		{
			name: "valid requires deck with flags",
			external: &ExternalBlock{
				Selector: selectorName,
				Requires: &ExternalRequires{
					Deck: &DeckRequires{
						Files: []string{"gateway-service.yaml"},
						Flags: []string{"--select-tag=kongctl"},
					},
				},
			},
		},
		{
			name: "requires deck rejects id",
			external: &ExternalBlock{
				ID:       "abc",
				Selector: selectorName,
				Requires: &ExternalRequires{
					Deck: &DeckRequires{Files: []string{"gateway-service.yaml"}},
				},
			},
			wantErr: true,
		},
		{
			name: "requires selector name",
			external: &ExternalBlock{
				Requires: &ExternalRequires{
					Deck: &DeckRequires{Files: []string{"gateway-service.yaml"}},
				},
			},
			wantErr: true,
		},
		{
			name: "selector must be name only",
			external: &ExternalBlock{
				Selector: &ExternalSelector{MatchFields: map[string]string{"id": "svc"}},
				Requires: &ExternalRequires{
					Deck: &DeckRequires{Files: []string{"gateway-service.yaml"}},
				},
			},
			wantErr: true,
		},
		{
			name: "selector cannot include extra fields",
			external: &ExternalBlock{
				Selector: &ExternalSelector{MatchFields: map[string]string{"name": "svc", "env": "dev"}},
				Requires: &ExternalRequires{
					Deck: &DeckRequires{Files: []string{"gateway-service.yaml"}},
				},
			},
			wantErr: true,
		},
		{
			name: "requires deck needs files",
			external: &ExternalBlock{
				Selector: selectorName,
				Requires: &ExternalRequires{
					Deck: &DeckRequires{},
				},
			},
			wantErr: true,
		},
		{
			name: "requires deck file cannot be a flag",
			external: &ExternalBlock{
				Selector: selectorName,
				Requires: &ExternalRequires{
					Deck: &DeckRequires{Files: []string{"--foo"}},
				},
			},
			wantErr: true,
		},
		{
			name: "requires deck flag must be a flag",
			external: &ExternalBlock{
				Selector: selectorName,
				Requires: &ExternalRequires{
					Deck: &DeckRequires{
						Files: []string{"gateway-service.yaml"},
						Flags: []string{"not-a-flag"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "requires deck flag cannot include konnect auth",
			external: &ExternalBlock{
				Selector: selectorName,
				Requires: &ExternalRequires{
					Deck: &DeckRequires{
						Files: []string{"gateway-service.yaml"},
						Flags: []string{"--konnect-token=abc"},
					},
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
				Deck: &DeckRequires{Files: []string{"gateway-service.yaml"}},
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
				Deck: &DeckRequires{Files: []string{"gateway-service.yaml"}},
			},
		},
	}

	require.ErrorContains(t, portal.Validate(), "_external.requires.deck")
}
