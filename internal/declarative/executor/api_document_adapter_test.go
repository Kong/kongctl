package executor

import (
	"context"
	"testing"

	"github.com/Kong/sdk-konnect-go/models/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression (#1947): title is optional for API documents everywhere
// upstream — the SDK's CreateAPIDocumentRequest.Title is a pointer, and
// explain/scaffold/resource validation treat it as optional — so a plan
// created without title must map cleanly instead of failing apply with
// "title is required".
func TestAPIDocumentAdapter_CreateWithoutTitle(t *testing.T) {
	adapter := NewAPIDocumentAdapter(nil)

	fields := map[string]any{
		"content": "document body",
		"slug":    "my-resource",
		"ref":     "my-resource",
	}

	var create components.CreateAPIDocumentRequest
	require.NoError(t, adapter.MapCreateFields(context.Background(), nil, fields, &create))

	assert.Equal(t, "document body", create.Content)
	require.NotNil(t, create.Slug)
	assert.Equal(t, "my-resource", *create.Slug)
	// title stays unset: the API treats it as optional.
	assert.Nil(t, create.Title)
}

func TestAPIDocumentAdapter_CreateWithTitle(t *testing.T) {
	adapter := NewAPIDocumentAdapter(nil)

	fields := map[string]any{
		"title":   "My document",
		"content": "document body",
	}

	var create components.CreateAPIDocumentRequest
	require.NoError(t, adapter.MapCreateFields(context.Background(), nil, fields, &create))

	require.NotNil(t, create.Title)
	assert.Equal(t, "My document", *create.Title)
	assert.Equal(t, "document body", create.Content)
}

func TestAPIDocumentAdapter_CreateStillRequiresContent(t *testing.T) {
	adapter := NewAPIDocumentAdapter(nil)

	var create components.CreateAPIDocumentRequest
	err := adapter.MapCreateFields(context.Background(), nil, map[string]any{
		"title": "My document",
	}, &create)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "content is required")

	assert.Equal(t, []string{"content"}, adapter.RequiredFields())
}
