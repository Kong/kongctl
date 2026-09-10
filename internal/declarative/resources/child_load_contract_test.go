package resources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPortalChildLoadRegistrations(t *testing.T) {
	want := []ResourceType{
		ResourceTypePortalPage,
		ResourceTypePortalSnippet,
		ResourceTypePortalCustomization,
		ResourceTypePortalAuthSettings,
		ResourceTypePortalIPAllowList,
		ResourceTypePortalIntegration,
		ResourceTypePortalIdentityProvider,
		ResourceTypePortalTeamGroupMapping,
		ResourceTypePortalCustomDomain,
		ResourceTypePortalEmailConfig,
		ResourceTypePortalAuditLogWebhook,
		ResourceTypePortalEmailTemplate,
	}

	got := make([]ResourceType, 0, len(childValidators[ResourceTypePortal]))
	for _, registration := range childValidators[ResourceTypePortal] {
		got = append(got, registration.kind)
	}

	// Pins both the registered set and the diagnostic order.
	require.Equal(t, want, got)
}

func TestChildLoadRejectsConflictingExtraction(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*childLoad[PortalPageResource, PortalResource])
	}{
		{
			name: "nested",
			configure: func(load *childLoad[PortalPageResource, PortalResource]) {
				load.nested = func(p *PortalResource) *[]PortalPageResource { return &p.Pages }
			},
		},
		{
			name: "setParent",
			configure: func(load *childLoad[PortalPageResource, PortalResource]) {
				load.setParent = func(r *PortalPageResource, ref string) { r.Portal = ref }
			},
		},
		{
			name: "beforeAppend",
			configure: func(load *childLoad[PortalPageResource, PortalResource]) {
				load.beforeAppend = func(*ResourceSet, *PortalPageResource) {}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			load := childLoad[PortalPageResource, PortalResource]{
				family:        ResourceTypePortal,
				extractOrder:  70,
				validateOrder: 10,
				extract:       extractPortalPageChildren,
				validate:      validatePortalChildRefs[PortalPageResource],
			}
			tt.configure(&load)

			require.PanicsWithValue(
				t,
				"register resource type "+string(ResourceTypePortalPage)+
					": custom extraction cannot also supply slice extraction",
				func() {
					registerChildResourceType(
						ResourceTypePortalPage,
						func(rs *ResourceSet) *[]PortalPageResource { return &rs.PortalPages },
						AutoExplain[PortalPageResource](),
						load,
					)
				},
			)
		})
	}
}

func TestChildLoadRejectsInvalidValidationDisposition(t *testing.T) {
	const omittedMessage = "child loader without validation requires an omission reason and no validation order: "
	const validatedMessage = "child validation requires a positive order and no omission reason: "
	validate := func(*ResourceSet) error { return nil }

	tests := []struct {
		name          string
		validate      func(*ResourceSet) error
		validateOrder int
		omittedReason string
		wantPanic     string
	}{
		{"missing omission reason", nil, 0, "", omittedMessage},
		{"omitted with positive order", nil, 10, "intentionally omitted", omittedMessage},
		{"omitted with negative order", nil, -1, "intentionally omitted", omittedMessage},
		{"validator with zero order", validate, 0, "", validatedMessage},
		{"validator with negative order", validate, -1, "", validatedMessage},
		{"validator with omission reason", validate, 10, "intentionally omitted", validatedMessage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registration := childLoadRegistration{
				parent:                  ResourceTypePortal,
				family:                  ResourceTypePortal,
				extractOrder:            70,
				extract:                 func(*ResourceSet, Resource) {},
				validate:                tt.validate,
				validateOrder:           tt.validateOrder,
				validationOmittedReason: tt.omittedReason,
			}
			require.PanicsWithValue(t, tt.wantPanic+string(ResourceTypePortalPage), func() {
				registerChildLoader(ResourceTypePortalPage, registration)
			})
		})
	}
}

func TestValidateRegisteredResourceWithoutValidation(t *testing.T) {
	for _, kind := range []ResourceType{ResourceTypePortalTeam, ResourceTypePortalTeamRole} {
		t.Run(string(kind), func(t *testing.T) {
			rs := &ResourceSet{}
			require.EqualError(t, rs.ValidateRegisteredResource(kind),
				"resource type "+string(kind)+" has no registered load validation")
		})
	}
}
