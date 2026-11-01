package konnect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

// RegistryReferencesAPI supplies an explicit empty inventory unless a test adds
// agents, associations or an observation failure.
type RegistryReferencesAPI struct {
	Agents  []string
	Links   map[string][]helpers.RegistryAgentPrincipalReference
	Failure string
	Calls   int
}

func registryReferenceResponse[T any](values []T, size, number *int64) *http.Response {
	pageSize, pageNumber := int64(100), int64(1)
	if size != nil {
		pageSize = *size
	}
	if number != nil {
		pageNumber = *number
	}
	start := min((pageNumber-1)*pageSize, int64(len(values)))
	end := min(start+pageSize, int64(len(values)))
	data := append(make([]T, 0), values[start:end]...)
	body, err := json.Marshal(map[string]any{
		"data": data,
		"meta": map[string]any{"page": map[string]any{"number": pageNumber, "size": pageSize, "total": len(values)}},
	})
	if err != nil {
		panic(err)
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(body))}
}

func (a *RegistryReferencesAPI) ListAgents(_ context.Context, request kkOps.ListAgentsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAgentsResponse, error) {
	a.Calls++
	if a.Failure == "agents" {
		return nil, fmt.Errorf("registry agent observation failed")
	}
	type agent struct {
		ID string `json:"id"`
	}
	values := make([]agent, 0, len(a.Agents))
	for _, id := range a.Agents {
		values = append(values, agent{ID: id})
	}
	return &kkOps.ListAgentsResponse{
		RawResponse: registryReferenceResponse(values, request.PageSize, request.PageNumber),
	}, nil
}

func (a *RegistryReferencesAPI) ListAgentPrincipals(_ context.Context, request kkOps.ListAgentPrincipalsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListAgentPrincipalsResponse, error) {
	a.Calls++
	if a.Failure == "links" {
		return nil, fmt.Errorf("registry association observation failed")
	}
	return &kkOps.ListAgentPrincipalsResponse{RawResponse: registryReferenceResponse(a.Links[request.AgentID],
		request.PageSize, request.PageNumber)}, nil
}
