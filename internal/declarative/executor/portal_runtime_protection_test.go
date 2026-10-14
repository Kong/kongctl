package executor

import (
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestExecutor_registeredPortalRejectsProtectedWrites(t *testing.T) {
	for _, action := range []planner.ActionType{planner.ActionUpdate, planner.ActionDelete} {
		t.Run(string(action), func(t *testing.T) {
			api := new(MockPortalAPI)
			api.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
				ListPortalsResponse: &kkComps.ListPortalsResponse{
					Data: []kkComps.ListPortalsResponsePortal{{
						ID: "portal-123", Name: "protected-portal",
						Labels: map[string]string{labels.NamespaceKey: "default", labels.ProtectedKey: labels.TrueValue},
					}},
					Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 1}},
				},
			}, nil)
			exec := New(state.NewClient(state.ClientConfig{PortalAPI: api}), nil, false)
			change := planner.PlannedChange{
				ResourceType: planner.ResourceTypePortal, ResourceID: "portal-123", Action: action,
				Namespace: "default", Fields: map[string]any{planner.FieldName: "protected-portal"},
			}
			var err error
			if action == planner.ActionUpdate {
				_, err = exec.updateResource(testContextWithLogger(), &change)
			} else {
				err = exec.deleteResource(testContextWithLogger(), &change)
			}
			require.ErrorContains(t, err, "is protected")
			api.AssertNotCalled(t, "UpdatePortal", mock.Anything, mock.Anything, mock.Anything)
			api.AssertNotCalled(t, "DeletePortal", mock.Anything, mock.Anything, mock.Anything)
			api.AssertExpectations(t)
		})
	}
}

// Explicit protection changes must remain executable through registered dispatch.
func TestExecutor_updateResourcePortalUnprotects(t *testing.T) {
	for _, tc := range []struct {
		name       string
		protection any
	}{
		{"in-memory", planner.ProtectionChange{Old: true, New: false}},
		{"saved-plan", map[string]any{"old": true, "new": false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := new(MockPortalAPI)
			api.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
				ListPortalsResponse: &kkComps.ListPortalsResponse{
					Data: []kkComps.ListPortalsResponsePortal{{
						ID: "portal-123", Name: "protected-portal",
						Labels: map[string]string{labels.NamespaceKey: "default", labels.ProtectedKey: labels.TrueValue},
					}},
					Meta: kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 1}},
				},
			}, nil)
			api.On("UpdatePortal", mock.Anything, "portal-123", mock.MatchedBy(func(request kkComps.UpdatePortal) bool {
				protected, present := request.Labels[labels.ProtectedKey]
				return present && protected == nil && request.Labels[labels.NamespaceKey] != nil &&
					*request.Labels[labels.NamespaceKey] == "default"
			})).Return(&kkOps.UpdatePortalResponse{PortalResponse: &kkComps.PortalResponse{ID: "portal-123"}}, nil).Once()
			exec := New(state.NewClient(state.ClientConfig{PortalAPI: api}), nil, false)
			change := planner.PlannedChange{
				ResourceType: planner.ResourceTypePortal, ResourceID: "portal-123", Action: planner.ActionUpdate,
				Namespace: "default", Fields: map[string]any{planner.FieldName: "protected-portal"}, Protection: tc.protection,
			}
			id, err := exec.updateResource(testContextWithLogger(), &change)
			assert.NoError(t, err)
			assert.Equal(t, "portal-123", id)
			api.AssertExpectations(t)
		})
	}
}
