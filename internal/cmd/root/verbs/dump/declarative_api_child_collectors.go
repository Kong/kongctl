package dump

import (
	"context"

	declresources "github.com/kong/kongctl/internal/declarative/resources"
)

type apiChildCollector = childCollector[declresources.APIResource]

var apiChildCollectors = buildAPIChildCollectors()

func buildAPIChildCollectors() []apiChildCollector {
	collectors := []apiChildCollector{
		childCollection(
			"failed to load API versions",
			func(ctx context.Context, d *childDumpContext) ([]declresources.APIVersionResource, error) {
				return buildAPIVersions(ctx, d.logger, d.client, d.parentID, d.parentName)
			},
			func(api *declresources.APIResource) *[]declresources.APIVersionResource { return &api.Versions },
		),
		childCollection(
			"failed to load API documents",
			func(ctx context.Context, d *childDumpContext) ([]declresources.APIDocumentResource, error) {
				return buildAPIDocuments(ctx, d.logger, d.client, d.parentID, d.parentName)
			},
			func(api *declresources.APIResource) *[]declresources.APIDocumentResource { return &api.Documents },
		),
		childCollection(
			"failed to load API publications",
			func(ctx context.Context, d *childDumpContext) ([]declresources.APIPublicationResource, error) {
				return buildAPIPublications(ctx, d.client, d.parentID)
			},
			func(api *declresources.APIResource) *[]declresources.APIPublicationResource { return &api.Publications },
		),
		childCollection(
			"failed to load API implementations",
			func(ctx context.Context, d *childDumpContext) ([]declresources.APIImplementationResource, error) {
				return buildAPIImplementations(ctx, d.logger, d.client, d.parentID, d.parentName)
			},
			func(api *declresources.APIResource) *[]declresources.APIImplementationResource {
				return &api.Implementations
			},
		),
	}
	if err := validateChildCollectors("API child dump", collectors); err != nil {
		panic(err)
	}
	return collectors
}
