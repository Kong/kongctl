package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoaderDerivesContentMetadataFromFrontmatter(t *testing.T) {
	t.Parallel()

	rs := loadContentMetadataConfig(t, `
portals:
  - ref: portal-1
    name: Portal 1
    pages:
      - ref: page-1
        slug: getting-started
        content: |
          ---
          title: Getting Started
          description: Page description
          ---

          # Getting Started
    snippets:
      - ref: snippet-1
        name: getting-started-banner
        content: |
          ---
          title: Getting Started Banner
          description: Snippet description
          ---

          <div>Welcome</div>
apis:
  - ref: api-1
    name: API 1
    documents:
      - ref: doc-1
        content: |
          ---
          title: API Guide
          slug: api-guide
          status: unpublished
          ---

          # API Guide
`)

	require.Len(t, rs.PortalPages, 1)
	require.NotNil(t, rs.PortalPages[0].Title)
	assert.Equal(t, "Getting Started", *rs.PortalPages[0].Title)
	require.NotNil(t, rs.PortalPages[0].Description)
	assert.Equal(t, "Page description", *rs.PortalPages[0].Description)

	require.Len(t, rs.PortalSnippets, 1)
	require.NotNil(t, rs.PortalSnippets[0].Title)
	assert.Equal(t, "Getting Started Banner", *rs.PortalSnippets[0].Title)
	require.NotNil(t, rs.PortalSnippets[0].Description)
	assert.Equal(t, "Snippet description", *rs.PortalSnippets[0].Description)

	require.Len(t, rs.APIs, 1)
	require.Len(t, rs.APIs[0].Documents, 1)
	doc := rs.APIs[0].Documents[0]
	require.NotNil(t, doc.Title)
	assert.Equal(t, "API Guide", *doc.Title)
	require.NotNil(t, doc.Slug)
	assert.Equal(t, "api-guide", *doc.Slug)
	require.NotNil(t, doc.Status)
	assert.Equal(t, "unpublished", string(*doc.Status))
}

func TestLoaderAllowsMatchingContentMetadata(t *testing.T) {
	t.Parallel()

	rs := loadContentMetadataConfig(t, `
portals:
  - ref: portal-1
    name: Portal 1
    pages:
      - ref: page-1
        slug: getting-started
        title: Getting Started
        description: Page description
        content: |
          ---
          title: Getting Started
          description: Page description
          ---

          # Getting Started
`)

	require.Len(t, rs.PortalPages, 1)
	require.NotNil(t, rs.PortalPages[0].Title)
	assert.Equal(t, "Getting Started", *rs.PortalPages[0].Title)
	require.NotNil(t, rs.PortalPages[0].Description)
	assert.Equal(t, "Page description", *rs.PortalPages[0].Description)
}

func TestLoaderDefaultsAPIDocumentSlugAfterFrontmatterTitleDerivation(t *testing.T) {
	t.Parallel()

	rs := loadContentMetadataConfig(t, `
apis:
  - ref: api-1
    name: API 1
    documents:
      - ref: doc-1
        content: |
          ---
          title: API Guide
          ---

          # API Guide
`)

	require.Len(t, rs.APIs, 1)
	require.Len(t, rs.APIs[0].Documents, 1)
	doc := rs.APIs[0].Documents[0]
	require.NotNil(t, doc.Title)
	assert.Equal(t, "API Guide", *doc.Title)
	require.NotNil(t, doc.Slug)
	assert.Equal(t, "api-guide", *doc.Slug)
}

func TestLoaderDoesNotDerivePortalPageSlugFromFrontmatterTitle(t *testing.T) {
	t.Parallel()

	_, err := loadContentMetadataConfigErr(t, `
portals:
  - ref: portal-1
    name: Portal 1
    pages:
      - ref: page-1
        content: |
          ---
          title: Getting Started
          ---

          # Getting Started
`)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "page slug is required")
}

func TestLoaderDoesNotDerivePortalSnippetNameFromFrontmatterTitle(t *testing.T) {
	t.Parallel()

	_, err := loadContentMetadataConfigErr(t, `
portals:
  - ref: portal-1
    name: Portal 1
    snippets:
      - ref: snippet-1
        content: |
          ---
          title: Getting Started
          ---

          body
`)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "snippet name is required")
}

func TestLoaderRejectsConflictingContentMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		resourceType string
		ref          string
		field        string
		topLevel     string
		frontmatter  string
		config       string
	}{
		{
			name:         "portal page title",
			resourceType: "portal_page",
			ref:          "page-1",
			field:        "title",
			topLevel:     "Top Page",
			frontmatter:  "Front Page",
			config: `
portals:
  - ref: portal-1
    name: Portal 1
    pages:
      - ref: page-1
        slug: page
        title: Top Page
        content: |
          ---
          title: Front Page
          ---

          # Body
`,
		},
		{
			name:         "portal page description",
			resourceType: "portal_page",
			ref:          "page-1",
			field:        "description",
			topLevel:     "Top description",
			frontmatter:  "Front description",
			config: `
portals:
  - ref: portal-1
    name: Portal 1
    pages:
      - ref: page-1
        slug: page
        description: Top description
        content: |
          ---
          description: Front description
          ---

          # Body
`,
		},
		{
			name:         "portal snippet title",
			resourceType: "portal_snippet",
			ref:          "snippet-1",
			field:        "title",
			topLevel:     "Top Snippet",
			frontmatter:  "Front Snippet",
			config: `
portals:
  - ref: portal-1
    name: Portal 1
    snippets:
      - ref: snippet-1
        name: snippet
        title: Top Snippet
        content: |
          ---
          title: Front Snippet
          ---

          body
`,
		},
		{
			name:         "portal snippet description",
			resourceType: "portal_snippet",
			ref:          "snippet-1",
			field:        "description",
			topLevel:     "Top description",
			frontmatter:  "Front description",
			config: `
portals:
  - ref: portal-1
    name: Portal 1
    snippets:
      - ref: snippet-1
        name: snippet
        description: Top description
        content: |
          ---
          description: Front description
          ---

          body
`,
		},
		{
			name:         "api document title",
			resourceType: "api_document",
			ref:          "doc-1",
			field:        "title",
			topLevel:     "Top Doc",
			frontmatter:  "Front Doc",
			config: `
apis:
  - ref: api-1
    name: API 1
    documents:
      - ref: doc-1
        title: Top Doc
        content: |
          ---
          title: Front Doc
          ---

          # Body
`,
		},
		{
			name:         "api document slug",
			resourceType: "api_document",
			ref:          "doc-1",
			field:        "slug",
			topLevel:     "top-doc",
			frontmatter:  "front-doc",
			config: `
apis:
  - ref: api-1
    name: API 1
    documents:
      - ref: doc-1
        title: Doc
        slug: top-doc
        content: |
          ---
          slug: front-doc
          ---

          # Body
`,
		},
		{
			name:         "api document status",
			resourceType: "api_document",
			ref:          "doc-1",
			field:        "status",
			topLevel:     "published",
			frontmatter:  "unpublished",
			config: `
apis:
  - ref: api-1
    name: API 1
    documents:
      - ref: doc-1
        title: Doc
        status: published
        content: |
          ---
          status: unpublished
          ---

          # Body
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := loadContentMetadataConfigErr(t, tt.config)
			require.Error(t, err)
			assert.Contains(t, err.Error(), fmt.Sprintf(`configuration error for %s %q`, tt.resourceType, tt.ref))
			assert.Contains(t, err.Error(), fmt.Sprintf(`field %q`, tt.field))
			assert.Contains(t, err.Error(), fmt.Sprintf(`top-level: %q`, tt.topLevel))
			assert.Contains(t, err.Error(), fmt.Sprintf(`frontmatter: %q`, tt.frontmatter))
		})
	}
}

func TestLoaderIgnoresUnknownContentMetadata(t *testing.T) {
	t.Parallel()

	rs := loadContentMetadataConfig(t, `
portals:
  - ref: portal-1
    name: Portal 1
    pages:
      - ref: page-1
        slug: getting-started
        content: |
          ---
          image: hero.png
          page-layout: landing
          ---

          # Getting Started
`)

	require.Len(t, rs.PortalPages, 1)
	require.NotNil(t, rs.PortalPages[0].Title)
	assert.Equal(t, "getting-started", *rs.PortalPages[0].Title)
	assert.Contains(t, rs.PortalPages[0].Content, "image: hero.png")
}

func TestLoaderRejectsInvalidContentFrontmatter(t *testing.T) {
	t.Parallel()

	_, err := loadContentMetadataConfigErr(t, `
portals:
  - ref: portal-1
    name: Portal 1
    pages:
      - ref: page-1
        slug: page
        content: |
          ---
          title: [broken
          ---

          # Body
`)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `configuration error for portal_page "page-1"`)
	assert.Contains(t, err.Error(), "invalid YAML frontmatter")
}

func TestLoaderRejectsUnclosedContentFrontmatter(t *testing.T) {
	t.Parallel()

	_, err := loadContentMetadataConfigErr(t, `
portals:
  - ref: portal-1
    name: Portal 1
    pages:
      - ref: page-1
        slug: page
        content: |
          ---
          title: Frontmatter Title

          # Body
`)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `configuration error for portal_page "page-1"`)
	assert.Contains(t, err.Error(), "unclosed YAML frontmatter block")
}

func TestLoaderIgnoresNonMapContentFrontmatter(t *testing.T) {
	t.Parallel()

	rs := loadContentMetadataConfig(t, `
portals:
  - ref: portal-1
    name: Portal 1
    pages:
      - ref: page-1
        slug: page
        content: |
          ---
          - title
          - description
          ---

          # Body
`)

	require.Len(t, rs.PortalPages, 1)
	require.NotNil(t, rs.PortalPages[0].Title)
	assert.Equal(t, "page", *rs.PortalPages[0].Title)
}

func loadContentMetadataConfig(t *testing.T, content string) *resources.ResourceSet {
	t.Helper()

	rs, err := loadContentMetadataConfigErr(t, content)
	require.NoError(t, err)
	return rs
}

func loadContentMetadataConfigErr(t *testing.T, content string) (*resources.ResourceSet, error) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return New().LoadFromSources([]Source{{Path: path, Type: SourceTypeFile}}, false)
}
