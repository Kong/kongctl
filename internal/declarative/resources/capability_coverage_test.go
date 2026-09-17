package resources

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManagedCapabilityCoverage(t *testing.T) {
	var managed, roots []ResourceType
	for _, collection := range SyncCollections() {
		managed = append(managed, collection.ResourceType)
		if collection.ParentType == "" {
			roots = append(roots, collection.ResourceType)
		}
	}
	require.Contains(t, roots, ResourceTypeAPI)
	require.Contains(t, roots, ResourceTypePortal)
	require.Contains(t, managed, ResourceTypeAPIVersion)
	require.NotContains(t, roots, ResourceTypeAPIVersion)

	without := func(kinds []ResourceType, omitted ...ResourceType) []ResourceType {
		return slices.DeleteFunc(slices.Clone(kinds), func(kind ResourceType) bool {
			return slices.Contains(omitted, kind)
		})
	}

	wrappers := []struct {
		name         string
		validate     func(string, []ResourceType) error
		valid        []ResourceType
		childKinds   []ResourceType
		wantChildErr string
	}{
		{
			name:         "resources",
			validate:     ValidateManagedResourceCoverage,
			valid:        managed,
			childKinds:   without(managed, ResourceTypeAPIVersion),
			wantChildErr: "consumer is missing resource types [api_version]",
		},
		{
			name:         "roots",
			validate:     ValidateManagedRootCoverage,
			valid:        roots,
			childKinds:   append(slices.Clone(roots), ResourceTypeAPIVersion),
			wantChildErr: "consumer registers unexpected resource type api_version",
		},
	}
	for _, wrapper := range wrappers {
		t.Run(wrapper.name, func(t *testing.T) {
			tests := []struct {
				name    string
				kinds   []ResourceType
				wantErr string
			}{
				{
					name:  "complete coverage",
					kinds: wrapper.valid,
				},
				{
					name:    "missing kind",
					kinds:   without(wrapper.valid, ResourceTypeAPI),
					wantErr: "consumer is missing resource types [api]",
				},
				{
					name:    "sorted missing kinds",
					kinds:   without(wrapper.valid, ResourceTypePortal, ResourceTypeAPI),
					wantErr: "consumer is missing resource types [api portal]",
				},
				{
					name:    "duplicate kind",
					kinds:   append(slices.Clone(wrapper.valid), ResourceTypeAPI),
					wantErr: "consumer registers resource type api more than once",
				},
				{
					name:    "sorted duplicate diagnostics",
					kinds:   append(slices.Clone(wrapper.valid), ResourceTypePortal, ResourceTypeAPI),
					wantErr: "consumer registers resource type api more than once",
				},
				{
					name:    "deck backed kind",
					kinds:   append(slices.Clone(wrapper.valid), ResourceTypeGatewayService),
					wantErr: "consumer registers unexpected resource type gateway_service",
				},
				{
					name: "sorted unexpected diagnostics",
					kinds: append(slices.Clone(wrapper.valid),
						ResourceTypeGatewayService, ResourceTypeAuditLogWebhookDestination),
					wantErr: "consumer registers unexpected resource type audit_log_webhook_destination",
				},
				{
					name:    "unregistered selector kind",
					kinds:   append(slices.Clone(wrapper.valid), ResourceTypeOrganizationUser),
					wantErr: "consumer registers unexpected resource type organization_user",
				},
				{
					name:    "root versus child filtering",
					kinds:   wrapper.childKinds,
					wantErr: wrapper.wantChildErr,
				},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					// Both input orders must preserve caller data and select the same diagnostic.
					for _, reverse := range []bool{false, true} {
						kinds := slices.Clone(tt.kinds)
						slices.Sort(kinds)
						if reverse {
							slices.Reverse(kinds)
						}
						before := slices.Clone(kinds)
						err := wrapper.validate("consumer", kinds)
						require.Equal(t, before, kinds, "reverse=%t", reverse)
						if tt.wantErr == "" {
							require.NoError(t, err, "reverse=%t", reverse)
						} else {
							require.EqualError(t, err, tt.wantErr, "reverse=%t", reverse)
						}
					}
				})
			}
		})
	}
}
