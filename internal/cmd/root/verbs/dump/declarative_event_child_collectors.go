package dump

import (
	"context"

	declresources "github.com/kong/kongctl/internal/declarative/resources"
)

type eventGatewayChildCollector = gatewayChildCollector[declresources.EventGatewayControlPlaneResource]

var eventGatewayChildCollectors = buildEventGatewayChildCollectors()

func buildEventGatewayChildCollectors() []eventGatewayChildCollector {
	// Nested policy reads stay inside their owning builder and retain their error policy.
	collectors := []eventGatewayChildCollector{
		gatewayChild(
			"failed to load event gateway backend clusters",
			func(
				ctx context.Context, d *gatewayChildDumpContext,
			) ([]declresources.EventGatewayBackendClusterResource, error) {
				return buildEventGatewayBackendClusters(ctx, d.logger, d.client, d.gatewayID, d.gatewayName)
			},
			func(g *declresources.EventGatewayControlPlaneResource) *[]declresources.EventGatewayBackendClusterResource {
				return &g.BackendClusters
			},
		),
		gatewayChild(
			"failed to load event gateway virtual clusters",
			func(
				ctx context.Context, d *gatewayChildDumpContext,
			) ([]declresources.EventGatewayVirtualClusterResource, error) {
				return buildEventGatewayVirtualClusters(ctx, d.logger, d.client, d.gatewayID, d.gatewayName)
			},
			func(g *declresources.EventGatewayControlPlaneResource) *[]declresources.EventGatewayVirtualClusterResource {
				return &g.VirtualClusters
			},
			declresources.ResourceTypeEventGatewayClusterPolicy,
			declresources.ResourceTypeEventGatewayProducePolicy,
			declresources.ResourceTypeEventGatewayConsumePolicy,
		),
		gatewayChild(
			"failed to load event gateway listeners",
			func(
				ctx context.Context, d *gatewayChildDumpContext,
			) ([]declresources.EventGatewayListenerResource, error) {
				return buildEventGatewayListeners(ctx, d.logger, d.client, d.gatewayID)
			},
			func(g *declresources.EventGatewayControlPlaneResource) *[]declresources.EventGatewayListenerResource {
				return &g.Listeners
			},
			declresources.ResourceTypeEventGatewayListenerPolicy,
		),
		gatewayChild(
			"failed to load event gateway data plane certificates",
			func(
				ctx context.Context, d *gatewayChildDumpContext,
			) ([]declresources.EventGatewayDataPlaneCertificateResource, error) {
				return buildEventGatewayDataPlaneCertificates(ctx, d.logger, d.client, d.gatewayID, d.gatewayName)
			},
			func(g *declresources.EventGatewayControlPlaneResource) *[]declresources.EventGatewayDataPlaneCertificateResource {
				return &g.DataPlaneCertificates
			},
		),
		gatewayChild(
			"failed to load event gateway schema registries",
			func(
				ctx context.Context, d *gatewayChildDumpContext,
			) ([]declresources.EventGatewaySchemaRegistryResource, error) {
				return buildEventGatewaySchemaRegistries(ctx, d.logger, d.client, d.gatewayID, d.gatewayName)
			},
			func(g *declresources.EventGatewayControlPlaneResource) *[]declresources.EventGatewaySchemaRegistryResource {
				return &g.SchemaRegistries
			},
		),
		gatewayChild(
			"failed to load event gateway static keys",
			func(
				ctx context.Context, d *gatewayChildDumpContext,
			) ([]declresources.EventGatewayStaticKeyResource, error) {
				return buildEventGatewayStaticKeys(ctx, d.logger, d.client, d.gatewayID, d.gatewayName)
			},
			func(g *declresources.EventGatewayControlPlaneResource) *[]declresources.EventGatewayStaticKeyResource {
				return &g.StaticKeys
			},
		),
		gatewayChild(
			"failed to load event gateway TLS trust bundles",
			func(
				ctx context.Context, d *gatewayChildDumpContext,
			) ([]declresources.EventGatewayTLSTrustBundleResource, error) {
				return buildEventGatewayTLSTrustBundles(ctx, d.logger, d.client, d.gatewayID, d.gatewayName)
			},
			func(g *declresources.EventGatewayControlPlaneResource) *[]declresources.EventGatewayTLSTrustBundleResource {
				return &g.TrustBundles
			},
		),
	}
	if err := validateGatewayChildCollectors("Event Gateway child dump", collectors); err != nil {
		panic(err)
	}
	return collectors
}
