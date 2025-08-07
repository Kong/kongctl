package adapters

import (
	"testing"

	"github.com/kong/kongctl/internal/declarative/external"
	"github.com/stretchr/testify/assert"
)

func TestBaseAdapter_ValidateParentContext(t *testing.T) {
	base := &BaseAdapter{}

	tests := []struct {
		name         string
		parent       *external.ResolvedParent
		expectedType string
		wantErr      bool
		errMsg       string
	}{
		{
			name:         "nil parent",
			parent:       nil,
			expectedType: "portal",
			wantErr:      true,
			errMsg:       "parent context required",
		},
		{
			name: "wrong parent type",
			parent: &external.ResolvedParent{
				ResourceType: "api",
				ID:           "123",
			},
			expectedType: "portal",
			wantErr:      true,
			errMsg:       "invalid parent type",
		},
		{
			name: "missing parent ID",
			parent: &external.ResolvedParent{
				ResourceType: "portal",
				ID:           "",
			},
			expectedType: "portal",
			wantErr:      true,
			errMsg:       "parent ID is required",
		},
		{
			name: "valid parent",
			parent: &external.ResolvedParent{
				ResourceType: "portal",
				ID:           "portal-123",
			},
			expectedType: "portal",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := base.ValidateParentContext(tt.parent, tt.expectedType)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBaseAdapter_FilterBySelector(t *testing.T) {
	base := &BaseAdapter{}

	type testResource struct {
		Name        string
		Description string
	}

	getField := func(resource interface{}, field string) string {
		r := resource.(*testResource)
		switch field {
		case "name":
			return r.Name
		case "description":
			return r.Description
		default:
			return ""
		}
	}

	tests := []struct {
		name      string
		resources []interface{}
		selector  map[string]string
		wantErr   bool
		errMsg    string
		expected  *testResource
	}{
		{
			name:      "no matches",
			resources: []interface{}{
				&testResource{Name: "foo", Description: "bar"},
			},
			selector: map[string]string{"name": "baz"},
			wantErr:  true,
			errMsg:   "no resources found",
		},
		{
			name: "multiple matches",
			resources: []interface{}{
				&testResource{Name: "test", Description: "first"},
				&testResource{Name: "test", Description: "second"},
			},
			selector: map[string]string{"name": "test"},
			wantErr:  true,
			errMsg:   "selector matched 2 resources",
		},
		{
			name: "single match",
			resources: []interface{}{
				&testResource{Name: "foo", Description: "bar"},
				&testResource{Name: "test", Description: "match"},
			},
			selector: map[string]string{"name": "test", "description": "match"},
			wantErr:  false,
			expected: &testResource{Name: "test", Description: "match"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := base.FilterBySelector(tt.resources, tt.selector, getField)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}