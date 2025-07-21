package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResourceValidation_RejectsColonsInRefs(t *testing.T) {
	tests := []struct {
		name         string
		resource     ResourceValidator
		expectedErr  string
	}{
		{
			name: "portal with colon in ref",
			resource: &PortalResource{
				Ref: "my:portal",
			},
			expectedErr: "invalid portal ref: ref cannot contain colons (:)",
		},
		{
			name: "api with colon in ref",
			resource: &APIResource{
				Ref: "api:v1",
			},
			expectedErr: "invalid API ref: ref cannot contain colons (:)",
		},
		{
			name: "auth strategy with colon in ref",
			resource: &ApplicationAuthStrategyResource{
				Ref: "auth:basic",
			},
			expectedErr: "invalid application auth strategy ref: ref cannot contain colons (:)",
		},
		{
			name: "control plane with colon in ref",
			resource: &ControlPlaneResource{
				Ref: "cp:prod",
			},
			expectedErr: "invalid control plane ref: ref cannot contain colons (:)",
		},
		{
			name: "api version with colon in ref",
			resource: &APIVersionResource{
				Ref: "version:1.0",
			},
			expectedErr: "invalid API version ref: ref cannot contain colons (:)",
		},
		{
			name: "api publication with colon in ref",
			resource: &APIPublicationResource{
				Ref:      "pub:portal",
				PortalID: "some-portal", // Required field
			},
			expectedErr: "invalid API publication ref: ref cannot contain colons (:)",
		},
		{
			name: "api implementation with colon in ref",
			resource: &APIImplementationResource{
				Ref: "impl:kong",
			},
			expectedErr: "invalid API implementation ref: ref cannot contain colons (:)",
		},
		{
			name: "api document with colon in ref",
			resource: &APIDocumentResource{
				Ref: "doc:guide",
			},
			expectedErr: "invalid API document ref: ref cannot contain colons (:)",
		},
		{
			name: "portal page with colon in ref",
			resource: &PortalPageResource{
				Ref: "page:home",
			},
			expectedErr: "invalid page ref: ref cannot contain colons (:)",
		},
		{
			name: "portal snippet with colon in ref",
			resource: &PortalSnippetResource{
				Ref:  "snippet:header",
				Name: "header", // Required field
			},
			expectedErr: "invalid snippet ref: ref cannot contain colons (:)",
		},
		{
			name: "portal customization with colon in ref",
			resource: &PortalCustomizationResource{
				Ref: "custom:theme",
			},
			expectedErr: "invalid customization ref: ref cannot contain colons (:)",
		},
		{
			name: "portal custom domain with colon in ref",
			resource: &PortalCustomDomainResource{
				Ref: "domain:example",
			},
			expectedErr: "invalid custom domain ref: ref cannot contain colons (:)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.resource.Validate()
			assert.Error(t, err)
			assert.Equal(t, tt.expectedErr, err.Error())
		})
	}
}

func TestResourceValidation_AcceptsValidRefs(t *testing.T) {
	tests := []struct {
		name     string
		resource ResourceValidator
	}{
		{
			name: "portal with valid ref",
			resource: &PortalResource{
				Ref: "my-portal",
			},
		},
		{
			name: "api with underscore",
			resource: &APIResource{
				Ref: "api_v1",
			},
		},
		{
			name: "auth strategy with numbers",
			resource: &ApplicationAuthStrategyResource{
				Ref: "auth123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.resource.Validate()
			// We only check ref validation passed; other required fields might still cause errors
			if err != nil {
				assert.NotContains(t, err.Error(), "ref")
			}
		})
	}
}