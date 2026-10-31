package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/pagination"
)

type IdentityGroupsReader interface {
	ListGroups(context.Context, kkOps.ListGroupsRequest, ...kkOps.Option) (*kkOps.ListGroupsResponse, error)
	GetGroup(context.Context, string, string, ...kkOps.Option) (*kkOps.GetGroupResponse, error)
}

// IdentityGroup preserves response presence instead of adding SDK defaults for
// provisioning and team-manager fields absent from older deployments.
type IdentityGroup struct {
	ID            string                                  `json:"id"                       yaml:"id"`
	Name          string                                  `json:"name"                     yaml:"name"`
	Description   string                                  `json:"description"              yaml:"description"`
	Type          *kkComps.KongGroupType                  `json:"type,omitempty"           yaml:"type,omitempty"`
	Metadata      map[string]kkComps.KongIdentityMetadata `json:"metadata"                 yaml:"metadata"`
	Labels        map[string]string                       `json:"labels"                   yaml:"labels"`
	ManagedBy     map[string]string                       `json:"managed_by"               yaml:"managed_by"`
	ProvisionedBy *kkComps.ProvisionedBy                  `json:"provisioned_by,omitempty" yaml:"provisioned_by,omitempty"`
	TeamManager   *kkComps.KongTeamManager                `json:"team_manager,omitempty"   yaml:"team_manager,omitempty"`
	CreatedAt     time.Time                               `json:"created_at"               yaml:"created_at"`
	UpdatedAt     time.Time                               `json:"updated_at"               yaml:"updated_at"`
}

func decodeIdentityGroupResponse(response *http.Response, destination any) error {
	if response == nil || response.Body == nil {
		return fmt.Errorf("missing group HTTP response")
	}
	data, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	response.Body = io.NopCloser(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("cannot read group response")
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("cannot decode group response")
	}
	return nil
}

type identityGroupPage[T any] struct {
	Data []T `json:"data"`
	Meta *struct {
		Page *struct {
			Next json.RawMessage `json:"next"`
			Size *float64        `json:"size"`
		} `json:"page"`
	} `json:"meta"`
}

func (p identityGroupPage[T]) next() (*string, error) {
	if p.Data == nil || p.Meta == nil || p.Meta.Page == nil || p.Meta.Page.Size == nil ||
		*p.Meta.Page.Size < 1 || len(p.Meta.Page.Next) == 0 {
		return nil, fmt.Errorf("missing or invalid group collection data or pagination")
	}
	var next *string
	if err := json.Unmarshal(p.Meta.Page.Next, &next); err != nil {
		return nil, fmt.Errorf("invalid group collection cursor")
	}
	return next, nil
}

func ListIdentityGroups(ctx context.Context, api IdentityGroupsReader, directoryID string,
	pageSize int64,
) ([]IdentityGroup, error) {
	if api == nil {
		return nil, fmt.Errorf("identity group reader is not configured")
	}
	if !util.IsValidUUID(directoryID) {
		return nil, fmt.Errorf("directory ID must be a UUID")
	}
	if pageSize < 1 {
		pageSize = 100
	}
	request := kkOps.ListGroupsRequest{
		DirectoryID: directoryID, Page: &kkComps.CursorPageParameters{Size: &pageSize},
	}
	result := make([]IdentityGroup, 0)
	seen := make(map[string]bool)
	var tracker pagination.CursorTracker
	for {
		response, err := api.ListGroups(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to list identity groups: %w", err)
		}
		if response == nil {
			return nil, fmt.Errorf("missing identity group collection response")
		}
		var page identityGroupPage[IdentityGroup]
		if err := decodeIdentityGroupResponse(response.RawResponse, &page); err != nil {
			return nil, err
		}
		next, err := page.next()
		if err != nil {
			return nil, err
		}
		for _, value := range page.Data {
			if !util.IsValidUUID(value.ID) || value.Name == "" || seen[value.ID] {
				return nil, fmt.Errorf("identity group collection contains an invalid or repeated resource")
			}
			seen[value.ID] = true
			result = append(result, value)
		}
		cursor, err := tracker.Next(next)
		if err != nil {
			return nil, fmt.Errorf("cannot paginate identity groups: %w", err)
		}
		if cursor == "" {
			return result, nil
		}
		request.Page.After = &cursor
	}
}

// ResolveIdentityGroup proves directory membership through a complete list,
// rejects ambiguous matches, then reads fresh detail without trusting defaults.
func ResolveIdentityGroup(ctx context.Context, api IdentityGroupsReader, directoryID, identifier string,
	pageSize int64,
) (*IdentityGroup, error) {
	values, err := ListIdentityGroups(ctx, api, directoryID, pageSize)
	if err != nil {
		return nil, err
	}
	var match *IdentityGroup
	byID := util.IsValidUUID(identifier)
	for _, value := range values {
		if (byID && value.ID == identifier) || (!byID && value.Name == identifier) {
			if match != nil {
				return nil, fmt.Errorf("multiple identity groups match %q in the selected directory", identifier)
			}
			match = &value
		}
	}
	if match == nil {
		return nil, fmt.Errorf("identity group %q not found in the selected directory", identifier)
	}
	response, err := api.GetGroup(ctx, directoryID, match.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get identity group: %w", err)
	}
	if response == nil {
		return nil, fmt.Errorf("missing identity group response")
	}
	var value IdentityGroup
	if err := decodeIdentityGroupResponse(response.RawResponse, &value); err != nil {
		return nil, err
	}
	if value.ID != match.ID || value.Name == "" || (!byID && value.Name != identifier) {
		return nil, fmt.Errorf("identity group identity changed during lookup")
	}
	return &value, nil
}
