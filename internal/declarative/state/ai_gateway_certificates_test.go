package state

import (
	"context"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListAIGatewayCertificatesFollowsCursorPagination(t *testing.T) {
	api := &testAIGatewayCertificatesAPI{t: t}
	client := NewClient(ClientConfig{AIGatewayCertificatesAPI: api})

	certificates, err := client.ListAIGatewayCertificates(t.Context(), "gateway-id")
	require.NoError(t, err)
	require.Len(t, certificates, 2)
	assert.Equal(t, []string{"", "next-cursor"}, api.pageAfter)
	assert.Equal(t, "first", certificates[0].Name)
	assert.Equal(t, "second", certificates[1].Name)
}

func TestListAIGatewayCACertificatesFollowsCursorPagination(t *testing.T) {
	api := &testAIGatewayCACertificatesAPI{t: t}
	client := NewClient(ClientConfig{AIGatewayCACertificatesAPI: api})

	certificates, err := client.ListAIGatewayCACertificates(t.Context(), "gateway-id")
	require.NoError(t, err)
	require.Len(t, certificates, 2)
	assert.Equal(t, []string{"", "next-cursor"}, api.pageAfter)
	assert.Equal(t, "first", certificates[0].Name)
	assert.Equal(t, "second", certificates[1].Name)
}

func TestListAIGatewaySNIsFollowsCursorPagination(t *testing.T) {
	api := &testAIGatewaySNIsAPI{t: t}
	client := NewClient(ClientConfig{AIGatewaySNIsAPI: api})

	snis, err := client.ListAIGatewaySNIs(t.Context(), "gateway-id")
	require.NoError(t, err)
	require.Len(t, snis, 2)
	assert.Equal(t, []string{"", "next-cursor"}, api.pageAfter)
	assert.Equal(t, "first", snis[0].Name)
	assert.Equal(t, "second", snis[1].Name)
}

type testAIGatewayCertificatesAPI struct {
	t         *testing.T
	pageAfter []string
}

func (a *testAIGatewayCertificatesAPI) ListAiGatewayCertificates(
	_ context.Context,
	request kkOps.ListAiGatewayCertificatesRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAiGatewayCertificatesResponse, error) {
	a.t.Helper()
	cursor := ""
	if request.PageAfter != nil {
		cursor = *request.PageAfter
	}
	a.pageAfter = append(a.pageAfter, cursor)
	if cursor == "" {
		next := "https://example.test/certificates?page%5Bafter%5D=next-cursor"
		return &kkOps.ListAiGatewayCertificatesResponse{
			ListAIGatewayCertificatesResponse: &kkComps.ListAIGatewayCertificatesResponse{
				Data: []kkComps.AIGatewayCertificate{{ID: "first-id", Name: "first", Cert: "cert-1"}},
				Meta: kkComps.CursorMeta{Page: kkComps.CursorMetaPage{Next: &next}},
			},
		}, nil
	}
	return &kkOps.ListAiGatewayCertificatesResponse{
		ListAIGatewayCertificatesResponse: &kkComps.ListAIGatewayCertificatesResponse{
			Data: []kkComps.AIGatewayCertificate{{ID: "second-id", Name: "second", Cert: "cert-2"}},
			Meta: kkComps.CursorMeta{Page: kkComps.CursorMetaPage{}},
		},
	}, nil
}

func (*testAIGatewayCertificatesAPI) CreateAiGatewayCertificate(
	context.Context, string, kkComps.CreateAIGatewayCertificateRequest, ...kkOps.Option,
) (*kkOps.CreateAiGatewayCertificateResponse, error) {
	return nil, nil
}

func (*testAIGatewayCertificatesAPI) GetAiGatewayCertificate(
	context.Context, string, string, ...kkOps.Option,
) (*kkOps.GetAiGatewayCertificateResponse, error) {
	return nil, nil
}

func (*testAIGatewayCertificatesAPI) UpdateAiGatewayCertificate(
	context.Context, kkOps.UpdateAiGatewayCertificateRequest, ...kkOps.Option,
) (*kkOps.UpdateAiGatewayCertificateResponse, error) {
	return nil, nil
}

func (*testAIGatewayCertificatesAPI) DeleteAiGatewayCertificate(
	context.Context, string, string, ...kkOps.Option,
) (*kkOps.DeleteAiGatewayCertificateResponse, error) {
	return nil, nil
}

type testAIGatewayCACertificatesAPI struct {
	helpers.AIGatewayCACertificatesAPI
	t         *testing.T
	pageAfter []string
}

func (a *testAIGatewayCACertificatesAPI) ListAiGatewayCaCertificates(
	_ context.Context,
	request kkOps.ListAiGatewayCaCertificatesRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAiGatewayCaCertificatesResponse, error) {
	a.t.Helper()
	cursor := requestedPageAfter(request.PageAfter)
	a.pageAfter = append(a.pageAfter, cursor)
	next := nextPageURL(cursor)
	name := "first"
	if cursor != "" {
		name = "second"
	}
	return &kkOps.ListAiGatewayCaCertificatesResponse{
		ListAIGatewayCACertificatesResponse: &kkComps.ListAIGatewayCACertificatesResponse{
			Data: []kkComps.AIGatewayCACertificate{{ID: name + "-id", Name: name, Cert: "cert"}},
			Meta: kkComps.CursorMeta{Page: kkComps.CursorMetaPage{Next: next}},
		},
	}, nil
}

type testAIGatewaySNIsAPI struct {
	helpers.AIGatewaySNIsAPI
	t         *testing.T
	pageAfter []string
}

func (a *testAIGatewaySNIsAPI) ListAiGatewaySnis(
	_ context.Context,
	request kkOps.ListAiGatewaySnisRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAiGatewaySnisResponse, error) {
	a.t.Helper()
	cursor := requestedPageAfter(request.PageAfter)
	a.pageAfter = append(a.pageAfter, cursor)
	next := nextPageURL(cursor)
	name := "first"
	if cursor != "" {
		name = "second"
	}
	return &kkOps.ListAiGatewaySnisResponse{
		ListAIGatewaySNIsResponse: &kkComps.ListAIGatewaySNIsResponse{
			Data: []kkComps.AIGatewaySNI{{
				ID: name + "-id", Name: name, DisplayName: name, Hostname: "example.test", Certificate: "cert",
			}},
			Meta: kkComps.CursorMeta{Page: kkComps.CursorMetaPage{Next: next}},
		},
	}, nil
}

func requestedPageAfter(after *string) string {
	if after == nil {
		return ""
	}
	return *after
}

func nextPageURL(cursor string) *string {
	if cursor != "" {
		return nil
	}
	next := "https://example.test/resources?page%5Bafter%5D=next-cursor"
	return &next
}
