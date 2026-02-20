package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractNameAndID(t *testing.T) {
	tests := []struct {
		name         string
		konnectRes   interface{}
		embeddedName string
		wantName     string
		wantID       string
	}{
		{
			name: "direct fields",
			konnectRes: struct {
				Name string
				ID   string
			}{Name: "test-service", ID: "uuid-123"},
			embeddedName: "",
			wantName:     "test-service",
			wantID:       "uuid-123",
		},
		{
			name: "embedded struct",
			konnectRes: struct {
				APIResponseSchema struct {
					Name string
					ID   string
				}
			}{APIResponseSchema: struct {
				Name string
				ID   string
			}{Name: "my-api", ID: "api-456"}},
			embeddedName: "APIResponseSchema",
			wantName:     "my-api",
			wantID:       "api-456",
		},
		{
			name: "pointer fields",
			konnectRes: func() interface{} {
				name := "ptr-name"
				id := "ptr-id"
				return struct {
					Name *string
					ID   *string
				}{Name: &name, ID: &id}
			}(),
			embeddedName: "",
			wantName:     "ptr-name",
			wantID:       "ptr-id",
		},
		{
			name:         "empty struct",
			konnectRes:   struct{}{},
			embeddedName: "",
			wantName:     "",
			wantID:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, id := extractNameAndID(tt.konnectRes, tt.embeddedName)
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
			if id != tt.wantID {
				t.Errorf("id = %q, want %q", id, tt.wantID)
			}
		})
	}
}

func TestTryMatchByField(t *testing.T) {
	tests := []struct {
		name          string
		konnectRes    any
		fieldName     string
		expectedValue string
		wantID        string
	}{
		{
			name: "match by direct string field",
			konnectRes: struct {
				Slug string
				ID   string
			}{Slug: "my-slug", ID: "id-1"},
			fieldName:     "Slug",
			expectedValue: "my-slug",
			wantID:        "id-1",
		},
		{
			name: "no match when value differs",
			konnectRes: struct {
				Slug string
				ID   string
			}{Slug: "other-slug", ID: "id-2"},
			fieldName:     "Slug",
			expectedValue: "my-slug",
			wantID:        "",
		},
		{
			name: "no match when ID is empty",
			konnectRes: struct {
				Name string
				ID   string
			}{Name: "test", ID: ""},
			fieldName:     "Name",
			expectedValue: "test",
			wantID:        "",
		},
		{
			name: "pointer field match",
			konnectRes: func() any {
				slug := "ptr-slug"
				id := "ptr-id"
				return struct {
					Slug *string
					ID   *string
				}{Slug: &slug, ID: &id}
			}(),
			fieldName:     "Slug",
			expectedValue: "ptr-slug",
			wantID:        "ptr-id",
		},
		{
			name:          "non-struct input returns empty",
			konnectRes:    "not-a-struct",
			fieldName:     "Name",
			expectedValue: "test",
			wantID:        "",
		},
		{
			name: "field does not exist returns empty",
			konnectRes: struct {
				ID string
			}{ID: "id-3"},
			fieldName:     "NonExistent",
			expectedValue: "value",
			wantID:        "",
		},
		{
			name: "pointer input is dereferenced",
			konnectRes: &struct {
				Name string
				ID   string
			}{Name: "ptr-name", ID: "ptr-id"},
			fieldName:     "Name",
			expectedValue: "ptr-name",
			wantID:        "ptr-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tryMatchByField(tt.konnectRes, tt.fieldName, tt.expectedValue)
			assert.Equal(t, tt.wantID, got)
		})
	}
}

func TestTryMatchByNameWithExternal(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		konnectRes   any
		opts         matchOptions
		external     *ExternalBlock
		wantID       string
		wantMatch    bool
	}{
		{
			name:         "match by name",
			resourceName: "my-api",
			konnectRes: struct {
				Name string
				ID   string
			}{Name: "my-api", ID: "id-1"},
			opts:      matchOptions{},
			external:  nil,
			wantID:    "id-1",
			wantMatch: true,
		},
		{
			name:         "no match when name differs",
			resourceName: "my-api",
			konnectRes: struct {
				Name string
				ID   string
			}{Name: "other-api", ID: "id-2"},
			opts:      matchOptions{},
			external:  nil,
			wantID:    "",
			wantMatch: false,
		},
		{
			name:         "no match when ID is empty",
			resourceName: "my-api",
			konnectRes: struct {
				Name string
				ID   string
			}{Name: "my-api", ID: ""},
			opts:      matchOptions{},
			external:  nil,
			wantID:    "",
			wantMatch: false,
		},
		{
			name:         "external match by ID",
			resourceName: "ignored",
			konnectRes: struct {
				Name string
				ID   string
			}{Name: "any-name", ID: "ext-id-1"},
			opts:      matchOptions{},
			external:  &ExternalBlock{ID: "ext-id-1"},
			wantID:    "ext-id-1",
			wantMatch: true,
		},
		{
			name:         "external ID mismatch",
			resourceName: "ignored",
			konnectRes: struct {
				Name string
				ID   string
			}{Name: "any-name", ID: "different-id"},
			opts:      matchOptions{},
			external:  &ExternalBlock{ID: "ext-id-1"},
			wantID:    "",
			wantMatch: false,
		},
		{
			name:         "external selector match",
			resourceName: "ignored",
			konnectRes: struct {
				Name string
				ID   string
			}{Name: "matched-name", ID: "sel-id"},
			opts: matchOptions{},
			external: &ExternalBlock{
				Selector: &ExternalSelector{
					MatchFields: map[string]string{"name": "matched-name"},
				},
			},
			wantID:    "sel-id",
			wantMatch: true,
		},
		{
			name:         "external selector no match",
			resourceName: "ignored",
			konnectRes: struct {
				Name string
				ID   string
			}{Name: "other-name", ID: "sel-id"},
			opts: matchOptions{},
			external: &ExternalBlock{
				Selector: &ExternalSelector{
					MatchFields: map[string]string{"name": "expected-name"},
				},
			},
			wantID:    "",
			wantMatch: false,
		},
		{
			name:         "match via embedded SDK type",
			resourceName: "my-api",
			konnectRes: struct {
				APIResponseSchema struct {
					Name string
					ID   string
				}
			}{APIResponseSchema: struct {
				Name string
				ID   string
			}{Name: "my-api", ID: "sdk-id"}},
			opts:      matchOptions{sdkType: "APIResponseSchema"},
			external:  nil,
			wantID:    "sdk-id",
			wantMatch: true,
		},
		{
			name:         "non-struct input returns no match",
			resourceName: "test",
			konnectRes:   "not-a-struct",
			opts:         matchOptions{},
			external:     nil,
			wantID:       "",
			wantMatch:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotMatch := tryMatchByNameWithExternal(
				tt.resourceName, tt.konnectRes, tt.opts, tt.external,
			)
			assert.Equal(t, tt.wantID, gotID)
			assert.Equal(t, tt.wantMatch, gotMatch)
		})
	}
}
