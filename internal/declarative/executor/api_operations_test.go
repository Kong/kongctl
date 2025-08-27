package executor

import (
	"context"
	"errors"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAPIAPI is a mock implementation of the API API interface
type MockAPIAPI struct {
	mock.Mock
}

func (m *MockAPIAPI) ListApis(
	ctx context.Context, req kkOps.ListApisRequest, opts ...kkOps.Option,
) (*kkOps.ListApisResponse, error) {
	args := m.Called(ctx, req, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.ListApisResponse), args.Error(1)
}

func (m *MockAPIAPI) CreateAPI(
	ctx context.Context, req kkComps.CreateAPIRequest, opts ...kkOps.Option,
) (*kkOps.CreateAPIResponse, error) {
	args := m.Called(ctx, req, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.CreateAPIResponse), args.Error(1)
}

func (m *MockAPIAPI) UpdateAPI(
	ctx context.Context, id string, req kkComps.UpdateAPIRequest, opts ...kkOps.Option,
) (*kkOps.UpdateAPIResponse, error) {
	args := m.Called(ctx, id, req, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.UpdateAPIResponse), args.Error(1)
}

func (m *MockAPIAPI) DeleteAPI(
	ctx context.Context, id string, opts ...kkOps.Option,
) (*kkOps.DeleteAPIResponse, error) {
	args := m.Called(ctx, id, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.DeleteAPIResponse), args.Error(1)
}

func (m *MockAPIAPI) FetchAPI(
	ctx context.Context, id string, opts ...kkOps.Option,
) (*kkOps.FetchAPIResponse, error) {
	args := m.Called(ctx, id, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*kkOps.FetchAPIResponse), args.Error(1)
}

func TestExecutor_deleteAPI(t *testing.T) {
	tests := []struct {
		name      string
		change    planner.PlannedChange
		setupMock func(*MockAPIAPI)
		wantErr   bool
		errMsg    string
	}{
		{
			name: "successful delete",
			change: planner.PlannedChange{
				ResourceType: "api",
				ResourceID:   "api-123",
				Action:       planner.ActionDelete,
				Fields: map[string]any{
					"name": "delete-api",
				},
			},
			setupMock: func(m *MockAPIAPI) {
				// Mock GetAPIByName for protection check
				m.On("ListApis", mock.Anything, mock.Anything, mock.Anything).Return(&kkOps.ListApisResponse{
					ListAPIResponse: &kkComps.ListAPIResponse{
						Data: []kkComps.APIResponseSchema{
							{
								ID:   "api-123",
								Name: "delete-api",
								Labels: map[string]string{
									labels.NamespaceKey: "default",
								},
							},
						},
						Meta: kkComps.PaginatedMeta{
							Page: kkComps.PageMeta{Total: 1},
						},
					},
				}, nil)

				// Mock DeleteAPI
				m.On("DeleteAPI", mock.Anything, "api-123", mock.Anything).
					Return(&kkOps.DeleteAPIResponse{}, nil)
			},
			wantErr: false,
		},
		{
			name: "protected API blocks deletion",
			change: planner.PlannedChange{
				ResourceType: "api",
				ResourceID:   "api-456",
				Action:       planner.ActionDelete,
				Fields: map[string]any{
					"name": "protected-api",
				},
			},
			setupMock: func(m *MockAPIAPI) {
				m.On("ListApis", mock.Anything, mock.Anything, mock.Anything).Return(&kkOps.ListApisResponse{
					ListAPIResponse: &kkComps.ListAPIResponse{
						Data: []kkComps.APIResponseSchema{
							{
								ID:   "api-456",
								Name: "protected-api",
								Labels: map[string]string{
									labels.NamespaceKey: "default",
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
			name: "delete non-managed API - not found by managed API search",
			change: planner.PlannedChange{
				ResourceType: "api",
				ResourceID:   "api-789",
				Action:       planner.ActionDelete,
				Fields: map[string]any{
					"name": "unmanaged-api",
				},
			},
			setupMock: func(m *MockAPIAPI) {
				// ListAPIs will be called but won't return this API
				// because it's not managed (ListManagedAPIs filters it out)
				m.On("ListApis", mock.Anything, mock.Anything, mock.Anything).Return(&kkOps.ListApisResponse{
					ListAPIResponse: &kkComps.ListAPIResponse{
						Data: []kkComps.APIResponseSchema{}, // Empty - unmanaged API filtered out
						Meta: kkComps.PaginatedMeta{
							Page: kkComps.PageMeta{Total: 0},
						},
					},
				}, nil)

				// Delete won't be called because API is not found in managed APIs
				// This is correct behavior - we only delete APIs we manage
			},
			wantErr: false, // Success - API not found means nothing to do
		},
		{
			name: "API already deleted",
			change: planner.PlannedChange{
				ResourceType: "api",
				ResourceID:   "api-999",
				Action:       planner.ActionDelete,
				Fields: map[string]any{
					"name": "already-deleted",
				},
			},
			setupMock: func(m *MockAPIAPI) {
				m.On("ListApis", mock.Anything, mock.Anything, mock.Anything).Return(&kkOps.ListApisResponse{
					ListAPIResponse: &kkComps.ListAPIResponse{
						Data: []kkComps.APIResponseSchema{},
						Meta: kkComps.PaginatedMeta{
							Page: kkComps.PageMeta{Total: 0},
						},
					},
				}, nil)
			},
			wantErr: false, // Already deleted is considered success
		},
		{
			name: "API deletion fails",
			change: planner.PlannedChange{
				ResourceType: "api",
				ResourceID:   "api-fail",
				Action:       planner.ActionDelete,
				Fields: map[string]any{
					"name": "fail-api",
				},
			},
			setupMock: func(m *MockAPIAPI) {
				m.On("ListApis", mock.Anything, mock.Anything, mock.Anything).Return(&kkOps.ListApisResponse{
					ListAPIResponse: &kkComps.ListAPIResponse{
						Data: []kkComps.APIResponseSchema{
							{
								ID:   "api-fail",
								Name: "fail-api",
								Labels: map[string]string{
									labels.NamespaceKey: "default",
								},
							},
						},
						Meta: kkComps.PaginatedMeta{
							Page: kkComps.PageMeta{Total: 1},
						},
					},
				}, nil)

				// Mock DeleteAPI to fail
				m.On("DeleteAPI", mock.Anything, "api-fail", mock.Anything).
					Return(nil, errors.New("API error"))
			},
			wantErr: true,
			errMsg:  "failed to delete API",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockAPI := new(MockAPIAPI)
			tt.setupMock(mockAPI)

			client := state.NewClient(state.ClientConfig{
				APIAPI: mockAPI,
			})
			executor := New(client, nil, false)

			// Execute
			err := executor.deleteAPI(testContextWithLogger(), tt.change)

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

func TestExecutor_createAPI(t *testing.T) {
	tests := []struct {
		name      string
		change    planner.PlannedChange
		setupMock func(*MockAPIAPI)
		wantErr   bool
		wantID    string
	}{
		{
			name: "successful create",
			change: planner.PlannedChange{
				ResourceType: "api",
				Action:       planner.ActionCreate,
				Fields: map[string]any{
					"name":        "new-api",
					"description": "Test API",
					"labels": map[string]any{
						"env": "test",
					},
				},
				Protection: false,
				Namespace:  "default",
			},
			setupMock: func(m *MockAPIAPI) {
				m.On("CreateAPI", mock.Anything, mock.MatchedBy(func(req kkComps.CreateAPIRequest) bool {
					return req.Name == "new-api" &&
						*req.Description == "Test API" &&
						req.Labels["env"] == "test" &&
						req.Labels[labels.NamespaceKey] == "default" &&
						req.Labels[labels.ProtectedKey] == "" // No protected label when false
				}), mock.Anything).Return(&kkOps.CreateAPIResponse{
					APIResponseSchema: &kkComps.APIResponseSchema{
						ID:   "api-created-123",
						Name: "new-api",
					},
				}, nil)
			},
			wantErr: false,
			wantID:  "api-created-123",
		},
		{
			name: "create with protection",
			change: planner.PlannedChange{
				ResourceType: "api",
				Action:       planner.ActionCreate,
				Fields: map[string]any{
					"name": "protected-api",
				},
				Protection: true,
				Namespace:  "default",
			},
			setupMock: func(m *MockAPIAPI) {
				m.On("CreateAPI", mock.Anything, mock.MatchedBy(func(req kkComps.CreateAPIRequest) bool {
					return req.Name == "protected-api" &&
						req.Labels[labels.ProtectedKey] == "true" &&
						req.Labels[labels.NamespaceKey] == "default"
				}), mock.Anything).Return(&kkOps.CreateAPIResponse{
					APIResponseSchema: &kkComps.APIResponseSchema{
						ID:   "api-protected-456",
						Name: "protected-api",
					},
				}, nil)
			},
			wantErr: false,
			wantID:  "api-protected-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockAPI := new(MockAPIAPI)
			tt.setupMock(mockAPI)

			client := state.NewClient(state.ClientConfig{
				APIAPI: mockAPI,
			})
			executor := New(client, nil, false)

			// Execute
			id, err := executor.createAPI(testContextWithLogger(), tt.change)

			// Verify
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantID, id)
			}

			mockAPI.AssertExpectations(t)
		})
	}
}
