package konnect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/google/uuid"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

type GroupAPI struct {
	Directories                          map[string][]helpers.IdentityGroup
	Memberships                          map[string][]kkComps.KongGroupMembershipEntry
	Keys                                 map[string][]string
	Writes                               []kkComps.ReplaceGroupBody
	Creates, Deletes                     int
	MembershipCreates, MembershipDeletes int
	Failure                              string
}

func Group(name, namespace string, protected bool) helpers.IdentityGroup {
	return helpers.IdentityGroup{
		ID: uuid.NewSHA1(uuid.NameSpaceDNS, []byte(name+namespace)).String(), Name: name,
		Type: new(kkComps.KongGroupTypeGroup), ProvisionedBy: new(kkComps.ProvisionedByKonnectUser),
		Description: "Existing description", Labels: map[string]string{"team": "identity"},
		Metadata:  map[string]kkComps.KongIdentityMetadata{"count": kkComps.CreateKongIdentityMetadataInteger(42)},
		ManagedBy: resources.BuildChildManagedBy(namespace, protected, "old"),
	}
}

func groupRaw(value any) (*http.Response, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return &http.Response{Body: io.NopCloser(bytes.NewReader(data))}, nil
}

func groupPage(value any) (*http.Response, error) {
	return groupRaw(
		map[string]any{"data": value, "meta": map[string]any{"page": map[string]any{"size": 100, "next": nil}}},
	)
}

func groupSDK(g helpers.IdentityGroup) kkComps.KongGroup {
	return kkComps.KongGroup{
		ID: g.ID, Name: g.Name, Description: g.Description, Type: g.Type, Metadata: g.Metadata,
		Labels: g.Labels, ManagedBy: g.ManagedBy, ProvisionedBy: g.ProvisionedBy, TeamManager: g.TeamManager,
	}
}

func (a *GroupAPI) ListGroups(
	_ context.Context,
	request kkOps.ListGroupsRequest,
	_ ...kkOps.Option,
) (*kkOps.ListGroupsResponse, error) {
	if a.Failure == "list" {
		return nil, fmt.Errorf("incomplete group observation")
	}
	values := a.Directories[request.DirectoryID]
	if values == nil {
		values = []helpers.IdentityGroup{}
	}
	raw, err := groupPage(values)
	return &kkOps.ListGroupsResponse{RawResponse: raw}, err
}

func (a *GroupAPI) GetGroup(
	_ context.Context,
	directoryID, id string,
	_ ...kkOps.Option,
) (*kkOps.GetGroupResponse, error) {
	for _, value := range a.Directories[directoryID] {
		if value.ID == id {
			raw, err := groupRaw(value)
			return &kkOps.GetGroupResponse{RawResponse: raw}, err
		}
	}
	return nil, fmt.Errorf("group fixture not found")
}

func (a *GroupAPI) CreateGroup(_ context.Context, directoryID string, request kkComps.CreateGroupBody,
	_ ...kkOps.Option,
) (*kkOps.CreateGroupResponse, error) {
	a.Creates++
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	var group helpers.IdentityGroup
	if err := json.Unmarshal(data, &group); err != nil {
		return nil, err
	}
	group.ID = uuid.NewString()
	if group.Type == nil {
		group.Type = new(kkComps.KongGroupTypeGroup)
	}
	group.ProvisionedBy = new(kkComps.ProvisionedByKonnectUser)
	if a.Directories == nil {
		a.Directories = make(map[string][]helpers.IdentityGroup)
	}
	a.Directories[directoryID] = append(a.Directories[directoryID], group)
	value := groupSDK(group)
	return &kkOps.CreateGroupResponse{KongGroup: &value}, nil
}

func (a *GroupAPI) ReplaceGroup(_ context.Context, request kkOps.ReplaceGroupRequest,
	_ ...kkOps.Option,
) (*kkOps.ReplaceGroupResponse, error) {
	a.Writes = append(a.Writes, request.ReplaceGroupBody)
	for i := range a.Directories[request.DirectoryID] {
		group := &a.Directories[request.DirectoryID][i]
		if group.ID != request.GroupID {
			continue
		}
		id, kind, provenance := group.ID, group.Type, group.ProvisionedBy
		data, err := json.Marshal(request.ReplaceGroupBody)
		if err != nil {
			return nil, err
		}
		*group = helpers.IdentityGroup{}
		if err := json.Unmarshal(data, group); err != nil {
			return nil, err
		}
		group.ID, group.Type, group.ProvisionedBy = id, kind, provenance
		value := groupSDK(*group)
		return &kkOps.ReplaceGroupResponse{KongGroup: &value}, nil
	}
	return nil, fmt.Errorf("group fixture not found")
}

func (a *GroupAPI) DeleteGroup(
	_ context.Context,
	directoryID, id string,
	_ ...kkOps.Option,
) (*kkOps.DeleteGroupResponse, error) {
	a.Deletes++
	for i, value := range a.Directories[directoryID] {
		if value.ID == id {
			a.Directories[directoryID] = append(a.Directories[directoryID][:i], a.Directories[directoryID][i+1:]...)
			break
		}
	}
	return &kkOps.DeleteGroupResponse{}, nil
}

func (a *GroupAPI) ListGroupMembership(_ context.Context, request kkOps.ListGroupMembershipRequest,
	_ ...kkOps.Option,
) (*kkOps.ListGroupMembershipResponse, error) {
	if a.Failure == "membership" {
		return nil, fmt.Errorf("incomplete membership observation")
	}
	values := a.Memberships[request.GroupID]
	if values == nil {
		values = []kkComps.KongGroupMembershipEntry{}
	}
	raw, err := groupPage(values)
	return &kkOps.ListGroupMembershipResponse{RawResponse: raw}, err
}

func (a *GroupAPI) GetGroupMembershipEntry(_ context.Context, request kkOps.GetGroupMembershipEntryRequest,
	_ ...kkOps.Option,
) (*kkOps.GetGroupMembershipEntryResponse, error) {
	for _, value := range a.Memberships[request.GroupID] {
		if value.ID == request.GroupMembershipID {
			return &kkOps.GetGroupMembershipEntryResponse{KongGroupMembershipEntry: &value}, nil
		}
	}
	return nil, fmt.Errorf("membership fixture not found")
}

func (a *GroupAPI) ListGroupKeys(_ context.Context, request kkOps.ListGroupKeysRequest,
	_ ...kkOps.Option,
) (*kkOps.ListGroupKeysResponse, error) {
	if a.Failure == "keys" {
		return nil, fmt.Errorf("credential observation failed")
	}
	values := a.Keys[request.GroupID]
	if values == nil {
		values = []string{}
	}
	raw, err := groupPage(values)
	return &kkOps.ListGroupKeysResponse{RawResponse: raw}, err
}

var _ helpers.IdentityGroupsAPI = (*GroupAPI)(nil)

func (a *GroupAPI) CreateGroupMembershipEntry(_ context.Context, request kkOps.CreateGroupMembershipEntryRequest,
	_ ...kkOps.Option,
) (*kkOps.CreateGroupMembershipEntryResponse, error) {
	a.MembershipCreates++
	if a.Memberships == nil {
		a.Memberships = make(map[string][]kkComps.KongGroupMembershipEntry)
	}
	body := request.CreateGroupMembershipEntryBody.Member
	for _, value := range a.Memberships[request.GroupID] {
		if value.Member.ID == body.ID && value.Member.Type == body.Type {
			return nil, fmt.Errorf("duplicate member")
		}
	}
	value := kkComps.KongGroupMembershipEntry{
		ID:     uuid.NewString(),
		Member: kkComps.KongGroupMember{ID: body.ID, Type: body.Type},
	}
	a.Memberships[request.GroupID] = append(a.Memberships[request.GroupID], value)
	return &kkOps.CreateGroupMembershipEntryResponse{KongGroupMembershipEntry: &value}, nil
}

func (a *GroupAPI) DeleteGroupMembershipEntry(_ context.Context, request kkOps.DeleteGroupMembershipEntryRequest,
	_ ...kkOps.Option,
) (*kkOps.DeleteGroupMembershipEntryResponse, error) {
	a.MembershipDeletes++
	for i, value := range a.Memberships[request.GroupID] {
		if value.ID == request.GroupMembershipID {
			a.Memberships[request.GroupID] = append(
				a.Memberships[request.GroupID][:i],
				a.Memberships[request.GroupID][i+1:]...,
			)
			break
		}
	}
	return &kkOps.DeleteGroupMembershipEntryResponse{}, nil
}
