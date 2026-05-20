package dump

import (
	"bytes"
	"context"
	"errors"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type portalPaginationStub struct {
	t               *testing.T
	listPortalsFunc func(context.Context, kkOps.ListPortalsRequest) (*kkOps.ListPortalsResponse, error)
}

func (p *portalPaginationStub) ListPortals(
	ctx context.Context,
	req kkOps.ListPortalsRequest,
) (*kkOps.ListPortalsResponse, error) {
	if p.listPortalsFunc != nil {
		return p.listPortalsFunc(ctx, req)
	}
	p.t.Fatalf("unexpected ListPortals call")
	return nil, nil
}

func (p *portalPaginationStub) GetPortal(context.Context, string) (*kkOps.GetPortalResponse, error) {
	p.t.Fatalf("unexpected GetPortal call")
	return nil, nil
}

func (p *portalPaginationStub) CreatePortal(
	context.Context,
	kkComps.CreatePortal,
) (*kkOps.CreatePortalResponse, error) {
	p.t.Fatalf("unexpected CreatePortal call")
	return nil, nil
}

func (p *portalPaginationStub) UpdatePortal(
	context.Context,
	string,
	kkComps.UpdatePortal,
) (*kkOps.UpdatePortalResponse, error) {
	p.t.Fatalf("unexpected UpdatePortal call")
	return nil, nil
}

func (p *portalPaginationStub) DeletePortal(context.Context, string, bool) (*kkOps.DeletePortalResponse, error) {
	p.t.Fatalf("unexpected DeletePortal call")
	return nil, nil
}

func TestDumpPortals_ExactPageBoundaryDoesNotRequestExtraPage(t *testing.T) {
	var requestedPages []int64

	api := &portalPaginationStub{
		t: t,
		listPortalsFunc: func(
			_ context.Context,
			req kkOps.ListPortalsRequest,
		) (*kkOps.ListPortalsResponse, error) {
			pageNumber := int64(1)
			if req.PageNumber != nil {
				pageNumber = *req.PageNumber
			}
			requestedPages = append(requestedPages, pageNumber)

			switch pageNumber {
			case 1:
				return &kkOps.ListPortalsResponse{
					ListPortalsResponse: &kkComps.ListPortalsResponse{
						Data: []kkComps.ListPortalsResponsePortal{
							{ID: "portal-1", Name: "portal-one"},
						},
						Meta: kkComps.PaginatedMeta{
							Page: kkComps.PageMeta{Total: 1},
						},
					},
				}, nil
			default:
				t.Fatalf("unexpected page request: %d", pageNumber)
				return nil, nil
			}
		},
	}

	var output bytes.Buffer
	err := dumpPortals(t.Context(), &output, api, 1, false, filterOptions{})
	require.NoError(t, err)
	assert.Equal(t, []int64{1}, requestedPages)
	assert.Contains(t, output.String(), "konnect_portal.portal_one")
}

func TestProcessPaginatedRequests_ReturnsExplicitErrorWhenPageLimitExceeded(t *testing.T) {
	sentinel := errors.New("requested page beyond test sentinel")

	err := processPaginatedRequests(func(pageNumber int64) (bool, error) {
		if pageNumber > 10000 {
			return false, sentinel
		}
		return true, nil
	})
	require.Error(t, err)
	assert.NotErrorIs(t, err, sentinel)
	assert.Contains(t, err.Error(), "pagination")
}
