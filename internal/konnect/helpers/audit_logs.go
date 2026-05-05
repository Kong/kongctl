package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/konnect/apiutil"
)

const (
	globalAPIBaseURL             = "https://global.api.konghq.com"
	auditLogDestinationsListPath = "/v3/audit-log-destinations"
)

// AuditLogDestination captures fields used to identify Konnect audit-log webhook destinations.
type AuditLogDestination struct {
	ID                  string
	Name                string
	Endpoint            string
	LogFormat           string
	SkipSSLVerification *bool
	CreatedAt           string
	UpdatedAt           string
}

// AuditLogDestinationsAPI exposes organization audit-log destination lookup operations.
type AuditLogDestinationsAPI interface {
	ListAuditLogDestinations(ctx context.Context) ([]AuditLogDestination, error)
}

// AuditLogDestinationsAPIImpl provides raw HTTP access for endpoints not present in the SDK.
type AuditLogDestinationsAPIImpl struct {
	Token      string
	HTTPClient kkSDK.HTTPClient
}

func (a *AuditLogDestinationsAPIImpl) ListAuditLogDestinations(ctx context.Context) ([]AuditLogDestination, error) {
	result, err := apiutil.Request(
		ctx,
		a.HTTPClient,
		http.MethodGet,
		globalAPIBaseURL,
		auditLogDestinationsListPath,
		a.Token,
		nil,
		nil,
	)
	if err != nil {
		return nil, err
	}
	if result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusMultipleChoices {
		body := strings.TrimSpace(string(result.Body))
		if body == "" {
			return nil, fmt.Errorf("list audit-log destinations failed with status %d", result.StatusCode)
		}
		return nil, fmt.Errorf("list audit-log destinations failed with status %d: %s", result.StatusCode, body)
	}

	var payload any
	if len(result.Body) > 0 {
		if err := json.Unmarshal(result.Body, &payload); err != nil {
			return nil, fmt.Errorf("failed to decode audit-log destination list response: %w", err)
		}
	}

	return extractAuditLogDestinations(payload), nil
}

// PortalAuditLogsAPI exposes portal audit-log webhook operations used by declarative config.
type PortalAuditLogsAPI interface {
	GetPortalAuditLogWebhook(ctx context.Context, portalID string,
		opts ...kkOps.Option) (*kkOps.GetPortalAuditLogWebhookResponse, error)
	UpdatePortalAuditLogWebhook(ctx context.Context, portalID string,
		body *kkComps.UpdatePortalAuditLogWebhook,
		opts ...kkOps.Option,
	) (*kkOps.UpdatePortalAuditLogWebhookResponse, error)
	DeletePortalAuditLogWebhook(ctx context.Context, portalID string,
		opts ...kkOps.Option) (*kkOps.DeletePortalAuditLogWebhookResponse, error)
}

// PortalAuditLogsAPIImpl provides a concrete implementation backed by the SDK.
type PortalAuditLogsAPIImpl struct {
	SDK *kkSDK.SDK
}

func (p *PortalAuditLogsAPIImpl) GetPortalAuditLogWebhook(ctx context.Context, portalID string,
	opts ...kkOps.Option,
) (*kkOps.GetPortalAuditLogWebhookResponse, error) {
	return p.SDK.PortalAuditLogs.GetPortalAuditLogWebhook(ctx, portalID, opts...)
}

func (p *PortalAuditLogsAPIImpl) UpdatePortalAuditLogWebhook(
	ctx context.Context,
	portalID string,
	body *kkComps.UpdatePortalAuditLogWebhook,
	opts ...kkOps.Option,
) (*kkOps.UpdatePortalAuditLogWebhookResponse, error) {
	return p.SDK.PortalAuditLogs.UpdatePortalAuditLogWebhook(ctx, portalID, body, opts...)
}

func (p *PortalAuditLogsAPIImpl) DeletePortalAuditLogWebhook(ctx context.Context, portalID string,
	opts ...kkOps.Option,
) (*kkOps.DeletePortalAuditLogWebhookResponse, error) {
	return p.SDK.PortalAuditLogs.DeletePortalAuditLogWebhook(ctx, portalID, opts...)
}

func extractAuditLogDestinations(payload any) []AuditLogDestination {
	records := make([]AuditLogDestination, 0)
	seen := make(map[string]struct{})

	var walk func(any)
	walk = func(node any) {
		switch typed := node.(type) {
		case map[string]any:
			record, ok := auditLogDestinationFromMap(typed)
			if ok {
				key := auditLogDestinationKey(record)
				if _, exists := seen[key]; !exists {
					seen[key] = struct{}{}
					records = append(records, record)
				}
				return
			}
			for _, child := range typed {
				walk(child)
			}
		case []any:
			for _, child := range typed {
				walk(child)
			}
		}
	}

	walk(payload)

	slices.SortFunc(records, func(left, right AuditLogDestination) int {
		leftName := strings.ToLower(left.Name)
		rightName := strings.ToLower(right.Name)
		if leftName == rightName {
			return strings.Compare(strings.ToLower(left.ID), strings.ToLower(right.ID))
		}
		return strings.Compare(leftName, rightName)
	})

	return records
}

func auditLogDestinationFromMap(value map[string]any) (AuditLogDestination, bool) {
	record := AuditLogDestination{
		ID:                  mapString(value, "id"),
		Name:                mapString(value, "name"),
		Endpoint:            mapString(value, "endpoint"),
		LogFormat:           mapString(value, "log_format"),
		SkipSSLVerification: mapBool(value, "skip_ssl_verification"),
		CreatedAt:           mapString(value, "created_at"),
		UpdatedAt:           mapString(value, "updated_at"),
	}
	if record.Endpoint == "" {
		return AuditLogDestination{}, false
	}
	if record.ID == "" && record.Name == "" {
		return AuditLogDestination{}, false
	}
	return record, true
}

func auditLogDestinationKey(record AuditLogDestination) string {
	// AuditLogDestination values are expected to come from auditLogDestinationFromMap/mapString,
	// which already normalizes these fields with strings.TrimSpace.
	if record.ID != "" {
		return "id:" + record.ID
	}
	return "name:endpoint:" + record.Name + "|" + record.Endpoint
}

func mapString(value map[string]any, key string) string {
	raw, ok := value[key]
	if !ok {
		return ""
	}
	str, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(str)
}

func mapBool(value map[string]any, key string) *bool {
	raw, ok := value[key]
	if !ok {
		return nil
	}
	boolVal, ok := raw.(bool)
	if !ok {
		return nil
	}
	return &boolVal
}
