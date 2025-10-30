package state

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/log"
)

// testContextWithLogger returns a context with a test logger
func testContextWithLogger() context.Context {
	logger := slog.Default()
	return context.WithValue(context.Background(), log.LoggerKey, logger)
}

// mockPortalAPI implements helpers.PortalAPI for testing
type mockPortalAPI struct {
	// ListPortals behavior
	listPortalsFunc func(context.Context, kkOps.ListPortalsRequest) (*kkOps.ListPortalsResponse, error)
	// CreatePortal behavior
	createPortalFunc func(context.Context, kkComps.CreatePortal) (*kkOps.CreatePortalResponse, error)
	// UpdatePortal behavior
	updatePortalFunc func(context.Context, string,
		kkComps.UpdatePortal) (*kkOps.UpdatePortalResponse, error)
	// GetPortal behavior
	getPortalFunc func(context.Context, string) (*kkOps.GetPortalResponse, error)
	// DeletePortal behavior
	deletePortalFunc func(context.Context, string, bool) (*kkOps.DeletePortalResponse, error)
}

func (m *mockPortalAPI) ListPortals(
	ctx context.Context,
	request kkOps.ListPortalsRequest,
) (*kkOps.ListPortalsResponse, error) {
	if m.listPortalsFunc != nil {
		return m.listPortalsFunc(ctx, request)
	}
	return nil, fmt.Errorf("ListPortals not implemented")
}

func (m *mockPortalAPI) CreatePortal(
	ctx context.Context,
	portal kkComps.CreatePortal,
) (*kkOps.CreatePortalResponse, error) {
	if m.createPortalFunc != nil {
		return m.createPortalFunc(ctx, portal)
	}
	return nil, fmt.Errorf("CreatePortal not implemented")
}

func (m *mockPortalAPI) UpdatePortal(
	ctx context.Context,
	id string,
	portal kkComps.UpdatePortal,
) (*kkOps.UpdatePortalResponse, error) {
	if m.updatePortalFunc != nil {
		return m.updatePortalFunc(ctx, id, portal)
	}
	return nil, fmt.Errorf("UpdatePortal not implemented")
}

func (m *mockPortalAPI) GetPortal(
	ctx context.Context,
	id string,
) (*kkOps.GetPortalResponse, error) {
	if m.getPortalFunc != nil {
		return m.getPortalFunc(ctx, id)
	}
	return nil, fmt.Errorf("GetPortal not implemented")
}

func (m *mockPortalAPI) DeletePortal(
	ctx context.Context,
	id string,
	force bool,
) (*kkOps.DeletePortalResponse, error) {
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
						req kkOps.ListPortalsRequest,
					) (*kkOps.ListPortalsResponse, error) {
						// First call returns data
						if *req.PageNumber == 1 {
							return &kkOps.ListPortalsResponse{
								ListPortalsResponse: &kkComps.ListPortalsResponse{
									Data: []kkComps.ListPortalsResponsePortal{
										newListPortal("portal-1", "Managed Portal", map[string]string{labels.NamespaceKey: "default"}),
										newListPortal("portal-2", "Unmanaged Portal", map[string]string{"env": "production"}),
										newListPortal("portal-3", "Another Managed", map[string]string{labels.NamespaceKey: "team-a"}),
									},
									Meta: kkComps.PaginatedMeta{
										Page: kkComps.PageMeta{
											Total: 3,
										},
									},
								},
							}, nil
						}
						// Subsequent calls return empty
						return &kkOps.ListPortalsResponse{
							ListPortalsResponse: &kkComps.ListPortalsResponse{
								Data: []kkComps.ListPortalsResponsePortal{},
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
						req kkOps.ListPortalsRequest,
					) (*kkOps.ListPortalsResponse, error) {
						switch *req.PageNumber {
						case 1:
							return &kkOps.ListPortalsResponse{
								ListPortalsResponse: &kkComps.ListPortalsResponse{
									Data: []kkComps.ListPortalsResponsePortal{
										newListPortal("portal-1", "Managed 1", map[string]string{labels.NamespaceKey: "default"}),
									},
									Meta: kkComps.PaginatedMeta{
										Page: kkComps.PageMeta{
											Total: 200, // More than pageSize
										},
									},
								},
							}, nil
						case 2:
							return &kkOps.ListPortalsResponse{
								ListPortalsResponse: &kkComps.ListPortalsResponse{
									Data: []kkComps.ListPortalsResponsePortal{
										newListPortal("portal-2", "Managed 2", map[string]string{labels.NamespaceKey: "default"}),
									},
									Meta: kkComps.PaginatedMeta{
										Page: kkComps.PageMeta{
											Total: 200,
										},
									},
								},
							}, nil
						default:
							return &kkOps.ListPortalsResponse{
								ListPortalsResponse: &kkComps.ListPortalsResponse{
									Data: []kkComps.ListPortalsResponsePortal{},
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
						_ kkOps.ListPortalsRequest,
					) (*kkOps.ListPortalsResponse, error) {
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
			client := NewClient(ClientConfig{
				PortalAPI: tt.setupMock(),
			})
			portals, err := client.ListManagedPortals(testContextWithLogger(), []string{"*"})

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
						_ kkOps.ListPortalsRequest,
					) (*kkOps.ListPortalsResponse, error) {
						return &kkOps.ListPortalsResponse{
							ListPortalsResponse: &kkComps.ListPortalsResponse{
								Data: []kkComps.ListPortalsResponsePortal{
									newListPortal("portal-1", "Other Portal", map[string]string{labels.NamespaceKey: "default"}),
									newListPortal("portal-2", "Target Portal", map[string]string{labels.NamespaceKey: "default"}),
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
						_ kkOps.ListPortalsRequest,
					) (*kkOps.ListPortalsResponse, error) {
						return &kkOps.ListPortalsResponse{
							ListPortalsResponse: &kkComps.ListPortalsResponse{
								Data: []kkComps.ListPortalsResponsePortal{
									newListPortal("portal-1", "Other Portal", map[string]string{labels.NamespaceKey: "default"}),
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
						_ kkOps.ListPortalsRequest,
					) (*kkOps.ListPortalsResponse, error) {
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
			client := NewClient(ClientConfig{
				PortalAPI: tt.setupMock(),
			})
			portal, err := client.GetPortalByName(testContextWithLogger(), tt.portalName)

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
		name      string
		portal    kkComps.CreatePortal
		setupMock func() helpers.PortalAPI
		wantErr   bool
		checkFunc func(t *testing.T, resp *kkComps.PortalResponse)
	}{
		{
			name: "successful create with labels",
			portal: kkComps.CreatePortal{
				Name: "New Portal",
				Labels: map[string]*string{
					"env": ptr("production"),
				},
			},
			setupMock: func() helpers.PortalAPI {
				return &mockPortalAPI{
					createPortalFunc: func(_ context.Context,
						portal kkComps.CreatePortal,
					) (*kkOps.CreatePortalResponse, error) {
						// State client no longer adds labels - executor handles it
						// Just verify user labels are preserved
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

						return &kkOps.CreatePortalResponse{
							PortalResponse: &kkComps.PortalResponse{
								ID:     "portal-new",
								Name:   portal.Name,
								Labels: respLabels,
							},
						}, nil
					},
				}
			},
			wantErr: false,
			checkFunc: func(t *testing.T, resp *kkComps.PortalResponse) {
				if resp.ID != "portal-new" {
					t.Errorf("Expected portal ID portal-new, got %s", resp.ID)
				}
			},
		},
		{
			name: "API error",
			portal: kkComps.CreatePortal{
				Name: "New Portal",
			},
			setupMock: func() helpers.PortalAPI {
				return &mockPortalAPI{
					createPortalFunc: func(_ context.Context,
						_ kkComps.CreatePortal,
					) (*kkOps.CreatePortalResponse, error) {
						return nil, fmt.Errorf("API error")
					},
				}
			},
			wantErr: true,
		},
		{
			name: "nil response portal",
			portal: kkComps.CreatePortal{
				Name: "New Portal",
			},
			setupMock: func() helpers.PortalAPI {
				return &mockPortalAPI{
					createPortalFunc: func(_ context.Context,
						_ kkComps.CreatePortal,
					) (*kkOps.CreatePortalResponse, error) {
						return &kkOps.CreatePortalResponse{
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
			client := NewClient(ClientConfig{
				PortalAPI: tt.setupMock(),
			})
			resp, err := client.CreatePortal(testContextWithLogger(), tt.portal, "default")

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
		name      string
		portalID  string
		portal    kkComps.UpdatePortal
		setupMock func() helpers.PortalAPI
		wantErr   bool
		checkFunc func(t *testing.T, resp *kkComps.PortalResponse)
	}{
		{
			name:     "successful update with labels",
			portalID: "portal-123",
			portal: kkComps.UpdatePortal{
				Name: ptr("Updated Portal"),
				Labels: map[string]*string{
					"env": ptr("staging"),
				},
			},
			setupMock: func() helpers.PortalAPI {
				return &mockPortalAPI{
					updatePortalFunc: func(_ context.Context, id string,
						portal kkComps.UpdatePortal,
					) (*kkOps.UpdatePortalResponse, error) {
						// Verify ID is passed correctly
						if id != "portal-123" {
							t.Errorf("Expected portal ID portal-123, got %s", id)
						}
						// State client no longer adds labels - executor handles it
						// Just verify that labels are passed through
						// User label should exist
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

						return &kkOps.UpdatePortalResponse{
							PortalResponse: &kkComps.PortalResponse{
								ID:     id,
								Name:   *portal.Name,
								Labels: respLabels,
							},
						}, nil
					},
				}
			},
			wantErr: false,
			checkFunc: func(t *testing.T, resp *kkComps.PortalResponse) {
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
			portal: kkComps.UpdatePortal{
				Name: ptr("Updated Portal"),
			},
			setupMock: func() helpers.PortalAPI {
				return &mockPortalAPI{
					updatePortalFunc: func(_ context.Context, _ string,
						_ kkComps.UpdatePortal,
					) (*kkOps.UpdatePortalResponse, error) {
						return nil, fmt.Errorf("API error")
					},
				}
			},
			wantErr: true,
		},
		{
			name:     "nil response portal",
			portalID: "portal-123",
			portal: kkComps.UpdatePortal{
				Name: ptr("Updated Portal"),
			},
			setupMock: func() helpers.PortalAPI {
				return &mockPortalAPI{
					updatePortalFunc: func(_ context.Context, _ string,
						_ kkComps.UpdatePortal,
					) (*kkOps.UpdatePortalResponse, error) {
						return &kkOps.UpdatePortalResponse{
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
			client := NewClient(ClientConfig{
				PortalAPI: tt.setupMock(),
			})
			resp, err := client.UpdatePortal(testContextWithLogger(), tt.portalID, tt.portal, "default")

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

func newListPortal(id, name string, labels map[string]string) kkComps.ListPortalsResponsePortal {
	return kkComps.ListPortalsResponsePortal{
		ID:     id,
		Name:   name,
		Labels: labels,
	}
}
