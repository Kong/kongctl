package state

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func TestListOrganizationTeamRolesPaginates(t *testing.T) {
	for _, scenario := range []struct {
		total, pageSize int
		pages           []int
	}{
		{0, 100, []int{1}},
		{1, 100, []int{1}},
		{9, 10, []int{1}},
		{10, 10, []int{1}},
		{11, 10, []int{1, 2}},
		{29, 10, []int{1, 2, 3}},
		{99, 100, []int{1}},
		{100, 100, []int{1}},
		{101, 100, []int{1, 2}},
		{199, 100, []int{1, 2}},
		{200, 100, []int{1, 2}},
		{201, 100, []int{1, 2, 3}},
		{3, 1, []int{1, 2, 3}},
	} {
		t.Run(fmt.Sprintf("total_%d_page_size_%d", scenario.total, scenario.pageSize), func(t *testing.T) {
			var pages []int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/v3/teams/team-1/assigned-roles", r.URL.Path)
				require.Equal(t, "100", r.URL.Query().Get("page[size]"))
				page, _ := strconv.Atoi(r.URL.Query().Get("page[number]"))
				if page == 0 {
					page = 1
				}
				pages = append(pages, page)
				data := make([]map[string]string, 0)
				for i := (page - 1) * scenario.pageSize; i < min(page*scenario.pageSize, scenario.total); i++ {
					data = append(data, map[string]string{
						"id": fmt.Sprintf("role-%d", i), "role_name": "Viewer",
						"entity_id": fmt.Sprintf("cp-%d", i), "entity_type_name": "Control Planes", "entity_region": "eu",
					})
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": data, "meta": map[string]any{"page": map[string]any{"total": scenario.total}},
				})
			}))
			defer server.Close()
			client := NewClient(ClientConfig{OrganizationTeamRolesAPI: &helpers.OrganizationTeamRolesAPIImpl{
				BaseURL: server.URL, HTTPClient: server.Client(),
			}})
			roles, err := client.ListOrganizationTeamRoles(t.Context(), "team-1")
			require.NoError(t, err)
			require.Len(t, roles, scenario.total)
			require.Equal(t, scenario.pages, pages)
			for i, role := range roles {
				require.Equal(t, fmt.Sprintf("role-%d", i), role.ID)
				require.Equal(t, "team-1", role.TeamID)
				require.Equal(t, "eu", role.EntityRegion)
			}
		})
	}
}

func TestListOrganizationTeamRolesRejectsPartialResults(t *testing.T) {
	for _, scenario := range []struct {
		name, body string
		status     int
		message    string
	}{
		{"later page error", `{"message":"unavailable"}`, 503, "status 503"},
		{"empty later page", `{"data":[],"meta":{"page":{"total":29}}}`, 200, "incomplete"},
		{"repeated page", `{"data":[{"id":"role-1"}],"meta":{"page":{"total":29}}}`, 200, "repeated ID"},
		{"missing metadata", `{"data":[]}`, 200, "pagination metadata"},
		{"malformed response", `{`, 200, "decode organization assigned roles"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				if calls == 1 {
					_, _ = w.Write([]byte(`{"data":[{"id":"role-1"}],"meta":{"page":{"total":29}}}`))
					return
				}
				w.WriteHeader(scenario.status)
				_, _ = w.Write([]byte(scenario.body))
			}))
			defer server.Close()
			client := NewClient(ClientConfig{OrganizationTeamRolesAPI: &helpers.OrganizationTeamRolesAPIImpl{
				BaseURL: server.URL, HTTPClient: server.Client(),
			}})
			roles, err := client.ListOrganizationTeamRoles(t.Context(), "team-1")
			require.Error(t, err)
			require.ErrorContains(t, err, scenario.message)
			require.Nil(t, roles)
			require.Equal(t, 2, calls)
		})
	}
}

func TestGetOrganizationTeamRoleStopsAtMatchingPage(t *testing.T) {
	for _, tc := range []struct {
		id    string
		pages []int
		found bool
	}{
		{"role-0", []int{1}, true}, {"role-100", []int{1, 2}, true}, {"absent", []int{1, 2}, false},
	} {
		t.Run(tc.id, func(t *testing.T) {
			var pages []int
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					page, err := strconv.Atoi(r.URL.Query().Get("page[number]"))
					require.NoError(t, err)
					pages = append(pages, page)
					values := make([]kkComps.AssignedRole, 0)
					for i := (page - 1) * 100; i < min(page*100, 101); i++ {
						values = append(
							values,
							kkComps.AssignedRole{ID: new(fmt.Sprintf("role-%d", i))},
						)
					}
					w.Header().Set("Content-Type", "application/json")
					require.NoError(
						t,
						json.NewEncoder(w).
							Encode(kkComps.AssignedRoleCollection{
								Data: values,
								Meta: &kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 101}},
							}),
					)
				}),
			)
			defer server.Close()
			client := NewClient(
				ClientConfig{
					OrganizationTeamRolesAPI: &helpers.OrganizationTeamRolesAPIImpl{
						BaseURL:    server.URL,
						HTTPClient: server.Client(),
					},
				},
			)
			role, err := client.GetOrganizationTeamRole(t.Context(), "team-1", tc.id)
			require.NoError(t, err)
			if tc.found {
				require.NotNil(t, role)
				require.Equal(t, tc.id, role.ID)
			} else {
				require.Nil(t, role)
			}
			require.Equal(t, tc.pages, pages)
		})
	}
}

type teamRolesWithoutMeta struct {
	helpers.OrganizationTeamRolesAPI
}

func (*teamRolesWithoutMeta) ListTeamRolesPage(
	context.Context,
	string,
	int64,
	int64,
) (*kkComps.AssignedRoleCollection, error) {
	return &kkComps.AssignedRoleCollection{Data: []kkComps.AssignedRole{{ID: new("role-1")}}}, nil
}

func TestListOrganizationTeamRolesRejectsMetadataOmittedByOtherImplementations(t *testing.T) {
	client := NewClient(ClientConfig{OrganizationTeamRolesAPI: &teamRolesWithoutMeta{}})
	roles, err := client.ListOrganizationTeamRoles(t.Context(), "team-1")
	require.ErrorContains(t, err, "missing assigned roles pagination metadata")
	require.Nil(t, roles)
}

func TestListOrganizationUserRolesReadsEveryPage(t *testing.T) {
	var pages []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v3/users/user-1/assigned-roles", r.URL.Path)
		page, err := strconv.Atoi(r.URL.Query().Get("page[number]"))
		require.NoError(t, err)
		pages = append(pages, page)
		data := make([]kkComps.AssignedRole, 0)
		for i := (page - 1) * 10; i < min(page*10, 29); i++ {
			data = append(data, kkComps.AssignedRole{
				ID:             new(fmt.Sprintf("role-%d", i)),
				RoleName:       new("Viewer"),
				EntityID:       new(fmt.Sprintf("cp-%d", i)),
				EntityTypeName: new("Control Planes"),
				EntityRegion:   new(kkComps.AssignedRoleEntityRegionEu),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(
			t,
			json.NewEncoder(w).
				Encode(kkComps.AssignedRoleCollection{
					Data: data,
					Meta: &kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 29}},
				}),
		)
	}))
	defer server.Close()
	client := NewClient(
		ClientConfig{
			OrganizationTeamRolesAPI: &helpers.OrganizationTeamRolesAPIImpl{
				BaseURL:    server.URL,
				HTTPClient: server.Client(),
			},
		},
	)
	roles, err := client.ListOrganizationUserRoles(t.Context(), "user-1")
	require.NoError(t, err)
	require.Len(t, roles, 29)
	require.Equal(t, []int{1, 2, 3}, pages)
	for i, role := range roles {
		require.Equal(t, fmt.Sprintf("role-%d", i), role.ID)
		require.Equal(t, "user-1", role.UserID)
		require.Equal(t, "eu", role.EntityRegion)
	}
}
