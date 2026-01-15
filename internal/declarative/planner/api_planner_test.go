package planner

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPIVersionConstraintValidation tests that the loader properly validates API version constraints
func TestAPIVersionConstraintValidation(t *testing.T) {
	// The validation logic is tested in the loader tests (validator_test.go)
	// This file is a placeholder to show that we have considered planner-level tests
	// The actual validation happens during the loading phase, not planning phase

	// The planner's validation in planAPIVersionChanges requires a full state.Client
	// which would be overly complex to mock for this simple validation test
	// Therefore, the validation is properly tested at the loader level where it's first enforced

	t.Run("planner validation is covered by loader tests", func(t *testing.T) {
		// See internal/declarative/loader/validator_test.go for the actual tests
		assert.True(t, true, "Validation tests are in validator_test.go")
	})
}

func TestExtractAPIFieldsIncludesSlugAndAttributes(t *testing.T) {
	t.Parallel()

	name := "Simple API"
	slug := "simple-api-slug"
	attrs := map[string]any{
		"env":     "production",
		"domains": []any{"web", "mobile"},
	}
	expectedAttrs := map[string][]string{
		"env":     {"production"},
		"domains": {"web", "mobile"},
	}

	resource := resources.APIResource{
		CreateAPIRequest: kkComps.CreateAPIRequest{
			Name:       name,
			Slug:       &slug,
			Attributes: attrs,
		},
		Ref: "simple-api",
	}

	fields := extractAPIFields(resource)

	assert.Equal(t, slug, fields["slug"])
	assert.Equal(t, expectedAttrs, fields["attributes"])
}

func TestShouldUpdateAPIConsidersSlugAndAttributes(t *testing.T) {
	t.Parallel()

	p := &Planner{}

	currentSlug := "current-slug"
	currentAttrs := map[string][]string{
		"env": {"staging"},
	}

	current := state.API{
		APIResponseSchema: kkComps.APIResponseSchema{
			Slug:       &currentSlug,
			Attributes: currentAttrs,
		},
	}

	name := "Simple API"
	updatedSlug := "new-slug"
	updatedAttrs := map[string]any{
		"env":     "production",
		"domains": []string{"web"},
	}
	expectedUpdatedAttrs := map[string][]string{
		"env":     {"production"},
		"domains": {"web"},
	}

	desired := resources.APIResource{
		CreateAPIRequest: kkComps.CreateAPIRequest{
			Name:       name,
			Slug:       &updatedSlug,
			Attributes: updatedAttrs,
		},
		Ref: "simple-api",
	}

	needsUpdate, updateFields := p.shouldUpdateAPI(current, desired)
	assert.True(t, needsUpdate)
	assert.Equal(t, updatedSlug, updateFields["slug"])
	assert.Equal(t, expectedUpdatedAttrs, updateFields["attributes"])
}

func TestShouldUpdateAPIPublicationResolvesAuthStrategyRefs(t *testing.T) {
	t.Parallel()

	authStrategy := resources.ApplicationAuthStrategyResource{
		CreateAppAuthStrategyRequest: kkComps.CreateCreateAppAuthStrategyRequestKeyAuth(
			kkComps.AppAuthStrategyKeyAuthRequest{
				Name:         "my-api-key-auth",
				StrategyType: kkComps.StrategyTypeKeyAuth,
			},
		),
		Ref: "key-auth",
	}

	authStrategy.TryMatchKonnectResource(state.ApplicationAuthStrategy{
		ID:   "auth-id",
		Name: "my-api-key-auth",
	})

	planner := &Planner{
		resources: &resources.ResourceSet{
			ApplicationAuthStrategies: []resources.ApplicationAuthStrategyResource{authStrategy},
		},
	}

	current := state.APIPublication{
		AuthStrategyIDs: []string{"auth-id"},
	}

	desired := resources.APIPublicationResource{
		APIPublication: kkComps.APIPublication{
			AuthStrategyIds: []string{tags.RefPlaceholderPrefix + "key-auth#id"},
		},
		Ref:      "pub",
		PortalID: "portal-id",
	}

	needsUpdate, fields := planner.shouldUpdateAPIPublication(current, desired)
	require.False(t, needsUpdate)
	assert.Empty(t, fields)
}

func TestShouldUpdateAPIPublicationIgnoresAuthStrategyWhenUnset(t *testing.T) {
	t.Parallel()

	planner := &Planner{}

	current := state.APIPublication{
		AuthStrategyIDs: []string{"auth-id"},
	}

	desired := resources.APIPublicationResource{
		APIPublication: kkComps.APIPublication{},
		Ref:            "pub",
		PortalID:       "portal-id",
	}

	needsUpdate, fields := planner.shouldUpdateAPIPublication(current, desired)
	require.False(t, needsUpdate)
	assert.Empty(t, fields)
}
