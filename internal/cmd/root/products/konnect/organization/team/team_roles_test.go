package team

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
	cmdtest "github.com/kong/kongctl/test/cmd"
	"github.com/stretchr/testify/require"
)

func TestFetchOrganizationTeamRolesReadsEveryPage(t *testing.T) {
	var pages []int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, err := strconv.Atoi(r.URL.Query().Get("page[number]"))
		require.NoError(t, err)
		pages = append(pages, page)
		roles := make([]kkComps.AssignedRole, 0)
		for i := (page - 1) * 100; i < min(page*100, 101); i++ {
			roles = append(roles, kkComps.AssignedRole{ID: new(fmt.Sprintf("role-%d", i))})
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(kkComps.AssignedRoleCollection{
			Data: roles, Meta: &kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 101}},
		}))
	}))
	defer server.Close()
	helper := &cmdtest.MockHelper{GetContextMock: func() context.Context { return t.Context() }}
	roles, err := fetchOrganizationTeamRoles(helper, &helpers.OrganizationTeamRolesAPIImpl{
		BaseURL: server.URL, HTTPClient: server.Client(),
	}, "team-1")
	require.NoError(t, err)
	require.Len(t, roles, 101)
	require.Equal(t, []int{1, 2}, pages)
	for i, role := range roles {
		require.Equal(t, fmt.Sprintf("role-%d", i), *role.ID)
	}
}

func TestFetchOrganizationTeamRolesPreservesEmptyArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[],"meta":{"page":{"total":0}}}`))
	}))
	defer server.Close()
	helper := &cmdtest.MockHelper{GetContextMock: func() context.Context { return t.Context() }}
	roles, err := fetchOrganizationTeamRoles(
		helper,
		&helpers.OrganizationTeamRolesAPIImpl{BaseURL: server.URL, HTTPClient: server.Client()},
		"team",
	)
	require.NoError(t, err)
	encoded, err := json.Marshal(roles)
	require.NoError(t, err)
	require.JSONEq(t, `[]`, string(encoded))
}
