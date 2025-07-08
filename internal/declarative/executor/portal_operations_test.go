package executor

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/log"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// testContextWithLogger returns a context with a test logger
func testContextWithLogger() context.Context {
	logger := slog.Default()
	return context.WithValue(context.Background(), log.LoggerKey, logger)
}

// MockPortalAPI for testing
type MockPortalAPI struct {
	mock.Mock
}

func (m *MockPortalAPI) ListPortals(
	ctx context.Context,
	request kkOps.ListPortalsRequest,
) (*kkOps.ListPortalsResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.ListPortalsResponse), args.Error(1)
}

func (m *MockPortalAPI) GetPortal(ctx context.Context, id string) (*kkOps.GetPortalResponse, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.GetPortalResponse), args.Error(1)
}

func (m *MockPortalAPI) CreatePortal(
	ctx context.Context,
	portal kkComps.CreatePortal,
) (*kkOps.CreatePortalResponse, error) {
	args := m.Called(ctx, portal)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.CreatePortalResponse), args.Error(1)
}

func (m *MockPortalAPI) UpdatePortal(
	ctx context.Context,
	id string,
	portal kkComps.UpdatePortal,
) (*kkOps.UpdatePortalResponse, error) {
	args := m.Called(ctx, id, portal)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.UpdatePortalResponse), args.Error(1)
}

func (m *MockPortalAPI) DeletePortal(
	ctx context.Context,
	id string,
	force bool,
) (*kkOps.DeletePortalResponse, error) {
	args := m.Called(ctx, id, force)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.DeletePortalResponse), args.Error(1)
}

func TestExecutor_createPortal(t *testing.T) {
	tests := []struct {
		name         string
		change       planner.PlannedChange
		setupMock    func(*MockPortalAPI)
		wantID       string
		wantErr      bool
		validateCall func(*testing.T, *MockPortalAPI)
	}{
		{
			name: "successful create with all fields",
			change: planner.PlannedChange{
				ResourceType: "portal",
				Action:       planner.ActionCreate,
				Fields: map[string]interface{}{
					"name":                     "test-portal",
					"description":              "Test description",
					"authentication_enabled":   true,
					"auto_approve_applications": true,
					"auto_approve_developers":  false,
					"rbac_enabled":             true,
					"labels": map[string]interface{}{
						"env":  "test",
						"team": "platform",
					},
				},
			},
			setupMock: func(m *MockPortalAPI) {
				m.On("CreatePortal", mock.Anything, mock.MatchedBy(func(p kkComps.CreatePortal) bool {
					// Verify fields
					if p.Name != "test-portal" {
						return false
					}
					if p.Description == nil || *p.Description != "Test description" {
						return false
					}
					if p.AuthenticationEnabled == nil || !*p.AuthenticationEnabled {
						return false
					}
					if p.AutoApproveApplications == nil || !*p.AutoApproveApplications {
						return false
					}
					if p.AutoApproveDevelopers == nil || *p.AutoApproveDevelopers {
						return false
					}
					if p.RbacEnabled == nil || !*p.RbacEnabled {
						return false
					}
					// Verify labels
					if p.Labels == nil {
						return false
					}
					// Should have user labels + management labels
					if p.Labels["env"] == nil || *p.Labels["env"] != "test" {
						return false
					}
					if p.Labels["team"] == nil || *p.Labels["team"] != "platform" {
						return false
					}
					if p.Labels[labels.ManagedKey] == nil || *p.Labels[labels.ManagedKey] != "true" {
						return false
					}
					return true
				})).Return(&kkOps.CreatePortalResponse{
					PortalResponse: &kkComps.PortalResponse{
						ID: "portal-123",
					},
				}, nil)
			},
			wantID:  "portal-123",
			wantErr: false,
		},
		{
			name: "successful create with minimal fields",
			change: planner.PlannedChange{
				ResourceType: "portal",
				Action:       planner.ActionCreate,
				Fields: map[string]interface{}{
					"name": "minimal-portal",
				},
			},
			setupMock: func(m *MockPortalAPI) {
				m.On("CreatePortal", mock.Anything, mock.MatchedBy(func(p kkComps.CreatePortal) bool {
					return p.Name == "minimal-portal" &&
						p.Labels[labels.ManagedKey] != nil &&
						*p.Labels[labels.ManagedKey] == "true"
				})).Return(&kkOps.CreatePortalResponse{
					PortalResponse: &kkComps.PortalResponse{
						ID: "portal-456",
					},
				}, nil)
			},
			wantID:  "portal-456",
			wantErr: false,
		},
		{
			name: "missing name field",
			change: planner.PlannedChange{
				ResourceType: "portal",
				Action:       planner.ActionCreate,
				Fields: map[string]interface{}{
					"description": "Missing name",
				},
			},
			setupMock: func(_ *MockPortalAPI) {},
			wantErr:   true,
		},
		{
			name: "API error",
			change: planner.PlannedChange{
				ResourceType: "portal",
				Action:       planner.ActionCreate,
				Fields: map[string]interface{}{
					"name": "error-portal",
				},
			},
			setupMock: func(m *MockPortalAPI) {
				m.On("CreatePortal", mock.Anything, mock.Anything).
					Return(nil, errors.New("API error"))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockAPI := new(MockPortalAPI)
			tt.setupMock(mockAPI)
			
			client := state.NewClient(state.ClientConfig{
				PortalAPI: mockAPI,
			})
			executor := New(client, nil, false)
			
			// Execute
			gotID, err := executor.createPortal(testContextWithLogger(), tt.change)
			
			// Verify
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantID, gotID)
			}
			
			mockAPI.AssertExpectations(t)
		})
	}
}

func TestExecutor_updatePortal(t *testing.T) {
	tests := []struct {
		name      string
		change    planner.PlannedChange
		setupMock func(*MockPortalAPI)
		wantID    string
		wantErr   bool
		errMsg    string
	}{
		{
			name: "successful update",
			change: planner.PlannedChange{
				ResourceType: "portal",
				ResourceID:   "portal-123",
				Action:       planner.ActionUpdate,
				Fields: map[string]interface{}{
					"name":        "updated-portal",
					"description": "Updated description",
				},
			},
			setupMock: func(m *MockPortalAPI) {
				// Mock GetPortalByName for protection check
				m.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
					ListPortalsResponse: &kkComps.ListPortalsResponse{
						Data: []kkComps.Portal{
							{
								ID:   "portal-123",
								Name: "updated-portal",
								Labels: map[string]string{
									labels.ManagedKey: "true",
									// No protected label
								},
							},
						},
						Meta: kkComps.PaginatedMeta{
							Page: kkComps.PageMeta{Total: 1},
						},
					},
				}, nil)
				
				// Mock UpdatePortal
				m.On("UpdatePortal", mock.Anything, "portal-123", mock.MatchedBy(func(p kkComps.UpdatePortal) bool {
					return p.Name != nil && *p.Name == "updated-portal" &&
						p.Description != nil && *p.Description == "Updated description"
				})).Return(&kkOps.UpdatePortalResponse{
					PortalResponse: &kkComps.PortalResponse{
						ID: "portal-123",
					},
				}, nil)
			},
			wantID:  "portal-123",
			wantErr: false,
		},
		{
			name: "update protected portal",
			change: planner.PlannedChange{
				ResourceType: "portal",
				ResourceID:   "portal-456",
				Action:       planner.ActionUpdate,
				Fields: map[string]interface{}{
					"name": "protected-portal",
				},
			},
			setupMock: func(m *MockPortalAPI) {
				m.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
					ListPortalsResponse: &kkComps.ListPortalsResponse{
						Data: []kkComps.Portal{
							{
								ID:   "portal-456",
								Name: "protected-portal",
								Labels: map[string]string{
									labels.ManagedKey:   "true",
									labels.ProtectedKey: "true",
								},
							},
						},
						Meta: kkComps.PaginatedMeta{
							Page: kkComps.PageMeta{Total: 1},
						},
					},
				}, nil)
			},
			wantErr: true,
			errMsg:  "resource is protected and cannot be updated",
		},
		{
			name: "portal not found",
			change: planner.PlannedChange{
				ResourceType: "portal",
				ResourceID:   "portal-789",
				Action:       planner.ActionUpdate,
				Fields: map[string]interface{}{
					"name": "missing-portal",
				},
			},
			setupMock: func(m *MockPortalAPI) {
				m.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
					ListPortalsResponse: &kkComps.ListPortalsResponse{
						Data: []kkComps.Portal{},
						Meta: kkComps.PaginatedMeta{
							Page: kkComps.PageMeta{Total: 0},
						},
					},
				}, nil)
			},
			wantErr: true,
			errMsg:  "portal no longer exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockAPI := new(MockPortalAPI)
			tt.setupMock(mockAPI)
			
			client := state.NewClient(state.ClientConfig{
				PortalAPI: mockAPI,
			})
			executor := New(client, nil, false)
			
			// Execute
			gotID, err := executor.updatePortal(testContextWithLogger(), tt.change)
			
			// Verify
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" && err != nil {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantID, gotID)
			}
			
			mockAPI.AssertExpectations(t)
		})
	}
}

func TestExecutor_deletePortal(t *testing.T) {
	tests := []struct {
		name      string
		change    planner.PlannedChange
		setupMock func(*MockPortalAPI)
		wantErr   bool
		errMsg    string
	}{
		{
			name: "successful delete",
			change: planner.PlannedChange{
				ResourceType: "portal",
				ResourceID:   "portal-123",
				Action:       planner.ActionDelete,
				Fields: map[string]interface{}{
					"name": "delete-portal",
				},
			},
			setupMock: func(m *MockPortalAPI) {
				// Mock GetPortalByName for protection check
				m.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
					ListPortalsResponse: &kkComps.ListPortalsResponse{
						Data: []kkComps.Portal{
							{
								ID:   "portal-123",
								Name: "delete-portal",
								Labels: map[string]string{
									labels.ManagedKey: "true",
								},
							},
						},
						Meta: kkComps.PaginatedMeta{
							Page: kkComps.PageMeta{Total: 1},
						},
					},
				}, nil)
				
				// Mock DeletePortal with force=true
				m.On("DeletePortal", mock.Anything, "portal-123", true).
					Return(&kkOps.DeletePortalResponse{}, nil)
			},
			wantErr: false,
		},
		{
			name: "delete protected portal",
			change: planner.PlannedChange{
				ResourceType: "portal",
				ResourceID:   "portal-456",
				Action:       planner.ActionDelete,
				Fields: map[string]interface{}{
					"name": "protected-portal",
				},
			},
			setupMock: func(m *MockPortalAPI) {
				m.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
					ListPortalsResponse: &kkComps.ListPortalsResponse{
						Data: []kkComps.Portal{
							{
								ID:   "portal-456",
								Name: "protected-portal",
								Labels: map[string]string{
									labels.ManagedKey:   "true",
									labels.ProtectedKey: "true",
								},
							},
						},
						Meta: kkComps.PaginatedMeta{
							Page: kkComps.PageMeta{Total: 1},
						},
					},
				}, nil)
			},
			wantErr: true,
			errMsg:  "resource is protected and cannot be deleted",
		},
		{
			name: "delete non-managed portal - not found by managed portal search",
			change: planner.PlannedChange{
				ResourceType: "portal",
				ResourceID:   "portal-789",
				Action:       planner.ActionDelete,
				Fields: map[string]interface{}{
					"name": "unmanaged-portal",
				},
			},
			setupMock: func(m *MockPortalAPI) {
				// ListPortals will be called but won't return this portal
				// because it's not managed (ListManagedPortals filters it out)
				m.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
					ListPortalsResponse: &kkComps.ListPortalsResponse{
						Data: []kkComps.Portal{}, // Empty - unmanaged portal filtered out
						Meta: kkComps.PaginatedMeta{
							Page: kkComps.PageMeta{Total: 0},
						},
					},
				}, nil)
				
				// Delete won't be called because portal is not found in managed portals
				// This is correct behavior - we only delete portals we manage
			},
			wantErr: false, // Success - portal not found means nothing to do
		},
		{
			name: "portal already deleted",
			change: planner.PlannedChange{
				ResourceType: "portal",
				ResourceID:   "portal-999",
				Action:       planner.ActionDelete,
				Fields: map[string]interface{}{
					"name": "already-deleted",
				},
			},
			setupMock: func(m *MockPortalAPI) {
				m.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
					ListPortalsResponse: &kkComps.ListPortalsResponse{
						Data: []kkComps.Portal{},
						Meta: kkComps.PaginatedMeta{
							Page: kkComps.PageMeta{Total: 0},
						},
					},
				}, nil)
			},
			wantErr: false, // Already deleted is considered success
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockAPI := new(MockPortalAPI)
			tt.setupMock(mockAPI)
			
			client := state.NewClient(state.ClientConfig{
				PortalAPI: mockAPI,
			})
			executor := New(client, nil, false)
			
			// Execute
			err := executor.deletePortal(testContextWithLogger(), tt.change)
			
			// Verify
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" && err != nil {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
			
			mockAPI.AssertExpectations(t)
		})
	}
}

func TestExecutor_protectionChangeBetweenPlanAndExecution(t *testing.T) {
	// This test verifies that protection status is checked at execution time,
	// not just during planning
	change := planner.PlannedChange{
		ResourceType: "portal",
		ResourceID:   "portal-123",
		Action:       planner.ActionUpdate,
		Fields: map[string]interface{}{
			"name": "test-portal",
		},
	}

	mockAPI := new(MockPortalAPI)
	
	// Simulate portal becoming protected after plan was generated
	mockAPI.On("ListPortals", mock.Anything, mock.Anything).Return(&kkOps.ListPortalsResponse{
		ListPortalsResponse: &kkComps.ListPortalsResponse{
			Data: []kkComps.Portal{
				{
					ID:   "portal-123",
					Name: "test-portal",
					Labels: map[string]string{
						labels.ManagedKey:   "true",
						labels.ProtectedKey: "true", // Protected after plan generation
					},
				},
			},
			Meta: kkComps.PaginatedMeta{
				Page: kkComps.PageMeta{Total: 1},
			},
		},
	}, nil)
	
	client := state.NewClient(state.ClientConfig{
		PortalAPI: mockAPI,
	})
	executor := New(client, nil, false)
	
	// Execute update
	_, err := executor.updatePortal(testContextWithLogger(), change)
	
	// Should fail due to protection
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resource is protected and cannot be updated")
	
	mockAPI.AssertExpectations(t)
}

// Ensure interfaces are satisfied
var _ helpers.PortalAPI = (*MockPortalAPI)(nil)