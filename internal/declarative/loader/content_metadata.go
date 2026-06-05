package loader

import (
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/frontmatter"
	"github.com/kong/kongctl/internal/declarative/resources"
)

func (l *Loader) normalizeContentMetadata(rs *resources.ResourceSet) error {
	for i := range rs.PortalPages {
		if err := normalizePortalPageContentMetadata(&rs.PortalPages[i]); err != nil {
			return err
		}
	}

	for i := range rs.PortalSnippets {
		if err := normalizePortalSnippetContentMetadata(&rs.PortalSnippets[i]); err != nil {
			return err
		}
	}

	for i := range rs.APIDocuments {
		if err := normalizeAPIDocumentContentMetadata(&rs.APIDocuments[i]); err != nil {
			return err
		}
	}

	for i := range rs.APIs {
		for j := range rs.APIs[i].Documents {
			if err := normalizeAPIDocumentContentMetadata(&rs.APIs[i].Documents[j]); err != nil {
				return err
			}
		}
	}

	return nil
}

func normalizePortalPageContentMetadata(page *resources.PortalPageResource) error {
	metadata, err := frontmatter.Parse(page.Content, frontmatter.PortalPageFields)
	if err != nil {
		return metadataParseError(resources.ResourceTypePortalPage, page.Ref, err)
	}

	if value, ok := metadata[frontmatter.FieldTitle]; ok {
		if err := deriveStringPtrMetadata(
			resources.ResourceTypePortalPage, page.Ref, frontmatter.FieldTitle, &page.Title, value,
		); err != nil {
			return err
		}
	}

	if value, ok := metadata[frontmatter.FieldDescription]; ok {
		if err := deriveStringPtrMetadata(
			resources.ResourceTypePortalPage, page.Ref, frontmatter.FieldDescription, &page.Description, value,
		); err != nil {
			return err
		}
	}

	return nil
}

func normalizePortalSnippetContentMetadata(snippet *resources.PortalSnippetResource) error {
	metadata, err := frontmatter.Parse(snippet.Content, frontmatter.PortalSnippetFields)
	if err != nil {
		return metadataParseError(resources.ResourceTypePortalSnippet, snippet.Ref, err)
	}

	if value, ok := metadata[frontmatter.FieldTitle]; ok {
		if err := deriveStringPtrMetadata(
			resources.ResourceTypePortalSnippet, snippet.Ref, frontmatter.FieldTitle, &snippet.Title, value,
		); err != nil {
			return err
		}
	}

	if value, ok := metadata[frontmatter.FieldDescription]; ok {
		if err := deriveStringPtrMetadata(
			resources.ResourceTypePortalSnippet, snippet.Ref, frontmatter.FieldDescription, &snippet.Description, value,
		); err != nil {
			return err
		}
	}

	return nil
}

func normalizeAPIDocumentContentMetadata(document *resources.APIDocumentResource) error {
	metadata, err := frontmatter.Parse(document.Content, frontmatter.APIDocumentFields)
	if err != nil {
		return metadataParseError(resources.ResourceTypeAPIDocument, document.Ref, err)
	}

	if value, ok := metadata[frontmatter.FieldTitle]; ok {
		if err := deriveStringPtrMetadata(
			resources.ResourceTypeAPIDocument, document.Ref, frontmatter.FieldTitle, &document.Title, value,
		); err != nil {
			return err
		}
	}

	if value, ok := metadata[frontmatter.FieldSlug]; ok {
		if err := deriveStringPtrMetadata(
			resources.ResourceTypeAPIDocument, document.Ref, frontmatter.FieldSlug, &document.Slug, value,
		); err != nil {
			return err
		}
	}

	if value, ok := metadata[frontmatter.FieldStatus]; ok {
		if document.Status == nil || string(*document.Status) == "" {
			status := kkComps.APIDocumentStatus(value)
			document.Status = &status
		} else if string(*document.Status) != value {
			return metadataConflictError(
				resources.ResourceTypeAPIDocument, document.Ref, frontmatter.FieldStatus, string(*document.Status), value,
			)
		}
	}

	return nil
}

func deriveStringPtrMetadata(
	resourceType resources.ResourceType,
	ref string,
	field string,
	target **string,
	value string,
) error {
	if *target == nil || **target == "" {
		derived := value
		*target = &derived
		return nil
	}
	if **target != value {
		return metadataConflictError(resourceType, ref, field, **target, value)
	}
	return nil
}

func metadataParseError(resourceType resources.ResourceType, ref string, err error) error {
	return fmt.Errorf("configuration error for %s %q: failed to parse content frontmatter: %w", resourceType, ref, err)
}

func metadataConflictError(
	resourceType resources.ResourceType,
	ref string,
	field string,
	topLevel string,
	front string,
) error {
	return fmt.Errorf(
		"configuration error for %s %q: field %q is defined in both the top-level resource and content "+
			"frontmatter with different values (top-level: %q, frontmatter: %q)",
		resourceType, ref, field, topLevel, front,
	)
}
