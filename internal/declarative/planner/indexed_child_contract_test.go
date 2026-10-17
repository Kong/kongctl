package planner

import (
	"context"
	"errors"
	"fmt"
	"testing"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	kkErrors "github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/stretchr/testify/require"
)

func TestIndexedPortalRetention(t *testing.T) {
	for _, kind := range []string{ResourceTypePortalSnippet, ResourceTypePortalTeam, ResourceTypePortalEmailTemplate} {
		for _, mode := range []PlanMode{PlanModeApply, PlanModeSync} {
			for _, alreadyPlanned := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/planned=%t", kind, mode, alreadyPlanned), func(t *testing.T) {
					snippets := &indexedSnippetAPI{}
					emails := &indexedEmailAPI{stubExternalPortalEmailsAPI: stubExternalPortalEmailsAPI{
						listPortalCustomEmailTemplatesFn: func() []kkComps.EmailTemplate {
							return []kkComps.EmailTemplate{{Name: "kept", Enabled: true}, {Name: "stale"}}
						},
					}}
					p := NewPlanner(
						state.NewClient(state.ClientConfig{PortalSnippetAPI: snippets, PortalEmailsAPI: emails}),
						discardPlannerLogger(),
					)
					p.matchedIdentities = make(map[string]matchedResourceIdentity)
					p.resourceCache.portalTeamsByPortalID["portal-id"] = []state.PortalTeam{
						{ID: "kept-id", Name: "kept"},
						{ID: "stale-id", Name: "stale"},
					}
					plan := NewPlan(CurrentPlanVersion, "test", mode)
					if alreadyPlanned {
						plan.AddChange(
							PlannedChange{
								ID:           "previous",
								ResourceType: kind,
								ResourceRef:  "kept-ref",
								Action:       ActionUpdate,
							},
						)
					}
					start := len(plan.Changes)
					var err error
					switch kind {
					case ResourceTypePortalSnippet:
						err = p.planPortalSnippetsChanges(t.Context(), "default", "portal-id", "portal-ref",
							[]resources.PortalSnippetResource{{Ref: "kept-ref", Name: "kept", Content: "same"}}, plan)
					case ResourceTypePortalTeam:
						err = p.planPortalTeamsChanges(
							t.Context(),
							"default",
							"portal-id",
							"portal-ref",
							[]resources.PortalTeamResource{
								{
									Ref:                     "kept-ref",
									PortalCreateTeamRequest: kkComps.PortalCreateTeamRequest{Name: "kept"},
								},
							},
							plan,
						)
					case ResourceTypePortalEmailTemplate:
						err = p.planPortalEmailTemplatesChanges(
							t.Context(),
							"default",
							"portal-id",
							"portal-ref",
							"Portal",
							[]resources.PortalEmailTemplateResource{
								{Ref: "kept-ref", Name: "kept", Enabled: new(true)},
							},
							plan,
						)
					}
					require.NoError(t, err)
					var deleted []string
					for _, change := range plan.Changes[start:] {
						require.Equal(t, ActionDelete, change.Action)
						deleted = append(deleted, change.Fields[FieldName].(string))
					}
					var want []string
					if mode == PlanModeSync {
						want = append(want, "stale")
						if alreadyPlanned && kind != ResourceTypePortalEmailTemplate {
							want = append(want, "kept")
						}
					}
					require.ElementsMatch(t, want, deleted)
					reads := 0
					if !alreadyPlanned && kind != ResourceTypePortalTeam {
						reads = 1
					}
					require.Equal(t, reads, snippets.reads+emails.reads)
					require.Empty(t, p.matchedIdentities, "indexed callers must not acquire automatic payload binding")
				})
			}
		}
	}
}

func TestIndexedSnippetFailurePreservesPartialPlan(t *testing.T) {
	failure := errors.New("detail unavailable")
	api := &indexedSnippetAPI{detailError: failure}
	p := NewPlanner(state.NewClient(state.ClientConfig{PortalSnippetAPI: api}), discardPlannerLogger())
	plan := NewPlan(CurrentPlanVersion, "test", PlanModeSync)
	err := p.planPortalSnippetsChanges(
		t.Context(),
		"default",
		"portal-id",
		"portal-ref",
		[]resources.PortalSnippetResource{
			{Ref: "new-ref", Name: "new"},
			{Ref: "kept-ref", Name: "kept"},
			{Ref: "later-ref", Name: "later"},
		},
		plan,
	)
	require.ErrorContains(t, err, "failed to fetch portal snippet kept-id for comparison")
	require.ErrorIs(t, err, failure)
	require.Len(t, plan.Changes, 1, "failure stops later creates and sync pruning")
	require.Equal(t, ActionCreate, plan.Changes[0].Action)
	require.Equal(t, "new-ref", plan.Changes[0].ResourceRef)
}

func TestIndexedMappedBindingAfterSuccess(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprintf("fail=%t", fail), func(t *testing.T) {
			p := &Planner{matchedIdentities: make(map[string]matchedResourceIdentity)}
			failure := errors.New("match failed")
			desired := resources.EventGatewayStaticKeyResource{
				Ref:                         "key-ref",
				EventGatewayStaticKeyCreate: kkComps.EventGatewayStaticKeyCreate{Name: "kept"},
			}
			removed := false
			err := reconcileMappedChildren(
				p,
				[]resources.EventGatewayStaticKeyResource{desired},
				map[string]string{"kept": "id", "stale": "stale-id"},
				mappedChildOperations[resources.EventGatewayStaticKeyResource, string]{
					desiredName: func(d resources.EventGatewayStaticKeyResource) string { return d.Name },
					create:      func(resources.EventGatewayStaticKeyResource) error { t.Fatal("unexpected create"); return nil },
					matched: func(resources.EventGatewayStaticKeyResource, string) error {
						require.Empty(t, p.matchedIdentities, "binding must follow successful match")
						if fail {
							return failure
						}
						return nil
					},
					remove: func(string, string) { removed = true },
				},
				PlanModeSync,
			)
			if fail {
				require.ErrorIs(t, err, failure)
				require.Empty(t, p.matchedIdentities)
				require.False(t, removed)
			} else {
				require.NoError(t, err)
				want := matchedResourceIdentity{resources.ResourceTypeEventGatewayStaticKey, "id"}
				require.Equal(t, want, p.matchedIdentities["key-ref"])
				require.True(t, removed)
			}
		})
	}
}

func TestIndexedEventCertificateIndex(t *testing.T) {
	for _, mode := range []PlanMode{PlanModeApply, PlanModeSync} {
		t.Run(string(mode), func(t *testing.T) {
			p := NewPlanner(
				state.NewClient(state.ClientConfig{EventGatewayDataPlaneCertificateAPI: &indexedCertificateAPI{}}),
				discardPlannerLogger(),
			)
			p.matchedIdentities = make(map[string]matchedResourceIdentity)
			plan := NewPlan(CurrentPlanVersion, "test", mode)
			desired := []resources.EventGatewayDataPlaneCertificateResource{}
			for _, name := range []string{"new", "kept"} {
				desired = append(desired, resources.EventGatewayDataPlaneCertificateResource{
					Ref: name + "-ref",
					CreateEventGatewayDataPlaneCertificateRequest: kkComps.CreateEventGatewayDataPlaneCertificateRequest{
						Name:        new(name),
						Certificate: "updated",
					},
				})
			}
			require.NoError(
				t,
				p.planDataPlaneCertificateChangesForExistingGateway(
					t.Context(),
					"default",
					"gateway-id",
					"gateway-ref",
					"Gateway",
					desired,
					plan,
				),
			)
			count := 2
			if mode == PlanModeSync {
				count = 3
			}
			require.Len(t, plan.Changes, count)
			require.Equal(t, ActionCreate, plan.Changes[0].Action)
			require.Equal(t, "new-ref", plan.Changes[0].ResourceRef)
			require.Equal(t, ActionUpdate, plan.Changes[1].Action)
			require.Equal(t, "last-id", plan.Changes[1].ResourceID)
			if mode == PlanModeSync {
				require.Equal(t, ActionDelete, plan.Changes[2].Action)
				require.Equal(t, "stale-id", plan.Changes[2].ResourceID)
			}
			require.Empty(t, p.matchedIdentities)
		})
	}
}

type indexedSnippetAPI struct {
	mockPortalSnippetAPI
	reads       int
	detailError error
}

func (a *indexedSnippetAPI) ListPortalSnippets(
	context.Context,
	kkOps.ListPortalSnippetsRequest,
	...kkOps.Option,
) (*kkOps.ListPortalSnippetsResponse, error) {
	return &kkOps.ListPortalSnippetsResponse{ListPortalSnippetsResponse: &kkComps.ListPortalSnippetsResponse{
		Data: []kkComps.PortalSnippetInfo{{ID: "kept-id", Name: "kept"}, {ID: "stale-id", Name: "stale"}},
	}}, nil
}

func (a *indexedSnippetAPI) GetPortalSnippet(
	context.Context,
	string,
	string,
	...kkOps.Option,
) (*kkOps.GetPortalSnippetResponse, error) {
	a.reads++
	if a.detailError != nil {
		return nil, a.detailError
	}
	return &kkOps.GetPortalSnippetResponse{
		PortalSnippetResponse: &kkComps.PortalSnippetResponse{ID: "kept-id", Name: "kept", Content: "same"},
	}, nil
}

type indexedEmailAPI struct {
	stubExternalPortalEmailsAPI
	reads int
}

func (a *indexedEmailAPI) GetPortalCustomEmailTemplate(
	context.Context,
	string,
	kkComps.EmailTemplateName,
	...kkOps.Option,
) (*kkOps.GetPortalCustomEmailTemplateResponse, error) {
	a.reads++
	// A missing detail response must fall back to the listed enabled value.
	return nil, &kkErrors.NotFoundError{}
}

type indexedCertificateAPI struct {
	helpers.EventGatewayDataPlaneCertificateAPI
}

func (*indexedCertificateAPI) ListEventGatewayDataPlaneCertificates(
	context.Context,
	kkOps.ListEventGatewayDataPlaneCertificatesRequest,
	...kkOps.Option,
) (*kkOps.ListEventGatewayDataPlaneCertificatesResponse, error) {
	return &kkOps.ListEventGatewayDataPlaneCertificatesResponse{
		ListEventGatewayDataPlaneCertificatesResponse: &kkComps.ListEventGatewayDataPlaneCertificatesResponse{
			Data: []kkComps.EventGatewayDataPlaneCertificate{
				{ID: "first-id", Name: new("kept"), Certificate: "updated"},
				{ID: "last-id", Name: new("kept"), Certificate: "old"},
				{ID: "stale-id", Name: new("stale")},
				{ID: "unnamed-id"},
			},
		},
	}, nil
}
