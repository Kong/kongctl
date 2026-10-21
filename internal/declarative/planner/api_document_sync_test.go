package planner

import (
	"context"
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/require"
)

func TestPlanAPIDocumentsSyncMixedDeclarations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name               string
		inline, standalone bool
		update             string
		deletes            []string
	}{
		{name: "converged", inline: true, standalone: true},
		{name: "update inline", inline: true, standalone: true, update: "inline"},
		{name: "update standalone", inline: true, standalone: true, update: "standalone"},
		{name: "remove standalone", inline: true, deletes: []string{"standalone-id"}},
		{name: "remove inline", standalone: true, deletes: []string{"inline-id", "child-id"}},
		{name: "remove all", deletes: []string{"inline-id", "child-id", "standalone-id"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := resources.APIResource{BaseResource: resources.BaseResource{
				Ref: "api", Kongctl: &resources.KongctlMeta{Namespace: new("test")},
			}}
			api.SetKonnectID("api-id")
			document := func(ref string) resources.APIDocumentResource {
				content := "content"
				if ref == tc.update {
					content = "updated"
				}
				return resources.APIDocumentResource{
					Ref: ref, API: "api",
					CreateAPIDocumentRequest: kkComps.CreateAPIDocumentRequest{
						Title: new(ref), Slug: new(ref), Content: content,
					},
				}
			}
			if tc.inline {
				child := document("child")
				child.ParentDocumentRef = "inline"
				api.Documents = []resources.APIDocumentResource{document("inline"), child}
			}
			rs := &resources.ResourceSet{APIs: []resources.APIResource{api}}
			if tc.standalone {
				rs.APIDocuments = []resources.APIDocumentResource{document("standalone")}
			}
			rs.EnsureSyncScope().AddChild(resources.ResourceTypeAPI, "api", resources.ResourceTypeAPIDocument)
			docs := []kkComps.APIDocumentSummaryWithChildren{
				{ID: "inline-id", Title: "inline", Slug: "inline"},
				{ID: "child-id", Title: "child", Slug: "child", ParentDocumentID: new("inline-id")},
				{ID: "standalone-id", Title: "standalone", Slug: "standalone"},
			}
			client := state.NewClient(state.ClientConfig{APIDocumentAPI: &syncDocumentAPI{
				stubAPIDocumentAPI: stubAPIDocumentAPI{response: &kkOps.ListAPIDocumentsResponse{
					ListAPIDocumentResponse: &kkComps.ListAPIDocumentResponse{Data: docs},
				}},
			}})
			p := NewPlanner(client, discardPlannerLogger())
			p.resources = rs
			plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
			config := &Config{Namespace: "test"}
			current := state.API{APIResponseSchema: kkComps.APIResponseSchema{ID: "api-id"}}
			require.NoError(t, p.planAPIChildResourceChanges(t.Context(), config, current, api, plan))
			require.NoError(t, p.planAPIDocumentsChanges(t.Context(), config, rs.APIDocuments, plan))
			deleted := []string{}
			updated := []string{}
			for _, change := range plan.Changes {
				require.Equal(t, ResourceTypeAPIDocument, change.ResourceType)
				require.Equal(t, "api-id", change.Parent.ID)
				require.Equal(t, "test", change.Namespace)
				if change.Action == ActionDelete {
					deleted = append(deleted, change.ResourceID)
				} else {
					require.Equal(t, ActionUpdate, change.Action)
					updated = append(updated, change.ResourceID)
				}
			}
			require.ElementsMatch(t, tc.deletes, deleted)
			if tc.update != "" {
				require.Equal(t, []string{tc.update + "-id"}, updated)
			} else {
				require.Empty(t, updated)
			}
		})
	}
}

type syncDocumentAPI struct{ stubAPIDocumentAPI }

func (s *syncDocumentAPI) FetchAPIDocument(
	_ context.Context, _ string, id string, _ ...kkOps.Option,
) (*kkOps.FetchAPIDocumentResponse, error) {
	for _, doc := range s.response.ListAPIDocumentResponse.Data {
		if doc.ID == id {
			return &kkOps.FetchAPIDocumentResponse{APIDocumentResponse: &kkComps.APIDocumentResponse{
				ID: id, Title: doc.Title, Slug: doc.Slug, Content: "content", ParentDocumentID: doc.ParentDocumentID,
			}}, nil
		}
	}
	return nil, fmt.Errorf("document %s not found", id)
}
