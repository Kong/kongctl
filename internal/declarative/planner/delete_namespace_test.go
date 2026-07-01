package planner

import (
	"io"
	"log/slog"
	"testing"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteBuildersSetNamespace(t *testing.T) {
	t.Parallel()

	const namespace = "team-a"

	testCases := []struct {
		name         string
		resourceType string
		planDelete   func(*Planner, *Plan)
	}{
		{
			name:         "api version",
			resourceType: ResourceTypeAPIVersion,
			planDelete: func(p *Planner, plan *Plan) {
				p.planAPIVersionDelete(namespace, "api", "api-id", "version-id", "v1", plan)
			},
		},
		{
			name:         "api publication",
			resourceType: ResourceTypeAPIPublication,
			planDelete: func(p *Planner, plan *Plan) {
				p.planAPIPublicationDelete(namespace, "api", "api-id", "portal-id", "portal", state.APIPublication{}, plan)
			},
		},
		{
			name:         "api document",
			resourceType: ResourceTypeAPIDocument,
			planDelete: func(p *Planner, plan *Plan) {
				p.planAPIDocumentDelete(namespace, "api", "api-id", "document-id", "overview", plan)
			},
		},
		{
			name:         "control plane data plane certificate",
			resourceType: ResourceTypeControlPlaneDataPlaneCertificate,
			planDelete: func(p *Planner, plan *Plan) {
				p.planControlPlaneDataPlaneCertificateDelete(namespace, "cp", "cp-id", "cert-id", "cert-fp", nil, plan)
			},
		},
		{
			name:         "event gateway backend cluster",
			resourceType: ResourceTypeEventGatewayBackendCluster,
			planDelete: func(p *Planner, plan *Plan) {
				p.planBackendClusterDelete(namespace, "gateway", "Gateway", "gateway-id", "cluster-id", "cluster", plan)
			},
		},
		{
			name:         "event gateway cluster policy",
			resourceType: ResourceTypeEventGatewayClusterPolicy,
			planDelete: func(p *Planner, plan *Plan) {
				p.planClusterPolicyDelete(namespace, "gateway-id", "gateway", "cluster-id", "cluster", "policy-id", "policy", plan)
			},
		},
		{
			name:         "event gateway consume policy",
			resourceType: ResourceTypeEventGatewayConsumePolicy,
			planDelete: func(p *Planner, plan *Plan) {
				p.planConsumePolicyDelete(namespace, "gateway-id", "gateway", "cluster-id", "cluster", "policy-id", "policy", plan)
			},
		},
		{
			name:         "event gateway data plane certificate",
			resourceType: ResourceTypeEventGatewayDataPlaneCertificate,
			planDelete: func(p *Planner, plan *Plan) {
				p.planDataPlaneCertificateDelete(namespace, "gateway", "Gateway", "gateway-id", "cert-id", "cert", plan)
			},
		},
		{
			name:         "event gateway listener",
			resourceType: ResourceTypeEventGatewayListener,
			planDelete: func(p *Planner, plan *Plan) {
				p.planListenerDelete(namespace, "gateway", "Gateway", "gateway-id", "listener-id", "listener", plan)
			},
		},
		{
			name:         "event gateway listener policy",
			resourceType: ResourceTypeEventGatewayListenerPolicy,
			planDelete: func(p *Planner, plan *Plan) {
				p.planListenerPolicyDelete(
					namespace, "gateway-id", "gateway", "listener-id", "listener", "policy-id", "policy", plan,
				)
			},
		},
		{
			name:         "event gateway produce policy",
			resourceType: ResourceTypeEventGatewayProducePolicy,
			planDelete: func(p *Planner, plan *Plan) {
				p.planProducePolicyDelete(namespace, "gateway-id", "gateway", "cluster-id", "cluster", "policy-id", "policy", plan)
			},
		},
		{
			name:         "event gateway schema registry",
			resourceType: ResourceTypeEventGatewaySchemaRegistry,
			planDelete: func(p *Planner, plan *Plan) {
				p.planSchemaRegistryDelete(namespace, "gateway", "gateway-id", "registry-id", "registry", plan)
			},
		},
		{
			name:         "event gateway static key",
			resourceType: ResourceTypeEventGatewayStaticKey,
			planDelete: func(p *Planner, plan *Plan) {
				p.planStaticKeyDelete(namespace, "gateway", "Gateway", "gateway-id", "key-id", "key", plan)
			},
		},
		{
			name:         "event gateway TLS trust bundle",
			resourceType: ResourceTypeEventGatewayTLSTrustBundle,
			planDelete: func(p *Planner, plan *Plan) {
				p.planTrustBundleDelete(namespace, "gateway", "gateway-id", "bundle-id", "bundle", plan)
			},
		},
		{
			name:         "event gateway virtual cluster",
			resourceType: ResourceTypeEventGatewayVirtualCluster,
			planDelete: func(p *Planner, plan *Plan) {
				p.planVirtualClusterDelete(namespace, "gateway", "Gateway", "gateway-id", "cluster-id", "cluster", plan)
			},
		},
		{
			name:         "portal page",
			resourceType: ResourceTypePortalPage,
			planDelete: func(p *Planner, plan *Plan) {
				p.planPortalPageDelete(namespace, "portal", "portal-id", "page-id", "overview", plan)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			planner := NewPlanner(nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
			planner.resources = &resources.ResourceSet{}
			plan := NewPlan("1.0", "test", PlanModeSync)

			tc.planDelete(planner, plan)

			require.Len(t, plan.Changes, 1)
			assert.Equal(t, tc.resourceType, plan.Changes[0].ResourceType)
			assert.Equal(t, ActionDelete, plan.Changes[0].Action)
			assert.Equal(t, namespace, plan.Changes[0].Namespace)
		})
	}
}
