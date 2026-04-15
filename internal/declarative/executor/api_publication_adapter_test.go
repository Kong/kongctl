package executor

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
)

type stubAPIPublicationAPI struct {
	t            *testing.T
	deleteAPIID  string
	deletePortal string
}

func (s *stubAPIPublicationAPI) PublishAPIToPortal(
	context.Context,
	kkOps.PublishAPIToPortalRequest,
	...kkOps.Option,
) (*kkOps.PublishAPIToPortalResponse, error) {
	s.t.Fatalf("unexpected PublishAPIToPortal call")
	return nil, nil
}

func (s *stubAPIPublicationAPI) DeletePublication(
	_ context.Context,
	apiID string,
	portalID string,
	_ ...kkOps.Option,
) (*kkOps.DeletePublicationResponse, error) {
	s.deleteAPIID = apiID
	s.deletePortal = portalID
	return &kkOps.DeletePublicationResponse{}, nil
}

func (s *stubAPIPublicationAPI) ListAPIPublications(
	context.Context,
	kkOps.ListAPIPublicationsRequest,
	...kkOps.Option,
) (*kkOps.ListAPIPublicationsResponse, error) {
	s.t.Fatalf("unexpected ListAPIPublications call")
	return nil, nil
}

func TestAPIPublicationAdapterDeleteUsesPortalIDField(t *testing.T) {
	t.Parallel()

	api := &stubAPIPublicationAPI{t: t}
	client := state.NewClient(state.ClientConfig{APIPublicationAPI: api})
	adapter := NewAPIPublicationAdapter(client)
	base := NewBaseCreateDeleteExecutor[kkComps.APIPublication](adapter, false)

	change := planner.PlannedChange{
		ID:           "1:d:api_publication:sms-to-getting-started-portal",
		ResourceType: "api_publication",
		ResourceRef:  "sms-to-getting-started-portal",
		ResourceID:   "api-123:portal-456",
		Action:       planner.ActionDelete,
		Fields: map[string]any{
			"api_id":    "api-123",
			"portal_id": "portal-456",
		},
		Parent: &planner.ParentInfo{Ref: "sms", ID: "api-123"},
	}

	if err := base.Delete(context.Background(), change); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if api.deleteAPIID != "api-123" {
		t.Fatalf("DeletePublication() apiID = %q, want %q", api.deleteAPIID, "api-123")
	}
	if api.deletePortal != "portal-456" {
		t.Fatalf("DeletePublication() portalID = %q, want %q", api.deletePortal, "portal-456")
	}
}

func TestAPIPublicationAdapterDeleteFallsBackToCompositeResourceID(t *testing.T) {
	t.Parallel()

	api := &stubAPIPublicationAPI{t: t}
	client := state.NewClient(state.ClientConfig{APIPublicationAPI: api})
	adapter := NewAPIPublicationAdapter(client)
	base := NewBaseCreateDeleteExecutor[kkComps.APIPublication](adapter, false)

	change := planner.PlannedChange{
		ID:           "1:d:api_publication:sms-to-getting-started-portal",
		ResourceType: "api_publication",
		ResourceRef:  "sms-to-getting-started-portal",
		ResourceID:   "api-123:portal-456",
		Action:       planner.ActionDelete,
		Parent:       &planner.ParentInfo{Ref: "sms", ID: "api-123"},
	}

	if err := base.Delete(context.Background(), change); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if api.deleteAPIID != "api-123" {
		t.Fatalf("DeletePublication() apiID = %q, want %q", api.deleteAPIID, "api-123")
	}
	if api.deletePortal != "portal-456" {
		t.Fatalf("DeletePublication() portalID = %q, want %q", api.deletePortal, "portal-456")
	}
}
