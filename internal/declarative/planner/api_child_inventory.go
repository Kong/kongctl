package planner

import (
	"context"
	"fmt"

	"github.com/kong/kongctl/internal/declarative/resources"
)

// apiChildPlanningInventory is deliberately ordered: planning uses versions,
// publications, implementations, documents, unlike the API export inventory.
func (p *Planner) apiChildPlanningInventory() []apiChildPlanningStep {
	return []apiChildPlanningStep{
		apiChildPlanningAdapter(p, resources.ResourceTypeAPIVersion, "version",
			func(api resources.APIResource) []resources.APIVersionResource { return api.Versions },
			p.getAPIVersionsForAPI,
			(*BasePlanner).GetDesiredAPIVersions,
			p.planAPIVersionCreate,
			p.planAPIVersionChanges, p.planAPIVersionsChanges),
		apiChildPlanningAdapter(p, resources.ResourceTypeAPIPublication, "publication",
			func(api resources.APIResource) []resources.APIPublicationResource { return api.Publications },
			p.getAPIPublicationsForAPI,
			(*BasePlanner).GetDesiredAPIPublications,
			p.planAPIPublicationCreate,
			p.planAPIPublicationChanges, p.planAPIPublicationsChanges),
		apiChildPlanningAdapter(p, resources.ResourceTypeAPIImplementation, "implementation",
			func(api resources.APIResource) []resources.APIImplementationResource { return api.Implementations },
			p.getAPIImplementationsForAPI,
			(*BasePlanner).GetDesiredAPIImplementations,
			p.planAPIImplementationCreate,
			p.planAPIImplementationChanges, p.planAPIImplementationsChanges),
		apiChildPlanningAdapter(p, resources.ResourceTypeAPIDocument, "document",
			func(api resources.APIResource) []resources.APIDocumentResource { return api.Documents },
			p.getAPIDocumentsForAPI,
			(*BasePlanner).GetDesiredAPIDocuments,
			func(namespace, apiRef, apiID string, child resources.APIDocumentResource, deps []string, plan *Plan) {
				// Root creation has no per-group lookup; extracted documents build their own.
				p.planAPIDocumentCreate(namespace, apiRef, apiID, child, deps, apiDocumentLookup{}, plan)
			},
			p.planAPIDocumentChanges, p.planAPIDocumentsChanges),
	}
}

// Each typed adapter binds selection and lifecycle methods once. Traversal owns
// ordering and scope; the existing lifecycle methods still own identity/pruning.
type apiChildPlanningStep struct {
	create    func(string, resources.APIResource, string, *Plan)
	reconcile func(context.Context, *Config, string, string, resources.APIResource, *Plan) error
	extracted func(context.Context, *Config, *BasePlanner, string, *Plan) error
}

func apiChildPlanningAdapter[D interface{ GetRef() string }](
	p *Planner,
	kind resources.ResourceType,
	label string,
	nested func(resources.APIResource) []D,
	forNewParent func(resources.APIResource) []D,
	selectExtracted func(*BasePlanner, string) []D,
	create func(string, string, string, D, []string, *Plan),
	reconcile func(context.Context, *Config, string, string, string, []D, *Plan) error,
	extracted func(context.Context, *Config, []D, *Plan) error,
) apiChildPlanningStep {
	return apiChildPlanningStep{
		create: func(namespace string, api resources.APIResource, changeID string, plan *Plan) {
			for _, child := range forNewParent(api) {
				if !plan.HasChange(string(kind), child.GetRef()) {
					create(namespace, api.GetRef(), "", child, []string{changeID}, plan)
				}
			}
		},
		reconcile: func(
			ctx context.Context, cfg *Config, namespace, apiID string, api resources.APIResource, plan *Plan,
		) error {
			if !p.shouldPlanChild(plan, resources.ResourceTypeAPI, api.GetRef(), kind) {
				return nil
			}
			if err := reconcile(ctx, cfg, namespace, apiID, api.GetRef(), nested(api), plan); err != nil {
				return fmt.Errorf("failed to plan API %s changes: %w", label, err)
			}
			return nil
		},
		extracted: func(ctx context.Context, cfg *Config, base *BasePlanner, namespace string, plan *Plan) error {
			return extracted(ctx, cfg, selectExtracted(base, namespace), plan)
		},
	}
}
