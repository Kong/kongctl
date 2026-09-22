package dump

import (
	"context"

	declresources "github.com/kong/kongctl/internal/declarative/resources"
)

type portalChildCollector struct {
	childCollector[declresources.PortalResource]
	fatal bool
}

var portalChildOmissions = []childExportOmission{{
	kind:   declresources.ResourceTypePortalTeamGroupMapping,
	reason: "Portal team group mappings do not yet support declarative dump",
}}

var portalChildCollectors = buildPortalChildCollectors()

func buildPortalChildCollectors() []portalChildCollector {
	collectors := []portalChildCollector{
		{
			childCollector: childCollection(
				"failed to load portal pages",
				func(ctx context.Context, d *childDumpContext) ([]declresources.PortalPageResource, error) {
					return buildPortalPages(ctx, d.logger, d.client, d.parentID, d.parentName)
				},
				func(p *declresources.PortalResource) *[]declresources.PortalPageResource { return &p.Pages },
			),
			fatal: true,
		},
		{
			childCollector: childCollection(
				"failed to load portal snippets",
				func(ctx context.Context, d *childDumpContext) ([]declresources.PortalSnippetResource, error) {
					return buildPortalSnippets(ctx, d.logger, d.client, d.parentID, d.parentName)
				},
				func(p *declresources.PortalResource) *[]declresources.PortalSnippetResource { return &p.Snippets },
			),
		},
		{
			// Roles nest under teams in output but share the Portal sync owner.
			childCollector: withCoScopedChildren(childCollection(
				"failed to load portal teams",
				func(ctx context.Context, d *childDumpContext) ([]declresources.PortalTeamResource, error) {
					return buildPortalTeams(ctx, d.logger, d.client, d.parentID, d.parentName)
				},
				func(p *declresources.PortalResource) *[]declresources.PortalTeamResource { return &p.Teams },
			), declresources.ResourceTypePortalTeamRole),
		},
		{
			childCollector: childCollection(
				"failed to load portal identity providers",
				func(ctx context.Context, d *childDumpContext) ([]declresources.PortalIdentityProviderResource, error) {
					return buildPortalIdentityProviders(ctx, d.client, d.parentID)
				},
				func(p *declresources.PortalResource) *[]declresources.PortalIdentityProviderResource {
					return &p.IdentityProviders
				},
			),
		},
		{
			childCollector: childSingleton(
				"failed to load portal auth settings",
				func(ctx context.Context, d *childDumpContext) (*declresources.PortalAuthSettingsResource, error) {
					return buildPortalAuthSettings(ctx, d.client, d.parentID)
				},
				func(p *declresources.PortalResource) **declresources.PortalAuthSettingsResource {
					return &p.AuthSettings
				},
			),
		},
		{
			childCollector: childSingleton(
				"failed to load portal IP allow list",
				func(ctx context.Context, d *childDumpContext) (*declresources.PortalIPAllowListResource, error) {
					return buildPortalIPAllowList(ctx, d.client, d.parentID)
				},
				func(p *declresources.PortalResource) **declresources.PortalIPAllowListResource { return &p.IPAllowList },
			),
		},
		{
			childCollector: childSingleton(
				"failed to load portal integrations",
				func(ctx context.Context, d *childDumpContext) (*declresources.PortalIntegrationResource, error) {
					return buildPortalIntegration(ctx, d.client, d.parentID)
				},
				func(p *declresources.PortalResource) **declresources.PortalIntegrationResource {
					return &p.Integrations
				},
			),
		},
		{
			childCollector: childSingleton(
				"failed to load portal customization",
				func(ctx context.Context, d *childDumpContext) (*declresources.PortalCustomizationResource, error) {
					return buildPortalCustomization(ctx, d.client, d.parentID)
				},
				func(p *declresources.PortalResource) **declresources.PortalCustomizationResource {
					return &p.Customization
				},
			),
		},
		{
			childCollector: childSingleton(
				"failed to load portal custom domain",
				func(ctx context.Context, d *childDumpContext) (*declresources.PortalCustomDomainResource, error) {
					return buildPortalCustomDomain(ctx, d.logger, d.client, d.parentID, d.parentName)
				},
				func(p *declresources.PortalResource) **declresources.PortalCustomDomainResource {
					return &p.CustomDomain
				},
			),
		},
		{
			childCollector: childSingleton(
				"failed to load portal email config",
				func(ctx context.Context, d *childDumpContext) (*declresources.PortalEmailConfigResource, error) {
					return buildPortalEmailConfig(ctx, d.client, d.parentID)
				},
				func(p *declresources.PortalResource) **declresources.PortalEmailConfigResource { return &p.EmailConfig },
			),
		},
		{
			childCollector: childMap(
				"failed to load portal email templates",
				func(ctx context.Context, d *childDumpContext) (map[string]declresources.PortalEmailTemplateResource, error) {
					return buildPortalEmailTemplates(ctx, d.client, d.parentID)
				},
				func(p *declresources.PortalResource) *map[string]declresources.PortalEmailTemplateResource {
					return &p.EmailTemplates
				},
			),
		},
		{
			childCollector: childSingleton(
				"failed to load portal audit log webhook",
				func(ctx context.Context, d *childDumpContext) (*declresources.PortalAuditLogWebhookResource, error) {
					return buildPortalAuditLogWebhook(ctx, d.client, d.parentID)
				},
				func(p *declresources.PortalResource) **declresources.PortalAuditLogWebhookResource {
					return &p.AuditLogWebhook
				},
			),
		},
		{
			childCollector: childCollector[declresources.PortalResource]{
				kind:     declresources.ResourceTypePortalAssetLogo,
				coScoped: []declresources.ResourceType{declresources.ResourceTypePortalAssetFavicon},
				warning:  "failed to load portal assets",
				collect: func(ctx context.Context, d *childDumpContext, p *declresources.PortalResource) error {
					// The builder logs each failed asset read and preserves the other asset.
					if assets := buildPortalAssets(ctx, d.logger, d.client, d.parentID, d.parentName); assets != nil {
						p.Assets = assets
					}
					return nil
				},
			},
		},
	}
	if err := validatePortalChildCollectors(collectors, portalChildOmissions); err != nil {
		panic(err)
	}
	return collectors
}

func validatePortalChildCollectors(collectors []portalChildCollector, omissions []childExportOmission) error {
	shared := make([]childCollector[declresources.PortalResource], len(collectors))
	for i, collector := range collectors {
		shared[i] = collector.childCollector
	}
	return validateChildCollectors("Portal child dump", shared, omissions...)
}
