package state

import (
	"context"
	"fmt"
	"testing"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/konnect/helpers"
	kkInternalComps "github.com/Kong/sdk-konnect-go-internal/models/components"
	kkInternalOps "github.com/Kong/sdk-konnect-go-internal/models/operations"
)

// mockPortalAPI implements helpers.PortalAPI for testing
type mockPortalAPI struct {
	// ListPortals behavior
	listPortalsFunc func(context.Context, kkInternalOps.ListPortalsRequest) (*kkInternalOps.ListPortalsResponse, error)
	// CreatePortal behavior
	createPortalFunc func(context.Context, kkInternalComps.CreatePortal) (*kkInternalOps.CreatePortalResponse, error)
	// UpdatePortal behavior
	updatePortalFunc func(context.Context, string,
		kkInternalComps.UpdatePortal) (*kkInternalOps.UpdatePortalResponse, error)
	// GetPortal behavior
	getPortalFunc func(context.Context, string) (*kkInternalOps.GetPortalResponse, error)
	// DeletePortal behavior
	deletePortalFunc func(context.Context, string, bool) (*kkInternalOps.DeletePortalResponse, error)
}

func (m *mockPortalAPI) ListPortals(
	ctx context.Context,
	request kkInternalOps.ListPortalsRequest,
) (*kkInternalOps.ListPortalsResponse, error) {
	if m.listPortalsFunc != nil {
		return m.listPortalsFunc(ctx, request)
	}
	return nil, fmt.Errorf("ListPortals not implemented")
}

func (m *mockPortalAPI) CreatePortal(
	ctx context.Context,
	portal kkInternalComps.CreatePortal,
) (*kkInternalOps.CreatePortalResponse, error) {
	if m.createPortalFunc != nil {
		return m.createPortalFunc(ctx, portal)
	}
	return nil, fmt.Errorf("CreatePortal not implemented")
}

func (m *mockPortalAPI) UpdatePortal(
	ctx context.Context,
	id string,
	portal kkInternalComps.UpdatePortal,
) (*kkInternalOps.UpdatePortalResponse, error) {
	if m.updatePortalFunc != nil {
		return m.updatePortalFunc(ctx, id, portal)
	}
	return nil, fmt.Errorf("UpdatePortal not implemented")
}

func (m *mockPortalAPI) GetPortal(
	ctx context.Context,
	id string,
) (*kkInternalOps.GetPortalResponse, error) {
	if m.getPortalFunc != nil {
		return m.getPortalFunc(ctx, id)
	}
	return nil, fmt.Errorf("GetPortal not implemented")
}

func (m *mockPortalAPI) DeletePortal(
	ctx context.Context,
	id string,
	force bool,
) (*kkInternalOps.DeletePortalResponse, error) {
	if m.deletePortalFunc != nil {
		return m.deletePortalFunc(ctx, id, force)
	}
	return nil, fmt.Errorf("DeletePortal not implemented")
}

func TestListManagedPortals(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func() helpers.PortalAPI
		wantPortals int
		wantErr     bool
	}{
		{
			name: "single page with managed and unmanaged portals",
			setupMock: func() helpers.PortalAPI {
				return &mockPortalAPI{
					listPortalsFunc: func(_ context.Context,
						req kkInternalOps.ListPortalsRequest) (*kkInternalOps.ListPortalsResponse, error) {
						// First call returns data
						if *req.PageNumber == 1 {
							return &kkInternalOps.ListPortalsResponse{
								ListPortalsResponse: &kkInternalComps.ListPortalsResponse{
									Data: []kkInternalComps.Portal{
										{
											ID:   "portal-1",
											Name: "Managed Portal",
											Labels: map[string]string{
												labels.ManagedKey: "true",
											},
										},
										{
											ID:   "portal-2",
											Name: "Unmanaged Portal",
											Labels: map[string]string{
												"env": "production",
											},
										},
										{
											ID:   "portal-3",
											Name: "Another Managed",
											Labels: map[string]string{
												labels.ManagedKey:    "true",
												labels.LastUpdatedKey: "abc123",
											},
										},
									},
									Meta: kkInternalComps.PaginatedMeta{
										Page: kkInternalComps.PageMeta{
											Total: 3,
										},
									},
								},
							}, nil
						}
						// Subsequent calls return empty
						return &kkInternalOps.ListPortalsResponse{
							ListPortalsResponse: &kkInternalComps.ListPortalsResponse{
								Data: []kkInternalComps.Portal{},
							},
						}, nil
					},
				}
			},
			wantPortals: 2, // Only managed portals
			wantErr:     false,
		},
		{
			name: "multiple pages",
			setupMock: func() helpers.PortalAPI {
				return &mockPortalAPI{
					listPortalsFunc: func(_ context.Context,
						req kkInternalOps.ListPortalsRequest) (*kkInternalOps.ListPortalsResponse, error) {
						switch *req.PageNumber {
						case 1:
							return &kkInternalOps.ListPortalsResponse{
								ListPortalsResponse: &kkInternalComps.ListPortalsResponse{
									Data: []kkInternalComps.Portal{
										{
											ID:   "portal-1",
											Name: "Managed 1",
											Labels: map[string]string{
												labels.ManagedKey: "true",
											},
										},
									},
									Meta: kkInternalComps.PaginatedMeta{
										Page: kkInternalComps.PageMeta{
											Total: 200, // More than pageSize
										},
									},
								},
							}, nil
						case 2:
							return &kkInternalOps.ListPortalsResponse{
								ListPortalsResponse: &kkInternalComps.ListPortalsResponse{
									Data: []kkInternalComps.Portal{
										{
											ID:   "portal-2",
											Name: "Managed 2",
											Labels: map[string]string{
												labels.ManagedKey: "true",
											},
										},
									},
									Meta: kkInternalComps.PaginatedMeta{
										Page: kkInternalComps.PageMeta{
											Total: 200,
										},
									},
								},
							}, nil
						default:
							return &kkInternalOps.ListPortalsResponse{
								ListPortalsResponse: &kkInternalComps.ListPortalsResponse{
									Data: []kkInternalComps.Portal{},
								},
							}, nil
						}
					},
				}
			},
			wantPortals: 2,
			wantErr:     false,
		},
		{
			name: "API error",
			setupMock: func() helpers.PortalAPI {
				return &mockPortalAPI{
					listPortalsFunc: func(_ context.Context,
						_ kkInternalOps.ListPortalsRequest) (*kkInternalOps.ListPortalsResponse, error) {
						return nil, fmt.Errorf("API error")
					},
				}
			},
			wantPortals: 0,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.setupMock())
			portals, err := client.ListManagedPortals(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("ListManagedPortals() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(portals) != tt.wantPortals {
				t.Errorf("ListManagedPortals() got %d portals, want %d", len(portals), tt.wantPortals)
			}

			// Verify normalized labels
			for _, p := range portals {
				if p.NormalizedLabels == nil {
					t.Errorf("Portal %s has nil NormalizedLabels", p.ID)
				}
				if !labels.IsManagedResource(p.NormalizedLabels) {
					t.Errorf("Portal %s is not marked as managed", p.ID)
				}
			}
		})
	}
}

func TestGetPortalByName(t *testing.T) {
	tests := []struct {
		name       string
		portalName string
		setupMock  func() helpers.PortalAPI
		wantFound  bool
		wantErr    bool
	}{
		{
			name:       "found",
			portalName: "Target Portal",
			setupMock: func() helpers.PortalAPI {
				return &mockPortalAPI{
					listPortalsFunc: func(_ context.Context,
						_ kkInternalOps.ListPortalsRequest) (*kkInternalOps.ListPortalsResponse, error) {
						return &kkInternalOps.ListPortalsResponse{
							ListPortalsResponse: &kkInternalComps.ListPortalsResponse{
								Data: []kkInternalComps.Portal{
									{
										ID:   "portal-1",
										Name: "Other Portal",
										Labels: map[string]string{
											labels.ManagedKey: "true",
										},
									},
									{
										ID:   "portal-2",
										Name: "Target Portal",
										Labels: map[string]string{
											labels.ManagedKey: "true",
										},
									},
								},
							},
						}, nil
					},
				}
			},
			wantFound: true,
			wantErr:   false,
		},
		{
			name:       "not found",
			portalName: "Missing Portal",
			setupMock: func() helpers.PortalAPI {
				return &mockPortalAPI{
					listPortalsFunc: func(_ context.Context,
						_ kkInternalOps.ListPortalsRequest) (*kkInternalOps.ListPortalsResponse, error) {
						return &kkInternalOps.ListPortalsResponse{
							ListPortalsResponse: &kkInternalComps.ListPortalsResponse{
								Data: []kkInternalComps.Portal{
									{
										ID:   "portal-1",
										Name: "Other Portal",
										Labels: map[string]string{
											labels.ManagedKey: "true",
										},
									},
								},
							},
						}, nil
					},
				}
			},
			wantFound: false,
			wantErr:   false,
		},
		{
			name:       "list error",
			portalName: "Any Portal",
			setupMock: func() helpers.PortalAPI {
				return &mockPortalAPI{
					listPortalsFunc: func(_ context.Context,
						_ kkInternalOps.ListPortalsRequest) (*kkInternalOps.ListPortalsResponse, error) {
						return nil, fmt.Errorf("API error")
					},
				}
			},
			wantFound: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.setupMock())
			portal, err := client.GetPortalByName(context.Background(), tt.portalName)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetPortalByName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantFound && portal == nil {
				t.Errorf("GetPortalByName() expected to find portal but got nil")
			}
			if !tt.wantFound && portal != nil {
				t.Errorf("GetPortalByName() expected nil but got portal %s", portal.ID)
			}
			if portal != nil && portal.Name != tt.portalName {
				t.Errorf("GetPortalByName() got portal name %s, want %s", portal.Name, tt.portalName)
			}
		})
	}
}

func TestCreatePortal(t *testing.T) {
	tests := []struct {
		name       string
		portal     kkInternalComps.CreatePortal
		setupMock  func() helpers.PortalAPI
		wantErr    bool
		checkFunc  func(t *testing.T, resp *kkInternalComps.PortalResponse)
	}{
		{
			name: "successful create with labels",
			portal: kkInternalComps.CreatePortal{
				Name: "New Portal",
				Labels: map[string]*string{
					"env": ptr("production"),
				},
			},
			setupMock: func() helpers.PortalAPI {
				return &mockPortalAPI{
					createPortalFunc: func(_ context.Context,
						portal kkInternalComps.CreatePortal) (*kkInternalOps.CreatePortalResponse, error) {
						// Verify labels were added
						if portal.Labels[labels.ManagedKey] == nil || *portal.Labels[labels.ManagedKey] != "true" {
							t.Errorf("Expected managed label to be true")
						}
						if portal.Labels[labels.LastUpdatedKey] == nil {
							t.Errorf("Expected last updated label to be set")
						}
						// User label should still exist
						if portal.Labels["env"] == nil || *portal.Labels["env"] != "production" {
							t.Errorf("Expected env label to be preserved")
						}

						// Convert pointer map to regular map for response
						respLabels := make(map[string]string)
						for k, v := range portal.Labels {
							if v != nil {
								respLabels[k] = *v
							}
						}
						
						return &kkInternalOps.CreatePortalResponse{
							PortalResponse: &kkInternalComps.PortalResponse{
								ID:     "portal-new",
								Name:   portal.Name,
								Labels: respLabels,
							},
						}, nil
					},
				}
			},
			wantErr: false,
			checkFunc: func(t *testing.T, resp *kkInternalComps.PortalResponse) {
				if resp.ID != "portal-new" {
					t.Errorf("Expected portal ID portal-new, got %s", resp.ID)
				}
			},
		},
		{
			name: "API error",
			portal: kkInternalComps.CreatePortal{
				Name: "New Portal",
			},
			setupMock: func() helpers.PortalAPI {
				return &mockPortalAPI{
					createPortalFunc: func(_ context.Context,
						_ kkInternalComps.CreatePortal) (*kkInternalOps.CreatePortalResponse, error) {
						return nil, fmt.Errorf("API error")
					},
				}
			},
			wantErr: true,
		},
		{
			name: "nil response portal",
			portal: kkInternalComps.CreatePortal{
				Name: "New Portal",
			},
			setupMock: func() helpers.PortalAPI {
				return &mockPortalAPI{
					createPortalFunc: func(_ context.Context,
						_ kkInternalComps.CreatePortal) (*kkInternalOps.CreatePortalResponse, error) {
						return &kkInternalOps.CreatePortalResponse{
							PortalResponse: nil,
						}, nil
					},
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.setupMock())
			resp, err := client.CreatePortal(context.Background(), tt.portal)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreatePortal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkFunc != nil {
				tt.checkFunc(t, resp)
			}
		})
	}
}

func TestUpdatePortal(t *testing.T) {
	tests := []struct {
		name       string
		portalID   string
		portal     kkInternalComps.UpdatePortal
		setupMock  func() helpers.PortalAPI
		wantErr    bool
		checkFunc  func(t *testing.T, resp *kkInternalComps.PortalResponse)
	}{
		{
			name:     "successful update with labels",
			portalID: "portal-123",
			portal: kkInternalComps.UpdatePortal{
				Name: ptr("Updated Portal"),
				Labels: map[string]*string{
					"env": ptr("staging"),
				},
			},
			setupMock: func() helpers.PortalAPI {
				return &mockPortalAPI{
					updatePortalFunc: func(_ context.Context, id string,
						portal kkInternalComps.UpdatePortal) (*kkInternalOps.UpdatePortalResponse, error) {
						// Verify ID is passed correctly
						if id != "portal-123" {
							t.Errorf("Expected portal ID portal-123, got %s", id)
						}
						// Verify labels were added
						if portal.Labels[labels.ManagedKey] == nil || *portal.Labels[labels.ManagedKey] != "true" {
							t.Errorf("Expected managed label to be true")
						}
						if portal.Labels[labels.LastUpdatedKey] == nil || *portal.Labels[labels.LastUpdatedKey] == "" {
							t.Errorf("Expected config hash label to be newhash456")
						}
						if portal.Labels[labels.LastUpdatedKey] == nil {
							t.Errorf("Expected last updated label to be set")
						}
						// User label should still exist
						if portal.Labels["env"] == nil || *portal.Labels["env"] != "staging" {
							t.Errorf("Expected env label to be preserved")
						}

						// Convert pointer map to regular map for response
						respLabels := make(map[string]string)
						for k, v := range portal.Labels {
							if v != nil {
								respLabels[k] = *v
							}
						}
						
						return &kkInternalOps.UpdatePortalResponse{
							PortalResponse: &kkInternalComps.PortalResponse{
								ID:     id,
								Name:   *portal.Name,
								Labels: respLabels,
							},
						}, nil
					},
				}
			},
			wantErr: false,
			checkFunc: func(t *testing.T, resp *kkInternalComps.PortalResponse) {
				if resp.ID != "portal-123" {
					t.Errorf("Expected portal ID portal-123, got %s", resp.ID)
				}
				if resp.Name != "Updated Portal" {
					t.Errorf("Expected portal name Updated Portal, got %s", resp.Name)
				}
			},
		},
		{
			name:     "API error",
			portalID: "portal-123",
			portal: kkInternalComps.UpdatePortal{
				Name: ptr("Updated Portal"),
			},
			setupMock: func() helpers.PortalAPI {
				return &mockPortalAPI{
					updatePortalFunc: func(_ context.Context, _ string,
						_ kkInternalComps.UpdatePortal) (*kkInternalOps.UpdatePortalResponse, error) {
						return nil, fmt.Errorf("API error")
					},
				}
			},
			wantErr: true,
		},
		{
			name:     "nil response portal",
			portalID: "portal-123",
			portal: kkInternalComps.UpdatePortal{
				Name: ptr("Updated Portal"),
			},
			setupMock: func() helpers.PortalAPI {
				return &mockPortalAPI{
					updatePortalFunc: func(_ context.Context, _ string,
						_ kkInternalComps.UpdatePortal) (*kkInternalOps.UpdatePortalResponse, error) {
						return &kkInternalOps.UpdatePortalResponse{
							PortalResponse: nil,
						}, nil
					},
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.setupMock())
			resp, err := client.UpdatePortal(context.Background(), tt.portalID, tt.portal)

			if (err != nil) != tt.wantErr {
				t.Errorf("UpdatePortal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkFunc != nil {
				tt.checkFunc(t, resp)
			}
		})
	}
}

// Helper function
func ptr(s string) *string {
	return &s
}