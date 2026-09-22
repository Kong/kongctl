package dump

import (
	"context"

	declresources "github.com/kong/kongctl/internal/declarative/resources"
)

type eventGatewayChildCollector = childCollector[declresources.EventGatewayControlPlaneResource]

var eventGatewayChildCollectors = buildEventGatewayChildCollectors()

func buildEventGatewayChildCollectors() []eventGatewayChildCollector {
	// Nested policy reads stay inside their owning builder and retain their error policy.
	collectors := []eventGatewayChildCollector{
		childCollection(
			"failed to load event gateway backend clusters",
			func(
				ctx context.Context, d *childDumpContext,
			) ([]declresources.EventGatewayBackendClusterResource, error) {
				return buildEventGatewayBackendClusters(ctx, d.logger, d.client, d.parentID, d.parentName)
			},
			func(g *declresources.EventGatewayControlPlaneResource) *[]declresources.EventGatewayBackendClusterResource {
				return &g.BackendClusters
			},
		),
		childCollection(
			"failed to load event gateway virtual clusters",
			func(
				ctx context.Context, d *childDumpContext,
			) ([]declresources.EventGatewayVirtualClusterResource, error) {
				return buildEventGatewayVirtualClusters(ctx, d.logger, d.client, d.parentID, d.parentName)
			},
			func(g *declresources.EventGatewayControlPlaneResource) *[]declresources.EventGatewayVirtualClusterResource {
				return &g.VirtualClusters
			},
			declresources.ResourceTypeEventGatewayClusterPolicy,
			declresources.ResourceTypeEventGatewayProducePolicy,
			declresources.ResourceTypeEventGatewayConsumePolicy,
		),
		childCollection(
			"failed to load event gateway listeners",
			func(
				ctx context.Context, d *childDumpContext,
			) ([]declresources.EventGatewayListenerResource, error) {
				return buildEventGatewayListeners(ctx, d.logger, d.client, d.parentID)
			},
			func(g *declresources.EventGatewayControlPlaneResource) *[]declresources.EventGatewayListenerResource {
				return &g.Listeners
			},
			declresources.ResourceTypeEventGatewayListenerPolicy,
		),
		childCollection(
			"failed to load event gateway data plane certificates",
			func(
				ctx context.Context, d *childDumpContext,
			) ([]declresources.EventGatewayDataPlaneCertificateResource, error) {
				return buildEventGatewayDataPlaneCertificates(ctx, d.logger, d.client, d.parentID, d.parentName)
			},
			func(g *declresources.EventGatewayControlPlaneResource) *[]declresources.EventGatewayDataPlaneCertificateResource {
				return &g.DataPlaneCertificates
			},
		),
		childCollection(
			"failed to load event gateway schema registries",
			func(
				ctx context.Context, d *childDumpContext,
			) ([]declresources.EventGatewaySchemaRegistryResource, error) {
				return buildEventGatewaySchemaRegistries(ctx, d.logger, d.client, d.parentID, d.parentName)
			},
			func(g *declresources.EventGatewayControlPlaneResource) *[]declresources.EventGatewaySchemaRegistryResource {
				return &g.SchemaRegistries
			},
		),
		childCollection(
			"failed to load event gateway static keys",
			func(
				ctx context.Context, d *childDumpContext,
			) ([]declresources.EventGatewayStaticKeyResource, error) {
				return buildEventGatewayStaticKeys(ctx, d.logger, d.client, d.parentID, d.parentName)
			},
			func(g *declresources.EventGatewayControlPlaneResource) *[]declresources.EventGatewayStaticKeyResource {
				return &g.StaticKeys
			},
		),
		childCollection(
			"failed to load event gateway TLS trust bundles",
			func(
				ctx context.Context, d *childDumpContext,
			) ([]declresources.EventGatewayTLSTrustBundleResource, error) {
				return buildEventGatewayTLSTrustBundles(ctx, d.logger, d.client, d.parentID, d.parentName)
			},
			func(g *declresources.EventGatewayControlPlaneResource) *[]declresources.EventGatewayTLSTrustBundleResource {
				return &g.TrustBundles
			},
		),
	}
	if err := validateChildCollectors("Event Gateway child dump", collectors); err != nil {
		panic(err)
	}
	return collectors
}
