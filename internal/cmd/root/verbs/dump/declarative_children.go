package dump

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkErrors "github.com/Kong/sdk-konnect-go/models/sdkerrors"

	declresources "github.com/kong/kongctl/internal/declarative/resources"
	declstate "github.com/kong/kongctl/internal/declarative/state"
)

func populatePortalChildren(
	ctx context.Context,
	logger *slog.Logger,
	client *declstate.Client,
	portals []declresources.PortalResource,
) error {
	if client == nil {
		return nil
	}

	for i := range portals {
		portal := &portals[i]
		portalID := strings.TrimSpace(portal.Ref)
		if portalID == "" {
			continue
		}

		if pages, err := buildPortalPages(ctx, logger, client, portalID, portal.Name); err != nil {
			return err
		} else if len(pages) > 0 {
			portal.Pages = pages
		}

		if snippets, err := buildPortalSnippets(ctx, logger, client, portalID, portal.Name); err != nil {
			logWarn(logger, "failed to load portal snippets", portalID, portal.Name, err)
		} else if len(snippets) > 0 {
			portal.Snippets = snippets
		}

		if teams, err := buildPortalTeams(ctx, logger, client, portalID, portal.Name); err != nil {
			logWarn(logger, "failed to load portal teams", portalID, portal.Name, err)
		} else if len(teams) > 0 {
			portal.Teams = teams
		}

		if authSettings, err := buildPortalAuthSettings(ctx, client, portalID); err != nil {
			logWarn(logger, "failed to load portal auth settings", portalID, portal.Name, err)
		} else if authSettings != nil {
			portal.AuthSettings = authSettings
		}

		if customization, err := buildPortalCustomization(ctx, client, portalID); err != nil {
			logWarn(logger, "failed to load portal customization", portalID, portal.Name, err)
		} else if customization != nil {
			portal.Customization = customization
		}

		if customDomain, err := buildPortalCustomDomain(ctx, logger, client, portalID, portal.Name); err != nil {
			logWarn(logger, "failed to load portal custom domain", portalID, portal.Name, err)
		} else if customDomain != nil {
			portal.CustomDomain = customDomain
		}

		if emailConfig, err := buildPortalEmailConfig(ctx, client, portalID); err != nil {
			logWarn(logger, "failed to load portal email config", portalID, portal.Name, err)
		} else if emailConfig != nil {
			portal.EmailConfig = emailConfig
		}

		if emailTemplates, err := buildPortalEmailTemplates(ctx, client, portalID); err != nil {
			logWarn(logger, "failed to load portal email templates", portalID, portal.Name, err)
		} else if len(emailTemplates) > 0 {
			portal.EmailTemplates = emailTemplates
		}

		if assets := buildPortalAssets(ctx, logger, client, portalID, portal.Name); assets != nil {
			portal.Assets = assets
		}
	}

	return nil
}

func populateAPIChildren(
	ctx context.Context,
	logger *slog.Logger,
	client *declstate.Client,
	apis []declresources.APIResource,
) {
	if client == nil {
		return
	}

	for i := range apis {
		api := &apis[i]
		apiID := strings.TrimSpace(api.Ref)
		if apiID == "" {
			continue
		}

		if versions, err := buildAPIVersions(ctx, logger, client, apiID, api.Name); err != nil {
			logWarn(logger, "failed to load API versions", apiID, api.Name, err)
		} else if len(versions) > 0 {
			api.Versions = versions
		}

		if documents, err := buildAPIDocuments(ctx, logger, client, apiID, api.Name); err != nil {
			logWarn(logger, "failed to load API documents", apiID, api.Name, err)
		} else if len(documents) > 0 {
			api.Documents = documents
		}

		if publications, err := buildAPIPublications(ctx, client, apiID); err != nil {
			logWarn(logger, "failed to load API publications", apiID, api.Name, err)
		} else if len(publications) > 0 {
			api.Publications = publications
		}

		if implementations, err := buildAPIImplementations(ctx, logger, client, apiID, api.Name); err != nil {
			logWarn(logger, "failed to load API implementations", apiID, api.Name, err)
		} else if len(implementations) > 0 {
			api.Implementations = implementations
		}
	}
}

func populateControlPlaneChildren(
	ctx context.Context,
	logger *slog.Logger,
	client *declstate.Client,
	controlPlanes []declresources.ControlPlaneResource,
) {
	if client == nil {
		return
	}

	for i := range controlPlanes {
		cp := &controlPlanes[i]
		controlPlaneID := strings.TrimSpace(cp.Ref)
		if controlPlaneID == "" {
			continue
		}

		if gatewayServices, err := buildGatewayServices(ctx, client, controlPlaneID); err != nil {
			logWarn(logger, "failed to load gateway services", controlPlaneID, cp.Name, err)
		} else if len(gatewayServices) > 0 {
			cp.GatewayServices = gatewayServices
		}
	}
}

func populateEventGatewayChildren(
	ctx context.Context,
	logger *slog.Logger,
	client *declstate.Client,
	gateways []declresources.EventGatewayControlPlaneResource,
) {
	if client == nil {
		return
	}

	for i := range gateways {
		gateway := &gateways[i]
		gatewayID := strings.TrimSpace(gateway.Ref)
		if gatewayID == "" {
			continue
		}

		if clusters, err := buildEventGatewayBackendClusters(ctx, logger, client, gatewayID, gateway.Name); err != nil {
			logWarn(logger, "failed to load event gateway backend clusters", gatewayID, gateway.Name, err)
		} else if len(clusters) > 0 {
			gateway.BackendClusters = clusters
		}

		if vclusters, err := buildEventGatewayVirtualClusters(ctx, logger, client, gatewayID, gateway.Name); err != nil {
			logWarn(logger, "failed to load event gateway virtual clusters", gatewayID, gateway.Name, err)
		} else if len(vclusters) > 0 {
			gateway.VirtualClusters = vclusters
		}
	}
}

func buildPortalPages(
	ctx context.Context,
	logger *slog.Logger,
	client *declstate.Client,
	portalID string,
	portalName string,
) ([]declresources.PortalPageResource, error) {
	pages, err := client.ListManagedPortalPages(ctx, portalID)
	if err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		return nil, nil
	}

	pageByID := make(map[string]*declresources.PortalPageResource, len(pages))
	parentByID := make(map[string]string, len(pages))
	order := make([]string, 0, len(pages))

	for _, page := range pages {
		parentByID[page.ID] = page.ParentPageID
		full, err := client.GetPortalPage(ctx, portalID, page.ID)
		if err != nil {
			logWarn(logger, "failed to fetch portal page", portalID, portalName, err)
			continue
		}
		if strings.TrimSpace(full.Content) == "" {
			logWarn(logger, "portal page missing content", portalID, portalName, nil)
			continue
		}
		mapped, err := mapPortalPageToResource(full)
		if err != nil {
			return nil, fmt.Errorf("portal page %q in portal %q: %w", full.ID, portalName, err)
		}
		pageByID[page.ID] = &mapped
		order = append(order, page.ID)
	}

	if len(pageByID) == 0 {
		return nil, nil
	}

	childrenByParent := make(map[string][]string)
	roots := make([]string, 0, len(pageByID))

	for _, id := range order {
		if pageByID[id] == nil {
			continue
		}
		parentID := parentByID[id]
		if parentID != "" && pageByID[parentID] != nil {
			childrenByParent[parentID] = append(childrenByParent[parentID], id)
			continue
		}
		roots = append(roots, id)
	}

	var build func(string) declresources.PortalPageResource
	build = func(id string) declresources.PortalPageResource {
		page := *pageByID[id]
		childIDs := childrenByParent[id]
		if len(childIDs) > 0 {
			page.Children = make([]declresources.PortalPageResource, 0, len(childIDs))
			for _, childID := range childIDs {
				page.Children = append(page.Children, build(childID))
			}
		}
		return page
	}

	result := make([]declresources.PortalPageResource, 0, len(roots))
	for _, id := range roots {
		result = append(result, build(id))
	}

	return result, nil
}

func mapPortalPageToResource(page *declstate.PortalPage) (declresources.PortalPageResource, error) {
	slug, err := normalizePortalPageSlug(page.Slug, page.ParentPageID)
	if err != nil {
		return declresources.PortalPageResource{}, err
	}

	res := declresources.PortalPageResource{
		CreatePortalPageRequest: kkComps.CreatePortalPageRequest{
			Slug:    slug,
			Content: page.Content,
		},
		Ref: page.ID,
	}

	if strings.TrimSpace(page.Title) != "" {
		title := page.Title
		res.Title = &title
	}
	if strings.TrimSpace(page.Description) != "" {
		desc := page.Description
		res.Description = &desc
	}
	if strings.TrimSpace(page.Visibility) != "" {
		visibility := kkComps.PageVisibilityStatus(page.Visibility)
		res.Visibility = &visibility
	}
	if strings.TrimSpace(page.Status) != "" {
		status := kkComps.PublishedStatus(page.Status)
		res.Status = &status
	}

	return res, nil
}

func normalizePortalPageSlug(rawSlug string, parentPageID string) (string, error) {
	trimmed := strings.TrimSpace(rawSlug)
	if trimmed == "" {
		return "", fmt.Errorf("slug is required")
	}

	if trimmed == "/" {
		if strings.TrimSpace(parentPageID) != "" {
			return "", fmt.Errorf("slug '/' is only valid for root pages")
		}
		return "/", nil
	}

	normalized := strings.Trim(trimmed, "/")
	if normalized == "" {
		return "", fmt.Errorf("slug is required")
	}
	if strings.Contains(normalized, "/") {
		return "", fmt.Errorf("slug must be a single path segment (got %q)", rawSlug)
	}

	return normalized, nil
}

func buildPortalSnippets(
	ctx context.Context,
	logger *slog.Logger,
	client *declstate.Client,
	portalID string,
	portalName string,
) ([]declresources.PortalSnippetResource, error) {
	snippets, err := client.ListPortalSnippets(ctx, portalID)
	if err != nil {
		return nil, err
	}
	if len(snippets) == 0 {
		return nil, nil
	}

	results := make([]declresources.PortalSnippetResource, 0, len(snippets))
	for _, snippet := range snippets {
		full, err := client.GetPortalSnippet(ctx, portalID, snippet.ID)
		if err != nil {
			logWarn(logger, "failed to fetch portal snippet", portalID, portalName, err)
			continue
		}
		if strings.TrimSpace(full.Content) == "" {
			logWarn(logger, "portal snippet missing content", portalID, portalName, nil)
			continue
		}
		results = append(results, mapPortalSnippetToResource(full))
	}

	return results, nil
}

func mapPortalSnippetToResource(snippet *declstate.PortalSnippet) declresources.PortalSnippetResource {
	res := declresources.PortalSnippetResource{
		Ref:     snippet.ID,
		Name:    snippet.Name,
		Content: snippet.Content,
	}

	if strings.TrimSpace(snippet.Title) != "" {
		title := snippet.Title
		res.Title = &title
	}
	if strings.TrimSpace(snippet.Description) != "" {
		desc := snippet.Description
		res.Description = &desc
	}
	if strings.TrimSpace(snippet.Visibility) != "" {
		visibility := kkComps.SnippetVisibilityStatus(snippet.Visibility)
		res.Visibility = &visibility
	}
	if strings.TrimSpace(snippet.Status) != "" {
		status := kkComps.PublishedStatus(snippet.Status)
		res.Status = &status
	}

	return res
}

func buildPortalTeams(
	ctx context.Context,
	logger *slog.Logger,
	client *declstate.Client,
	portalID string,
	portalName string,
) ([]declresources.PortalTeamResource, error) {
	teams, err := client.ListPortalTeams(ctx, portalID)
	if err != nil {
		return nil, err
	}
	if len(teams) == 0 {
		return nil, nil
	}

	results := make([]declresources.PortalTeamResource, 0, len(teams))
	for _, team := range teams {
		teamRes := declresources.PortalTeamResource{
			PortalCreateTeamRequest: kkComps.PortalCreateTeamRequest{
				Name: team.Name,
			},
			Ref: team.ID,
		}

		if strings.TrimSpace(team.Description) != "" {
			desc := team.Description
			teamRes.Description = &desc
		}

		roles, err := client.ListPortalTeamRoles(ctx, portalID, team.ID)
		if err != nil {
			logWarn(logger, "failed to load portal team roles", portalID, portalName, err)
			results = append(results, teamRes)
			continue
		}

		if len(roles) > 0 {
			teamRes.Roles = make([]declresources.PortalTeamRoleResource, 0, len(roles))
			for _, role := range roles {
				teamRes.Roles = append(teamRes.Roles, declresources.PortalTeamRoleResource{
					Ref:            role.ID,
					RoleName:       role.RoleName,
					EntityID:       role.EntityID,
					EntityTypeName: role.EntityTypeName,
					EntityRegion:   role.EntityRegion,
				})
			}
		}

		results = append(results, teamRes)
	}

	return results, nil
}

func buildPortalAuthSettings(
	ctx context.Context,
	client *declstate.Client,
	portalID string,
) (*declresources.PortalAuthSettingsResource, error) {
	settings, err := client.GetPortalAuthSettings(ctx, portalID)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	ref := buildChildRef("portal-auth-settings", portalID)
	resource := declresources.PortalAuthSettingsResource{
		Ref: ref,
		PortalAuthenticationSettingsUpdateRequest: kkComps.PortalAuthenticationSettingsUpdateRequest{
			BasicAuthEnabled:       boolPointer(settings.BasicAuthEnabled),
			OidcAuthEnabled:        boolPointer(settings.OidcAuthEnabled),
			SamlAuthEnabled:        settings.SamlAuthEnabled,
			OidcTeamMappingEnabled: boolPointer(settings.OidcTeamMappingEnabled),
			KonnectMappingEnabled:  boolPointer(settings.KonnectMappingEnabled),
			IdpMappingEnabled:      settings.IdpMappingEnabled,
		},
	}

	if settings.OidcConfig != nil {
		if strings.TrimSpace(settings.OidcConfig.Issuer) != "" {
			resource.OidcIssuer = stringPointer(settings.OidcConfig.Issuer)
		}
		if strings.TrimSpace(settings.OidcConfig.ClientID) != "" {
			resource.OidcClientID = stringPointer(settings.OidcConfig.ClientID)
		}
		if len(settings.OidcConfig.Scopes) > 0 {
			resource.OidcScopes = settings.OidcConfig.Scopes
		}
		if settings.OidcConfig.ClaimMappings != nil {
			resource.OidcClaimMappings = &kkComps.PortalAuthenticationSettingsUpdateRequestPortalClaimMappings{
				Name:   settings.OidcConfig.ClaimMappings.Name,
				Email:  settings.OidcConfig.ClaimMappings.Email,
				Groups: settings.OidcConfig.ClaimMappings.Groups,
			}
		}
	}

	return &resource, nil
}

func buildPortalCustomization(
	ctx context.Context,
	client *declstate.Client,
	portalID string,
) (*declresources.PortalCustomizationResource, error) {
	customization, err := client.GetPortalCustomization(ctx, portalID)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	if customization == nil {
		return nil, nil
	}

	ref := buildChildRef("portal-customization", portalID)
	resource := declresources.PortalCustomizationResource{
		PortalCustomization: *customization,
		Ref:                 ref,
	}

	return &resource, nil
}

func buildPortalCustomDomain(
	ctx context.Context,
	logger *slog.Logger,
	client *declstate.Client,
	portalID string,
	portalName string,
) (*declresources.PortalCustomDomainResource, error) {
	domain, err := client.GetPortalCustomDomain(ctx, portalID)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	if domain == nil {
		return nil, nil
	}

	method := strings.ToLower(strings.TrimSpace(domain.DomainVerificationMethod))
	switch method {
	case "http":
		ssl := kkComps.CreateCreatePortalCustomDomainSSLHTTP(kkComps.HTTP{})
		resource := declresources.PortalCustomDomainResource{
			CreatePortalCustomDomainRequest: kkComps.CreatePortalCustomDomainRequest{
				Hostname: domain.Hostname,
				Enabled:  domain.Enabled,
				Ssl:      ssl,
			},
			Ref: buildChildRef("portal-custom-domain", portalID),
		}
		return &resource, nil
	case "custom_certificate":
		logWarn(logger, "portal custom domain uses custom_certificate; skipping (certificate data not available)",
			portalID, portalName, nil)
		return nil, nil
	default:
		if method == "" {
			return nil, nil
		}
		logWarn(logger, "portal custom domain uses unsupported verification method; skipping", portalID, portalName, nil)
		return nil, nil
	}
}

func buildPortalEmailConfig(
	ctx context.Context,
	client *declstate.Client,
	portalID string,
) (*declresources.PortalEmailConfigResource, error) {
	cfg, err := client.GetPortalEmailConfig(ctx, portalID)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	if cfg == nil {
		return nil, nil
	}

	ref := cfg.ID
	if strings.TrimSpace(ref) == "" {
		ref = buildChildRef("portal-email-config", portalID)
	}

	resource := declresources.PortalEmailConfigResource{
		Ref: ref,
		PostPortalEmailConfig: kkComps.PostPortalEmailConfig{
			DomainName:   cfg.DomainName,
			FromName:     stringPointer(cfg.FromName),
			FromEmail:    stringPointer(cfg.FromEmail),
			ReplyToEmail: stringPointer(cfg.ReplyToEmail),
		},
	}

	return &resource, nil
}

func buildPortalEmailTemplates(
	ctx context.Context,
	client *declstate.Client,
	portalID string,
) (map[string]declresources.PortalEmailTemplateResource, error) {
	templates, err := client.ListPortalCustomEmailTemplates(ctx, portalID)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(templates) == 0 {
		return nil, nil
	}

	result := make(map[string]declresources.PortalEmailTemplateResource, len(templates))
	for _, tpl := range templates {
		name := strings.TrimSpace(tpl.Name)
		if name == "" {
			continue
		}
		resource := declresources.PortalEmailTemplateResource{
			Ref:     buildChildRef("portal-email-template", portalID, name),
			Name:    kkComps.EmailTemplateName(name),
			Enabled: boolPointer(tpl.Enabled),
		}

		if tpl.Content != nil {
			resource.Content = &declresources.PortalEmailTemplateContent{
				Subject:     tpl.Content.Subject,
				Title:       tpl.Content.Title,
				Body:        tpl.Content.Body,
				ButtonLabel: tpl.Content.ButtonLabel,
			}
		}

		result[name] = resource
	}

	return result, nil
}

func buildPortalAssets(
	ctx context.Context,
	logger *slog.Logger,
	client *declstate.Client,
	portalID string,
	portalName string,
) *declresources.PortalAssetsResource {
	var assets declresources.PortalAssetsResource
	var hasAny bool

	logo, err := client.GetPortalAssetLogo(ctx, portalID)
	if err != nil {
		if !isNotFound(err) {
			logWarn(logger, "failed to fetch portal logo", portalID, portalName, err)
		}
	} else if strings.TrimSpace(logo) != "" {
		assets.Logo = &logo
		hasAny = true
	}

	favicon, err := client.GetPortalAssetFavicon(ctx, portalID)
	if err != nil {
		if !isNotFound(err) {
			logWarn(logger, "failed to fetch portal favicon", portalID, portalName, err)
		}
	} else if strings.TrimSpace(favicon) != "" {
		assets.Favicon = &favicon
		hasAny = true
	}

	if !hasAny {
		return nil
	}

	return &assets
}

func buildAPIVersions(
	ctx context.Context,
	logger *slog.Logger,
	client *declstate.Client,
	apiID string,
	apiName string,
) ([]declresources.APIVersionResource, error) {
	versions, err := client.ListAPIVersions(ctx, apiID)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, nil
	}

	results := make([]declresources.APIVersionResource, 0, len(versions))
	for _, version := range versions {
		full, err := client.FetchAPIVersion(ctx, apiID, version.ID)
		if err != nil {
			logWarn(logger, "failed to fetch API version", apiID, apiName, err)
			continue
		}
		if full == nil || strings.TrimSpace(full.Spec) == "" {
			logWarn(logger, "API version missing spec content", apiID, apiName, nil)
			continue
		}

		ver := full.Version
		res := declresources.APIVersionResource{
			CreateAPIVersionRequest: kkComps.CreateAPIVersionRequest{
				Version: &ver,
				Spec: kkComps.CreateAPIVersionRequestSpec{
					Content: stringPointer(full.Spec),
				},
			},
			Ref: full.ID,
		}

		results = append(results, res)
	}

	return results, nil
}

func buildAPIPublications(
	ctx context.Context,
	client *declstate.Client,
	apiID string,
) ([]declresources.APIPublicationResource, error) {
	publications, err := client.ListAPIPublications(ctx, apiID)
	if err != nil {
		return nil, err
	}
	if len(publications) == 0 {
		return nil, nil
	}

	results := make([]declresources.APIPublicationResource, 0, len(publications))
	for _, publication := range publications {
		ref, err := resolveAPIPublicationRef(apiID, publication.PortalID, publication.ID)
		if err != nil {
			return nil, err
		}

		res := declresources.APIPublicationResource{
			APIPublication: kkComps.APIPublication{
				AuthStrategyIds:          publication.AuthStrategyIDs,
				AutoApproveRegistrations: boolPointer(publication.AutoApproveRegistrations),
			},
			Ref:      ref,
			PortalID: strings.TrimSpace(publication.PortalID),
		}

		if strings.TrimSpace(publication.Visibility) != "" {
			vis := kkComps.APIPublicationVisibility(publication.Visibility)
			res.Visibility = &vis
		}

		results = append(results, res)
	}

	return results, nil
}

func resolveAPIPublicationRef(apiID, portalID, publicationID string) (string, error) {
	ref := strings.TrimSpace(publicationID)
	if ref != "" {
		return ref, nil
	}

	portalID = strings.TrimSpace(portalID)
	if portalID == "" {
		return "", fmt.Errorf("api publication missing portal_id")
	}

	return buildChildRef("api-publication", apiID, portalID), nil
}

func buildAPIImplementations(
	ctx context.Context,
	logger *slog.Logger,
	client *declstate.Client,
	apiID string,
	apiName string,
) ([]declresources.APIImplementationResource, error) {
	implementations, err := client.ListAPIImplementations(ctx, apiID)
	if err != nil {
		return nil, err
	}
	if len(implementations) == 0 {
		return nil, nil
	}

	results := make([]declresources.APIImplementationResource, 0, len(implementations))
	for _, impl := range implementations {
		if impl.Service == nil || strings.TrimSpace(impl.Service.ID) == "" ||
			strings.TrimSpace(impl.Service.ControlPlaneID) == "" {
			logWarn(logger, "API implementation missing service reference; skipping", apiID, apiName, nil)
			continue
		}

		service := kkComps.APIImplementationService{
			ID:             impl.Service.ID,
			ControlPlaneID: impl.Service.ControlPlaneID,
		}
		ref := kkComps.ServiceReference{Service: &service}
		res := declresources.APIImplementationResource{
			Ref:               impl.ID,
			APIImplementation: kkComps.CreateAPIImplementationServiceReference(ref),
		}

		results = append(results, res)
	}

	return results, nil
}

func buildAPIDocuments(
	ctx context.Context,
	logger *slog.Logger,
	client *declstate.Client,
	apiID string,
	apiName string,
) ([]declresources.APIDocumentResource, error) {
	documents, err := client.ListAPIDocuments(ctx, apiID)
	if err != nil {
		return nil, err
	}
	if len(documents) == 0 {
		return nil, nil
	}

	docByID := make(map[string]*declresources.APIDocumentResource, len(documents))
	parentByID := make(map[string]string, len(documents))
	order := make([]string, 0, len(documents))

	for _, doc := range documents {
		parentByID[doc.ID] = doc.ParentDocumentID
		full, err := client.GetAPIDocument(ctx, apiID, doc.ID)
		if err != nil {
			logWarn(logger, "failed to fetch API document", apiID, apiName, err)
			continue
		}
		if full == nil || strings.TrimSpace(full.Content) == "" {
			logWarn(logger, "API document missing content", apiID, apiName, nil)
			continue
		}
		mapped := mapAPIDocumentToResource(full)
		docByID[doc.ID] = &mapped
		order = append(order, doc.ID)
	}

	if len(docByID) == 0 {
		return nil, nil
	}

	childrenByParent := make(map[string][]string)
	roots := make([]string, 0, len(docByID))

	for _, id := range order {
		if docByID[id] == nil {
			continue
		}
		parentID := parentByID[id]
		if parentID != "" && docByID[parentID] != nil {
			childrenByParent[parentID] = append(childrenByParent[parentID], id)
			continue
		}
		roots = append(roots, id)
	}

	var build func(string) declresources.APIDocumentResource
	build = func(id string) declresources.APIDocumentResource {
		doc := *docByID[id]
		childIDs := childrenByParent[id]
		if len(childIDs) > 0 {
			doc.Children = make([]declresources.APIDocumentResource, 0, len(childIDs))
			for _, childID := range childIDs {
				doc.Children = append(doc.Children, build(childID))
			}
		}
		return doc
	}

	result := make([]declresources.APIDocumentResource, 0, len(roots))
	for _, id := range roots {
		result = append(result, build(id))
	}

	return result, nil
}

func mapAPIDocumentToResource(doc *declstate.APIDocument) declresources.APIDocumentResource {
	res := declresources.APIDocumentResource{
		CreateAPIDocumentRequest: kkComps.CreateAPIDocumentRequest{
			Content: doc.Content,
		},
		Ref: doc.ID,
	}

	if strings.TrimSpace(doc.Title) != "" {
		title := doc.Title
		res.Title = &title
	}
	if strings.TrimSpace(doc.Slug) != "" {
		slug := doc.Slug
		res.Slug = &slug
	}
	if strings.TrimSpace(doc.Status) != "" {
		status := kkComps.APIDocumentStatus(doc.Status)
		res.Status = &status
	}

	return res
}

func buildGatewayServices(
	ctx context.Context,
	client *declstate.Client,
	controlPlaneID string,
) ([]declresources.GatewayServiceResource, error) {
	services, err := client.ListGatewayServices(ctx, controlPlaneID)
	if err != nil {
		return nil, err
	}
	if len(services) == 0 {
		return nil, nil
	}

	results := make([]declresources.GatewayServiceResource, 0, len(services))
	for _, svc := range services {
		ref := strings.TrimSpace(svc.ID)
		if ref == "" {
			name := ""
			if svc.Service.Name != nil {
				name = strings.TrimSpace(*svc.Service.Name)
			}
			if name == "" {
				return nil, fmt.Errorf("gateway service missing id and name for control plane %s", controlPlaneID)
			}
			ref = buildChildRef("gateway-service", controlPlaneID, name)
			res := declresources.GatewayServiceResource{
				Ref: ref,
				External: &declresources.ExternalBlock{
					Selector: &declresources.ExternalSelector{
						MatchFields: map[string]string{"name": name},
					},
				},
			}
			results = append(results, res)
			continue
		}

		res := declresources.GatewayServiceResource{
			Ref: ref,
			External: &declresources.ExternalBlock{
				ID: ref,
			},
		}
		results = append(results, res)
	}

	return results, nil
}

func buildEventGatewayBackendClusters(
	ctx context.Context,
	logger *slog.Logger,
	client *declstate.Client,
	gatewayID string,
	gatewayName string,
) ([]declresources.EventGatewayBackendClusterResource, error) {
	clusters, err := client.ListEventGatewayBackendClusters(ctx, gatewayID)
	if err != nil {
		return nil, err
	}
	if len(clusters) == 0 {
		return nil, nil
	}

	results := make([]declresources.EventGatewayBackendClusterResource, 0, len(clusters))
	for _, cluster := range clusters {
		auth, err := convertBackendClusterAuthentication(cluster.Authentication)
		if err != nil {
			logWarn(logger, "failed to map backend cluster authentication", gatewayID, gatewayName, err)
			continue
		}

		res := declresources.EventGatewayBackendClusterResource{
			CreateBackendClusterRequest: kkComps.CreateBackendClusterRequest{
				Name:                                     cluster.Name,
				Description:                              cluster.Description,
				Authentication:                           auth,
				InsecureAllowAnonymousVirtualClusterAuth: cluster.InsecureAllowAnonymousVirtualClusterAuth,
				BootstrapServers:                         cluster.BootstrapServers,
				TLS:                                      cluster.TLS,
				MetadataUpdateIntervalSeconds:            cluster.MetadataUpdateIntervalSeconds,
				Labels:                                   cluster.Labels,
			},
			Ref: cluster.ID,
		}

		results = append(results, res)
	}

	return results, nil
}

func buildEventGatewayVirtualClusters(
	ctx context.Context,
	logger *slog.Logger,
	client *declstate.Client,
	gatewayID string,
	gatewayName string,
) ([]declresources.EventGatewayVirtualClusterResource, error) {
	clusters, err := client.ListEventGatewayVirtualClusters(ctx, gatewayID)
	if err != nil {
		return nil, err
	}
	if len(clusters) == 0 {
		return nil, nil
	}

	results := make([]declresources.EventGatewayVirtualClusterResource, 0, len(clusters))
	for _, cluster := range clusters {
		auth, err := convertVirtualClusterAuthentication(cluster.Authentication)
		if err != nil {
			logWarn(logger, "failed to map virtual cluster authentication", gatewayID, gatewayName, err)
			continue
		}

		destination := convertBackendClusterDestination(cluster.Destination)
		res := declresources.EventGatewayVirtualClusterResource{
			CreateVirtualClusterRequest: kkComps.CreateVirtualClusterRequest{
				Name:           cluster.Name,
				Description:    cluster.Description,
				Destination:    destination,
				Authentication: auth,
				Namespace:      cluster.Namespace,
				ACLMode:        cluster.ACLMode,
				DNSLabel:       cluster.DNSLabel,
				Labels:         cluster.Labels,
			},
			Ref: cluster.ID,
		}

		results = append(results, res)
	}

	return results, nil
}

func convertBackendClusterAuthentication(
	auth kkComps.BackendClusterAuthenticationSensitiveDataAwareScheme,
) (kkComps.BackendClusterAuthenticationScheme, error) {
	payload, err := json.Marshal(auth)
	if err != nil {
		return kkComps.BackendClusterAuthenticationScheme{}, err
	}

	var converted kkComps.BackendClusterAuthenticationScheme
	if err := json.Unmarshal(payload, &converted); err != nil {
		return kkComps.BackendClusterAuthenticationScheme{}, err
	}

	return converted, nil
}

func convertVirtualClusterAuthentication(
	auth []kkComps.VirtualClusterAuthenticationSensitiveDataAwareScheme,
) ([]kkComps.VirtualClusterAuthenticationScheme, error) {
	if len(auth) == 0 {
		return []kkComps.VirtualClusterAuthenticationScheme{}, nil
	}

	payload, err := json.Marshal(auth)
	if err != nil {
		return nil, err
	}

	var converted []kkComps.VirtualClusterAuthenticationScheme
	if err := json.Unmarshal(payload, &converted); err != nil {
		return nil, err
	}

	return converted, nil
}

func convertBackendClusterDestination(dest kkComps.BackendClusterReference) kkComps.BackendClusterReferenceModify {
	if strings.TrimSpace(dest.ID) != "" {
		return kkComps.CreateBackendClusterReferenceModifyBackendClusterReferenceByID(
			kkComps.BackendClusterReferenceByID{ID: dest.ID},
		)
	}
	return kkComps.CreateBackendClusterReferenceModifyBackendClusterReferenceByName(
		kkComps.BackendClusterReferenceByName{Name: dest.Name},
	)
}

func boolPointer(val bool) *bool {
	return &val
}

func buildChildRef(prefix string, parts ...string) string {
	cleanPrefix := sanitizeRefPart(prefix)
	base := cleanPrefix
	if len(parts) > 0 {
		base = base + ":" + strings.Join(parts, ":")
	}

	hash := sha256.Sum256([]byte(base))
	suffix := hex.EncodeToString(hash[:])[:12]

	ref := cleanPrefix + "-" + suffix
	if len(ref) <= declresources.MaxRefLength {
		return ref
	}

	trimLen := declresources.MaxRefLength - len(suffix) - 1
	if trimLen < 1 {
		return suffix
	}

	return cleanPrefix[:trimLen] + "-" + suffix
}

func sanitizeRefPart(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "ref"
	}

	var b strings.Builder
	b.Grow(len(trimmed))
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('-')
	}

	out := strings.Trim(b.String(), "-_")
	if out == "" {
		return "ref"
	}

	first := out[0]
	if (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') || (first >= '0' && first <= '9') {
		return out
	}

	return "r" + out
}

func logWarn(logger *slog.Logger, message, resourceID, resourceName string, err error) {
	if logger == nil {
		return
	}

	fields := []any{}
	if resourceID != "" {
		fields = append(fields, "resource_id", resourceID)
	}
	if resourceName != "" {
		fields = append(fields, "resource_name", resourceName)
	}
	if err != nil {
		fields = append(fields, "error", err)
	}

	logger.Warn(message, fields...)
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var notFound *kkErrors.NotFoundError
	return errors.As(err, &notFound)
}
