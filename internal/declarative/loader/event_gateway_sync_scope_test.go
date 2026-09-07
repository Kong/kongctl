package loader

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoaderEventGatewayPolicySyncScope(t *testing.T) {
	families := []struct {
		name          string
		parentType    resources.ResourceType
		policyType    resources.ResourceType
		nestedParents string
		rootParents   string
		nestedPolicy  string
		rootPolicies  string
		parentKey     string
		policyFields  string
	}{
		{
			name:          "cluster",
			parentType:    resources.ResourceTypeEventGatewayVirtualCluster,
			policyType:    resources.ResourceTypeEventGatewayClusterPolicy,
			nestedParents: "virtual_clusters",
			rootParents:   "event_gateway_virtual_clusters",
			nestedPolicy:  "cluster_policies",
			rootPolicies:  "event_gateway_virtual_cluster_cluster_policies",
			parentKey:     "virtual_cluster",
			policyFields: "type: acls, config: {rules: [{action: deny, " +
				"operations: [{name: read}], resource_names: [{match: '*'}], resource_type: topic}]}",
		},
		{
			name:          "produce",
			parentType:    resources.ResourceTypeEventGatewayVirtualCluster,
			policyType:    resources.ResourceTypeEventGatewayProducePolicy,
			nestedParents: "virtual_clusters",
			rootParents:   "event_gateway_virtual_clusters",
			nestedPolicy:  "produce_policies",
			rootPolicies:  "event_gateway_virtual_cluster_produce_policies",
			parentKey:     "virtual_cluster",
			policyFields:  "type: modify_headers, config: {actions: [{op: remove, key: x-test}]}",
		},
		{
			name:          "consume",
			parentType:    resources.ResourceTypeEventGatewayVirtualCluster,
			policyType:    resources.ResourceTypeEventGatewayConsumePolicy,
			nestedParents: "virtual_clusters",
			rootParents:   "event_gateway_virtual_clusters",
			nestedPolicy:  "consume_policies",
			rootPolicies:  "event_gateway_virtual_cluster_consume_policies",
			parentKey:     "virtual_cluster",
			policyFields:  "type: modify_headers, config: {actions: [{op: remove, key: x-test}]}",
		},
		{
			name:          "listener",
			parentType:    resources.ResourceTypeEventGatewayListener,
			policyType:    resources.ResourceTypeEventGatewayListenerPolicy,
			nestedParents: "listeners",
			rootParents:   "event_gateway_listeners",
			nestedPolicy:  "policies",
			rootPolicies:  "event_gateway_listener_policies",
			parentKey:     "listener",
			policyFields:  "type: tls_server, config: {certificates: [], allow_plaintext: true}",
		},
	}

	for _, family := range families {
		for _, placement := range []struct {
			name       string
			rootParent bool
			rootPolicy bool
		}{
			{name: "gateway-nested parent"},
			// Preserve the existing exclusion: nested policies on root-declared
			// parents do not acquire sync scope, even when populated.
			{name: "root-declared parent", rootParent: true},
			{name: "root-declared policy", rootParent: true, rootPolicy: true},
		} {
			for _, state := range []string{"omitted", "empty", "populated"} {
				t.Run(family.name+"/"+placement.name+"/"+state, func(t *testing.T) {
					policy := "ref: policy, " + family.policyFields
					if placement.rootPolicy {
						policy += ", event_gateway: gateway, " + family.parentKey + ": owner"
					}
					collection := "[]"
					if state == "populated" {
						collection = "[{" + policy + "}]"
					}
					parent := "ref: owner, name: Owner"
					if placement.rootParent {
						parent += ", event_gateway: gateway"
					}
					if !placement.rootPolicy && state != "omitted" {
						parent += ", " + family.nestedPolicy + ": " + collection
					}
					input := "event_gateways:\n  - ref: gateway\n    name: Gateway\n"
					if placement.rootParent {
						input += fmt.Sprintf("%s: [{%s}]\n", family.rootParents, parent)
					} else {
						input += fmt.Sprintf("    %s: [{%s}]\n", family.nestedParents, parent)
					}
					if placement.rootPolicy && state != "omitted" {
						input += family.rootPolicies + ": " + collection + "\n"
					}

					rs, err := New().parseYAML(strings.NewReader(input), "test.yaml", ".")
					require.NoError(t, err)
					require.NotNil(t, rs.SyncScope)

					// Exact scopes cover both the two-segment parent path and
					// three-segment policy path, with no scope on another owner.
					want := []resources.ChildSyncScope{{
						ParentType:   resources.ResourceTypeEventGatewayControlPlane,
						ParentRef:    "gateway",
						ResourceType: family.parentType,
					}}
					if (!placement.rootParent && state != "omitted") ||
						(placement.rootPolicy && state == "populated") {
						want = append(want, resources.ChildSyncScope{
							ParentType:   family.parentType,
							ParentRef:    "owner",
							ResourceType: family.policyType,
						})
					}
					assert.ElementsMatch(t, want, rs.SyncScope.ChildScopes())
					if placement.rootPolicy && state == "empty" {
						// Empty root policies defer rejection to planning; they
						// cannot identify a parent to manage.
						assert.Equal(
							t,
							[]resources.ResourceType{family.policyType},
							rs.SyncScope.RootChildCollectionTypes(),
						)
					} else {
						assert.Empty(t, rs.SyncScope.RootChildCollectionTypes())
					}
				})
			}
		}
	}
}
