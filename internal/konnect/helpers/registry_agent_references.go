package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"

	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/util"
)

// RegistryAgentReferencesReader exposes only the cross-service inventory needed
// before Identity retirement. It does not authorize Registry mutations.
type RegistryAgentReferencesReader interface {
	ListAgents(context.Context, kkOps.ListAgentsRequest, ...kkOps.Option) (*kkOps.ListAgentsResponse, error)
	ListAgentPrincipals(context.Context, kkOps.ListAgentPrincipalsRequest, ...kkOps.Option) (
		*kkOps.ListAgentPrincipalsResponse, error)
}

type registryAgentReferencesReader struct {
	api     RegistryAgentReferencesReader
	baseURL string
}

func (r *registryAgentReferencesReader) ListAgents(ctx context.Context, request kkOps.ListAgentsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAgentsResponse, error) {
	if r.baseURL != "" {
		opts = append(opts, kkOps.WithServerURL(r.baseURL))
	}
	return r.api.ListAgents(ctx, request, opts...)
}

func (r *registryAgentReferencesReader) ListAgentPrincipals(
	ctx context.Context, request kkOps.ListAgentPrincipalsRequest,
	opts ...kkOps.Option,
) (*kkOps.ListAgentPrincipalsResponse, error) {
	if r.baseURL != "" {
		opts = append(opts, kkOps.WithServerURL(r.baseURL))
	}
	return r.api.ListAgentPrincipals(ctx, request, opts...)
}

type RegistryAgentPrincipalReference struct {
	ID          string `json:"id"`
	DirectoryID string `json:"directory_id"`
	PrincipalID string `json:"principal_id"`
	AgentID     string `json:"-"`
}

type registryReferencePage[T any] struct {
	Data []T `json:"data"`
	Meta *struct {
		Page *struct {
			Number *float64 `json:"number"`
			Size   *float64 `json:"size"`
			Total  *float64 `json:"total"`
		} `json:"page"`
	} `json:"meta"`
}

func listRegistryReferencePages[T any](ctx context.Context, kind string, size int64,
	page func(context.Context, int64, int64) (*http.Response, error), id func(T) string,
) ([]T, error) {
	values := make([]T, 0)
	seen := make(map[string]bool)
	var total, actualSize float64
	for number := int64(1); ; number++ {
		response, err := page(ctx, number, size)
		if err != nil {
			return nil, identitySecretAPIError("observe Registry "+kind+" before Identity deletion", err)
		}
		if response == nil || response.Body == nil || response.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("missing Registry %s collection response; deletion is blocked", kind)
		}
		data, err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		response.Body = io.NopCloser(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("cannot read Registry %s collection; deletion is blocked", kind)
		}
		var current registryReferencePage[T]
		if err := json.Unmarshal(data, &current); err != nil {
			return nil, fmt.Errorf("cannot decode Registry %s collection; deletion is blocked", kind)
		}
		if current.Data == nil || current.Meta == nil || current.Meta.Page == nil {
			return nil, fmt.Errorf("missing Registry %s data or pagination; deletion is blocked", kind)
		}
		meta := current.Meta.Page
		if meta.Number == nil || meta.Size == nil || meta.Total == nil || *meta.Number != float64(number) ||
			*meta.Size < 1 || *meta.Total < 0 || *meta.Size > 1<<53 || *meta.Total > 1<<53 ||
			math.Trunc(*meta.Size) != *meta.Size || math.Trunc(*meta.Total) != *meta.Total ||
			float64(len(current.Data)) > *meta.Size {
			return nil, fmt.Errorf("invalid Registry %s pagination; deletion is blocked", kind)
		}
		if number == 1 {
			total, actualSize = *meta.Total, *meta.Size
		} else if total != *meta.Total || actualSize != *meta.Size {
			return nil, fmt.Errorf("cannot verify Registry %s: inventory changed during pagination; retry the operation", kind)
		}
		for _, entry := range current.Data {
			key := id(entry)
			if !util.IsValidUUID(key) || seen[key] {
				return nil, fmt.Errorf("cannot verify Registry %s: invalid or repeated ID; deletion is blocked", kind)
			}
			seen[key] = true
		}
		values = append(values, current.Data...)
		if float64(len(values)) > total {
			return nil, fmt.Errorf("inconsistent Registry %s count; deletion is blocked", kind)
		}
		// A terminal empty page corroborates the advertised total rather than
		// trusting it to stop before an incoming reference on a later page.
		if len(current.Data) == 0 {
			if float64(len(values)) != total {
				return nil, fmt.Errorf("incomplete Registry %s inventory; deletion is blocked", kind)
			}
			return values, nil
		}
	}
}

// ListRegistryAgentPrincipalReferences observes every Registry Agent regardless
// of ownership or the selected declarative scope. Failed pages never yield a
// usable partial inventory.
func ListRegistryAgentPrincipalReferences(ctx context.Context, api RegistryAgentReferencesReader,
	pageSize int64,
) ([]RegistryAgentPrincipalReference, error) {
	if api == nil {
		return nil, fmt.Errorf("cannot verify Registry Agent references: reader is not configured; deletion is blocked")
	}
	if pageSize < 1 {
		pageSize = 100
	}
	type agent struct {
		ID string `json:"id"`
	}
	agents, err := listRegistryReferencePages(ctx, "Agents", pageSize,
		func(ctx context.Context, number, size int64) (*http.Response, error) {
			response, err := api.ListAgents(ctx, kkOps.ListAgentsRequest{PageNumber: &number, PageSize: &size})
			if err != nil || response == nil {
				return nil, err
			}
			return response.RawResponse, nil
		}, func(value agent) string { return value.ID })
	if err != nil {
		return nil, err
	}
	result := make([]RegistryAgentPrincipalReference, 0)
	for _, agent := range agents {
		links, err := listRegistryReferencePages(ctx, "Agent principal associations", pageSize,
			func(ctx context.Context, number, size int64) (*http.Response, error) {
				response, err := api.ListAgentPrincipals(ctx, kkOps.ListAgentPrincipalsRequest{
					AgentID: agent.ID, PageNumber: &number, PageSize: &size,
				})
				if err != nil || response == nil {
					return nil, err
				}
				return response.RawResponse, nil
			}, func(value RegistryAgentPrincipalReference) string { return value.ID })
		if err != nil {
			return nil, err
		}
		for _, link := range links {
			if !util.IsValidUUID(link.DirectoryID) || !util.IsValidUUID(link.PrincipalID) {
				return nil, fmt.Errorf("cannot verify Registry Agent association: invalid Identity target; deletion is blocked")
			}
			link.AgentID = agent.ID
			result = append(result, link)
		}
	}
	return result, nil
}
