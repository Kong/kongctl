package executor

import (
	"context"
	"fmt"
	"log/slog"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/log"
)

// PortalIntegrationAdapter implements SingletonOperations for portal integrations.
type PortalIntegrationAdapter struct {
	client *state.Client
}

// NewPortalIntegrationAdapter creates a new adapter.
func NewPortalIntegrationAdapter(client *state.Client) *PortalIntegrationAdapter {
	return &PortalIntegrationAdapter{client: client}
}

// MapUpdateFields maps planner fields into the SDK upsert request.
func (p *PortalIntegrationAdapter) MapUpdateFields(
	ctx context.Context,
	fields map[string]any,
	update *kkComps.PortalIntegrations,
) error {
	if logger, ok := ctx.Value(log.LoggerKey).(*slog.Logger); ok && logger != nil {
		logger.Debug("mapping portal integration update fields", "fields", fields)
	}

	if v, ok := fields[planner.FieldGoogleTagManager]; ok {
		gtm, err := mapGoogleTagManagerIntegration(v)
		if err != nil {
			return err
		}
		update.GoogleTagManager = gtm
	}

	if v, ok := fields[planner.FieldGoogleAnalytics4]; ok {
		ga4, err := mapGoogleAnalytics4Integration(v)
		if err != nil {
			return err
		}
		update.GoogleAnalytics4 = ga4
	}

	if logger, ok := ctx.Value(log.LoggerKey).(*slog.Logger); ok && logger != nil {
		logger.Debug(
			"mapped portal integration request",
			"has_google_tag_manager", update.GoogleTagManager != nil,
			"has_google_analytics_4", update.GoogleAnalytics4 != nil,
		)
	}

	return nil
}

// Update executes the API call for portal integrations.
func (p *PortalIntegrationAdapter) Update(
	ctx context.Context,
	portalID string,
	req kkComps.PortalIntegrations,
) error {
	if portalID == "" {
		return fmt.Errorf("portal ID required for portal integrations update")
	}
	return p.client.UpsertPortalIntegrations(ctx, portalID, req)
}

func (p *PortalIntegrationAdapter) ResourceType() string {
	return string(resources.ResourceTypePortalIntegration)
}

func mapGoogleTagManagerIntegration(value any) (*kkComps.GoogleTagManagerIntegration, error) {
	if value == nil {
		return nil, nil
	}

	if integration, ok := value.(kkComps.GoogleTagManagerIntegration); ok {
		return &integration, nil
	}

	fields, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("google_tag_manager must be an object")
	}

	enabled, err := requiredBool(fields, planner.FieldEnabled, planner.FieldGoogleTagManager)
	if err != nil {
		return nil, err
	}
	integrationType, err := requiredString(fields, planner.FieldType, planner.FieldGoogleTagManager)
	if err != nil {
		return nil, err
	}
	configFields, err := requiredObject(fields, planner.FieldConfigData, planner.FieldGoogleTagManager)
	if err != nil {
		return nil, err
	}
	id, err := requiredString(configFields, planner.FieldID, planner.FieldGoogleTagManager+"."+planner.FieldConfigData)
	if err != nil {
		return nil, err
	}

	config := kkComps.ConfigData{ID: id}
	config.L = optionalString(configFields, planner.FieldL)
	config.Preview = optionalString(configFields, planner.FieldPreview)
	config.CookiesWin = optionalBool(configFields, planner.FieldCookiesWin)
	config.Debug = optionalBool(configFields, planner.FieldDebug)
	config.Npa = optionalBool(configFields, planner.FieldNPA)
	config.DataLayer = optionalString(configFields, planner.FieldDataLayer)
	config.EnvName = optionalString(configFields, planner.FieldEnvName)
	config.AuthReferrerPolicy = optionalString(configFields, planner.FieldAuthReferrerPolicy)

	return &kkComps.GoogleTagManagerIntegration{
		Enabled:    enabled,
		Type:       kkComps.GoogleTagManagerIntegrationType(integrationType),
		ConfigData: config,
	}, nil
}

func mapGoogleAnalytics4Integration(value any) (*kkComps.GoogleAnalytics4Integration, error) {
	if value == nil {
		return nil, nil
	}

	if integration, ok := value.(kkComps.GoogleAnalytics4Integration); ok {
		return &integration, nil
	}

	fields, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("google_analytics_4 must be an object")
	}

	enabled, err := requiredBool(fields, planner.FieldEnabled, planner.FieldGoogleAnalytics4)
	if err != nil {
		return nil, err
	}
	integrationType, err := requiredString(fields, planner.FieldType, planner.FieldGoogleAnalytics4)
	if err != nil {
		return nil, err
	}
	configFields, err := requiredObject(fields, planner.FieldConfigData, planner.FieldGoogleAnalytics4)
	if err != nil {
		return nil, err
	}
	id, err := requiredString(configFields, planner.FieldID, planner.FieldGoogleAnalytics4+"."+planner.FieldConfigData)
	if err != nil {
		return nil, err
	}

	config := kkComps.GoogleAnalytics4IntegrationConfigData{
		ID: id,
		L:  optionalString(configFields, planner.FieldL),
	}

	return &kkComps.GoogleAnalytics4Integration{
		Enabled:    enabled,
		Type:       kkComps.GoogleAnalytics4IntegrationType(integrationType),
		ConfigData: config,
	}, nil
}

func requiredObject(fields map[string]any, field string, parent string) (map[string]any, error) {
	value, ok := fields[field]
	if !ok || value == nil {
		return nil, fmt.Errorf("%s.%s is required", parent, field)
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s.%s must be an object", parent, field)
	}
	return object, nil
}

func requiredString(fields map[string]any, field string, parent string) (string, error) {
	value, ok := fields[field].(string)
	if !ok || value == "" {
		return "", fmt.Errorf("%s.%s is required", parent, field)
	}
	return value, nil
}

func requiredBool(fields map[string]any, field string, parent string) (bool, error) {
	value, ok := fields[field].(bool)
	if !ok {
		return false, fmt.Errorf("%s.%s is required", parent, field)
	}
	return value, nil
}

func optionalString(fields map[string]any, field string) *string {
	value, ok := fields[field].(string)
	if !ok {
		return nil
	}
	return &value
}

func optionalBool(fields map[string]any, field string) *bool {
	value, ok := fields[field].(bool)
	if !ok {
		return nil
	}
	return &value
}
