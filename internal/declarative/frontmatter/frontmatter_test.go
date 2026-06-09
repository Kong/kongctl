package frontmatter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRecognizedScalarFields(t *testing.T) {
	t.Parallel()

	content := "---\ntitle: Frontmatter Title\ndescription: Frontmatter Description\nimage: hero.png\n---\n# Body\n"

	metadata, err := Parse(content, PortalPageFields)
	require.NoError(t, err)

	assert.Equal(t, Metadata{
		FieldTitle:       "Frontmatter Title",
		FieldDescription: "Frontmatter Description",
	}, metadata)
}

func TestParseSupportsCRLF(t *testing.T) {
	t.Parallel()

	content := "---\r\ntitle: Frontmatter Title\r\n---\r\n# Body\r\n"

	metadata, err := Parse(content, PortalPageFields)
	require.NoError(t, err)

	assert.Equal(t, "Frontmatter Title", metadata[FieldTitle])
}

func TestParseIgnoresContentWithoutOpeningDelimiter(t *testing.T) {
	t.Parallel()

	content := "title: Not Frontmatter\n---\n# Body\n"

	metadata, err := Parse(content, PortalPageFields)
	require.NoError(t, err)

	assert.Empty(t, metadata)
}

func TestParseIgnoresUnknownAndNonScalarFields(t *testing.T) {
	t.Parallel()

	content := `---
title: Frontmatter Title
description:
  text: nested
image: hero.png
tags:
  - one
---
# Body
`

	metadata, err := Parse(content, PortalPageFields)
	require.NoError(t, err)

	assert.Equal(t, Metadata{FieldTitle: "Frontmatter Title"}, metadata)
}

func TestParseRejectsInvalidFrontmatter(t *testing.T) {
	t.Parallel()

	_, err := Parse("---\ntitle: [broken\n---\n# Body\n", PortalPageFields)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid YAML frontmatter")
}

func TestParseRejectsUnclosedFrontmatter(t *testing.T) {
	t.Parallel()

	_, err := Parse("---\ntitle: Frontmatter Title\n# Body\n", PortalPageFields)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unclosed YAML frontmatter block")
}

func TestParseIgnoresNonMapFrontmatter(t *testing.T) {
	t.Parallel()

	metadata, err := Parse("---\n- title\n- description\n---\n# Body\n", PortalPageFields)
	require.NoError(t, err)

	assert.Empty(t, metadata)
}
