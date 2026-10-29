package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"reflect"
	"strings"
	"unicode/utf8"

	kkSDK "github.com/Kong/sdk-konnect-go"
	c "github.com/Kong/sdk-konnect-go/models/components"
	o "github.com/Kong/sdk-konnect-go/models/operations"
)

// BillingKind identifies native API collections, distinct from declarative resource types.
type BillingKind string

const (
	BillingTaxCode  BillingKind = "tax-codes"
	BillingCurrency BillingKind = "currencies"
	BillingApp      BillingKind = "apps"
	BillingProfile  BillingKind = "profiles"
	BillingPlan     BillingKind = "plans"
)

// BillingObject preserves the complete API payload, including embedded pricing
// unions. Identity and lifecycle are interpreted by each resource's planner.
type BillingObject map[string]any

func (b BillingObject) String(field string) string { v, _ := b[field].(string); return v }
func (b BillingObject) ID() string                 { return b.String("id") }
func (b BillingObject) Labels() map[string]string {
	result := map[string]string{}
	switch labels := b["labels"].(type) {
	case map[string]any:
		for k, v := range labels {
			if s, ok := v.(string); ok {
				result[k] = s
			}
		}
	case map[string]string:
		return maps.Clone(labels)
	}
	return result
}

// BillingCatalogAPI isolates the scoped SDK operations used by both CLI paths.
// Currency deliberately has no update/delete contract.
type BillingListOptions struct {
	IncludeDeleted  bool
	CurrencyType    string
	ExpandCostBasis bool
}

type BillingCatalogAPI interface {
	List(context.Context, BillingKind, int64, ...BillingListOptions) ([]BillingObject, error)
	Get(context.Context, BillingKind, string) (BillingObject, error)
	Create(context.Context, BillingKind, map[string]any) (BillingObject, error)
	Update(context.Context, BillingKind, string, map[string]any) (BillingObject, error)
	Delete(context.Context, BillingKind, string) error
	PlanAction(context.Context, string, string) (BillingObject, error)
	Catalog(context.Context, string, int64) (any, error)
	CostBases(context.Context, string, int64) (any, error)
	CreateCostBasis(context.Context, string, c.CreateCostBasisRequest) (any, error)
}

type billingCatalogSDK struct{ sdk *kkSDK.SDK }

func (k *KonnectSDK) GetBillingCatalogAPI() BillingCatalogAPI {
	if k.SDK == nil {
		return nil
	}
	return &billingCatalogSDK{sdk: k.SDK}
}

func (m *MockKonnectSDK) GetBillingCatalogAPI() BillingCatalogAPI {
	if m.BillingCatalogFactory != nil {
		return m.BillingCatalogFactory()
	}
	return nil
}

func billingObject(value any) (BillingObject, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var result BillingObject
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("billing response missing data")
	}
	return result, nil
}

// DecodeBillingRequest rejects unknown fields even inside generated SDK unions.
// Generated union decode errors include the entire input; never return them,
// because app requests may contain credentials.
func DecodeBillingRequest(fields map[string]any, target any) error {
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("invalid billing request")
	}
	if err = json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("invalid billing request: check field types and union discriminators")
	}
	var canonical map[string]any
	if err := json.Unmarshal(data, &canonical); err != nil {
		return fmt.Errorf("invalid billing request")
	}
	if err := ValidateBillingRequired(canonical, target); err != nil {
		return err
	}
	normalized, err := billingObject(target)
	if err != nil {
		return fmt.Errorf("invalid billing request")
	}
	return checkBillingFields(canonical, normalized, "")
}

func checkBillingFields(input, output map[string]any, path string) error {
	for key, value := range input {
		out, ok := output[key]
		if !ok {
			if value == nil {
				continue
			}
			// Empty optional maps and arrays disappear through SDK omitempty fields.
			if m, ok := value.(map[string]any); ok && len(m) == 0 {
				continue
			}
			if a, ok := value.([]any); ok && len(a) == 0 {
				continue
			}
			return fmt.Errorf("unsupported billing request field %s%s", path, key)
		}
		if in, ok := value.(map[string]any); ok {
			if other, ok := out.(map[string]any); ok {
				if err := checkBillingFields(in, other, path+key+"."); err != nil {
					return err
				}
			}
		}
		if in, ok := value.([]any); ok {
			if other, ok := out.([]any); ok {
				for i, v := range in {
					if m, ok := v.(map[string]any); ok && i < len(other) {
						if n, ok := other[i].(map[string]any); ok {
							if err := checkBillingFields(m, n, fmt.Sprintf("%s%s[%d].", path, key, i)); err != nil {
								return err
							}
						}
					}
				}
			}
		}
	}
	return nil
}

// NormalizeBillingFields validates the SDK request and materializes only literal
// API defaults. The caller retains explicit field-presence validation.
func NormalizeBillingFields(kind BillingKind, fields map[string]any, update bool) (map[string]any, error) {
	var req any
	switch kind {
	case BillingTaxCode:
		if update {
			req = &c.UpsertTaxCodeRequest{}
		} else {
			req = &c.CreateTaxCodeRequest{}
		}
	case BillingCurrency:
		if update {
			return nil, fmt.Errorf("currencies cannot be updated")
		}
		req = &c.CreateCurrencyCustomRequest{}
	case BillingApp:
		req = &c.BillingUpdateAppRequest{}
		if !update {
			return normalizeBillingAppInstall(fields)
		}
	case BillingProfile:
		if update {
			req = &c.UpsertBillingProfileRequest{}
		} else {
			req = &c.CreateBillingProfileRequest{}
		}
	case BillingPlan:
		if update {
			req = &c.UpsertPlanRequest{}
		} else {
			req = &c.CreatePlanRequest{}
		}
	default:
		return nil, fmt.Errorf("unsupported billing resource %q", kind)
	}
	if err := DecodeBillingRequest(fields, req); err != nil {
		return nil, err
	}
	return billingObject(req)
}

func normalizeBillingAppInstall(fields map[string]any) (map[string]any, error) {
	install := maps.Clone(fields)
	env, _ := install["api_key_env"].(string)
	delete(install, "api_key_env")
	missingKey := install["type"] == "stripe" && install["api_key"] == nil
	if env != "" || missingKey {
		install["api_key"] = "environment-secret"
	}
	for _, field := range []string{"labels", "description", "enable_draft_sync_hook", "enable_issuing_sync_hook"} {
		delete(install, field)
	}
	var req c.BillingInstallAppRequest
	if err := DecodeBillingRequest(install, &req); err != nil {
		return nil, err
	}
	result, err := billingObject(req)
	if err != nil {
		return nil, err
	}
	if missingKey {
		delete(result, "api_key")
	}
	if env != "" {
		delete(result, "api_key")
		result["api_key_env"] = env
	}
	for _, field := range []string{"labels", "description", "enable_draft_sync_hook", "enable_issuing_sync_hook"} {
		if value, ok := fields[field]; ok {
			result[field] = value
		}
	}
	update := maps.Clone(result)
	for _, field := range []string{"api_key", "api_key_env", "create_billing_profile"} {
		delete(update, field)
	}
	needsMetadata := false
	for _, field := range []string{"labels", "description", "enable_draft_sync_hook", "enable_issuing_sync_hook"} {
		if _, ok := fields[field]; ok {
			needsMetadata = true
		}
	}
	if needsMetadata {
		var metadata c.BillingUpdateAppRequest
		if err := DecodeBillingRequest(update, &metadata); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (a *billingCatalogSDK) List(
	ctx context.Context,
	kind BillingKind,
	size int64,
	options ...BillingListOptions,
) ([]BillingObject, error) {
	if size < 1 || size > 1000 {
		return nil, fmt.Errorf("page size must be between 1 and 1000")
	}
	settings := BillingListOptions{}
	if len(options) > 1 {
		return nil, fmt.Errorf("at most one list options value is accepted")
	}
	if len(options) == 1 {
		settings = options[0]
	}
	if settings.CurrencyType != "" && settings.CurrencyType != "custom" && settings.CurrencyType != "fiat" {
		return nil, fmt.Errorf("currency type must be custom or fiat")
	}
	result := make([]BillingObject, 0)
	for number := int64(1); number <= 10000; number++ {
		page := &c.PagePaginationQuery{Size: &size, Number: &number}
		var values []BillingObject
		var total float64
		switch kind {
		case BillingTaxCode:
			resp, err := a.sdk.OpenMeterTax.ListTaxCodes(ctx, page, &settings.IncludeDeleted)
			if err != nil {
				return nil, err
			}
			if resp == nil || resp.TaxCodePagePaginatedResponse == nil {
				return nil, fmt.Errorf("list %s response missing data", kind)
			}
			for _, v := range resp.TaxCodePagePaginatedResponse.Data {
				b, err := billingObject(v)
				if err != nil {
					return nil, err
				}
				values = append(values, b)
			}
			total = resp.TaxCodePagePaginatedResponse.Meta.Page.Total
		case BillingCurrency:
			resp, err := a.sdk.OpenMeterCurrencies.ListCurrencies(ctx, billingCurrenciesRequest(page, settings))
			if err != nil {
				return nil, err
			}
			if resp == nil || resp.CurrencyPagePaginatedResponse == nil {
				return nil, fmt.Errorf("list %s response missing data", kind)
			}
			for _, v := range resp.CurrencyPagePaginatedResponse.Data {
				b, err := billingObject(v)
				if err != nil {
					return nil, err
				}
				values = append(values, b)
			}
			total = resp.CurrencyPagePaginatedResponse.Meta.Page.Total
		case BillingApp:
			resp, err := a.sdk.OpenMeterApps.ListApps(ctx, o.ListAppsRequest{Page: page})
			if err != nil {
				return nil, err
			}
			if resp == nil || resp.AppPagePaginatedResponse == nil {
				return nil, fmt.Errorf("list %s response missing data", kind)
			}
			for _, v := range resp.AppPagePaginatedResponse.Data {
				b, err := billingObject(v)
				if err != nil {
					return nil, err
				}
				values = append(values, b)
			}
			total = resp.AppPagePaginatedResponse.Meta.Page.Total
		case BillingProfile:
			resp, err := a.sdk.OpenMeterBillingSettings.ListBillingProfiles(
				ctx,
				o.ListBillingProfilesRequest{Page: page},
			)
			if err != nil {
				return nil, err
			}
			if resp == nil || resp.BillingProfilePagePaginatedResponse == nil {
				return nil, fmt.Errorf("list %s response missing data", kind)
			}
			for _, v := range resp.BillingProfilePagePaginatedResponse.Data {
				b, err := billingObject(v)
				if err != nil {
					return nil, err
				}
				values = append(values, b)
			}
			total = resp.BillingProfilePagePaginatedResponse.Meta.Page.Total
		case BillingPlan:
			resp, err := a.sdk.OpenMeterProductCatalog.ListPlans(ctx, o.ListPlansRequest{Page: page})
			if err != nil {
				return nil, err
			}
			if resp == nil || resp.PlanPagePaginatedResponse == nil {
				return nil, fmt.Errorf("list %s response missing data", kind)
			}
			for _, v := range resp.PlanPagePaginatedResponse.Data {
				b, err := billingObject(v)
				if err != nil {
					return nil, err
				}
				values = append(values, b)
			}
			total = resp.PlanPagePaginatedResponse.Meta.Page.Total
		default:
			return nil, fmt.Errorf("unsupported billing resource %q", kind)
		}
		result = append(result, values...)
		if len(values) == 0 || float64(len(result)) >= total {
			return result, nil
		}
	}
	return nil, fmt.Errorf("billing pagination exceeded safety limit")
}

func (a *billingCatalogSDK) Get(ctx context.Context, kind BillingKind, id string) (BillingObject, error) {
	if !IsMeterULID(id) {
		return nil, fmt.Errorf("billing resource id must be an uppercase ULID")
	}
	switch kind {
	case BillingTaxCode:
		resp, err := a.sdk.OpenMeterTax.GetTaxCode(ctx, id)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.BillingTaxCode == nil {
			return nil, fmt.Errorf("get %s response missing data", kind)
		}
		return billingObject(resp.BillingTaxCode)
	case BillingCurrency:
		resp, err := a.sdk.OpenMeterCurrencies.GetCustomCurrency(ctx, id)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.BillingCurrencyCustom == nil {
			return nil, fmt.Errorf("get %s response missing data", kind)
		}
		return billingObject(resp.BillingCurrencyCustom)
	case BillingApp:
		resp, err := a.sdk.OpenMeterApps.GetApp(ctx, id)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.BillingApp == nil {
			return nil, fmt.Errorf("get %s response missing data", kind)
		}
		return billingObject(resp.BillingApp)
	case BillingProfile:
		resp, err := a.sdk.OpenMeterBillingSettings.GetBillingProfile(ctx, id)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.BillingProfile == nil {
			return nil, fmt.Errorf("get %s response missing data", kind)
		}
		return billingObject(resp.BillingProfile)
	case BillingPlan:
		resp, err := a.sdk.OpenMeterProductCatalog.GetPlan(ctx, id)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.BillingPlan == nil {
			return nil, fmt.Errorf("get %s response missing data", kind)
		}
		return billingObject(resp.BillingPlan)
	default:
		return nil, fmt.Errorf("unsupported billing resource %q", kind)
	}
}

func (a *billingCatalogSDK) Create(
	ctx context.Context,
	kind BillingKind,
	fields map[string]any,
) (BillingObject, error) {
	if _, err := NormalizeBillingFields(kind, fields, false); err != nil {
		return nil, err
	}
	switch kind {
	case BillingTaxCode:
		var req c.CreateTaxCodeRequest
		if err := DecodeBillingRequest(fields, &req); err != nil {
			return nil, err
		}
		resp, err := a.sdk.OpenMeterTax.CreateTaxCode(ctx, req)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.BillingTaxCode == nil {
			return nil, fmt.Errorf("create %s response missing data", kind)
		}
		return billingObject(resp.BillingTaxCode)
	case BillingCurrency:
		var req c.CreateCurrencyCustomRequest
		if err := DecodeBillingRequest(fields, &req); err != nil {
			return nil, err
		}
		resp, err := a.sdk.OpenMeterCurrencies.CreateCustomCurrency(ctx, req)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.BillingCurrencyCustom == nil {
			return nil, fmt.Errorf("create %s response missing data", kind)
		}
		return billingObject(resp.BillingCurrencyCustom)
	case BillingApp:
		install := maps.Clone(fields)
		env, _ := install["api_key_env"].(string)
		delete(install, "api_key_env")
		if env != "" {
			secret, ok := os.LookupEnv(env)
			if !ok || secret == "" {
				return nil, fmt.Errorf("billing app credential environment variable %q is empty", env)
			}
			install["api_key"] = secret
		}
		for _, key := range []string{"labels", "description", "enable_draft_sync_hook", "enable_issuing_sync_hook"} {
			delete(install, key)
		}
		var req c.BillingInstallAppRequest
		if err := DecodeBillingRequest(install, &req); err != nil {
			return nil, err
		}
		resp, err := a.sdk.OpenMeterApps.InstallApp(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("install billing app failed; check credentials and API connectivity")
		}
		if resp == nil || resp.BillingInstallAppResponse == nil {
			return nil, fmt.Errorf("install app response missing data")
		}
		app, err := billingObject(resp.BillingInstallAppResponse)
		if err != nil {
			return nil, err
		}
		update := map[string]any{"name": fields["name"], "type": fields["type"]}
		for _, key := range []string{"labels", "description", "enable_draft_sync_hook", "enable_issuing_sync_hook"} {
			if value, ok := fields[key]; ok {
				update[key] = value
			}
		}
		if _, needsLabels := fields["labels"]; needsLabels || fields["description"] != nil ||
			fields["enable_draft_sync_hook"] != nil || fields["enable_issuing_sync_hook"] != nil {
			updated, err := a.Update(ctx, kind, app.ID(), update)
			if err != nil {
				return nil, fmt.Errorf(
					"app %s installed but metadata update failed; adopt this ULID before retrying",
					app.ID(),
				)
			}
			return updated, nil
		}
		return app, nil
	case BillingProfile:
		var req c.CreateBillingProfileRequest
		if err := DecodeBillingRequest(fields, &req); err != nil {
			return nil, err
		}
		resp, err := a.sdk.OpenMeterBillingSettings.CreateBillingProfile(ctx, req)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.BillingProfile == nil {
			return nil, fmt.Errorf("create %s response missing data", kind)
		}
		return billingObject(resp.BillingProfile)
	case BillingPlan:
		var req c.CreatePlanRequest
		if err := DecodeBillingRequest(fields, &req); err != nil {
			return nil, err
		}
		resp, err := a.sdk.OpenMeterProductCatalog.CreatePlan(ctx, req)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.BillingPlan == nil {
			return nil, fmt.Errorf("create %s response missing data", kind)
		}
		return billingObject(resp.BillingPlan)
	default:
		return nil, fmt.Errorf("unsupported billing resource %q", kind)
	}
}

func (a *billingCatalogSDK) Update(
	ctx context.Context,
	kind BillingKind,
	id string,
	fields map[string]any,
) (BillingObject, error) {
	if !IsMeterULID(id) {
		return nil, fmt.Errorf("billing resource id must be an uppercase ULID")
	}
	switch kind {
	case BillingTaxCode:
		var req c.UpsertTaxCodeRequest
		if err := DecodeBillingRequest(fields, &req); err != nil {
			return nil, err
		}
		resp, err := a.sdk.OpenMeterTax.UpsertTaxCode(ctx, id, req)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.BillingTaxCode == nil {
			return nil, fmt.Errorf("update %s response missing data", kind)
		}
		return billingObject(resp.BillingTaxCode)
	case BillingApp:
		var req c.BillingUpdateAppRequest
		if err := DecodeBillingRequest(fields, &req); err != nil {
			return nil, err
		}
		resp, err := a.sdk.OpenMeterApps.UpdateApp(ctx, id, req)
		if err != nil {
			return nil, fmt.Errorf("update billing app failed; check credentials and API connectivity")
		}
		if resp == nil || resp.BillingApp == nil {
			return nil, fmt.Errorf("update %s response missing data", kind)
		}
		return billingObject(resp.BillingApp)
	case BillingProfile:
		var req c.UpsertBillingProfileRequest
		if err := DecodeBillingRequest(fields, &req); err != nil {
			return nil, err
		}
		resp, err := a.sdk.OpenMeterBillingSettings.UpdateBillingProfile(ctx, id, req)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.BillingProfile == nil {
			return nil, fmt.Errorf("update %s response missing data", kind)
		}
		return billingObject(resp.BillingProfile)
	case BillingPlan:
		var req c.UpsertPlanRequest
		if err := DecodeBillingRequest(fields, &req); err != nil {
			return nil, err
		}
		resp, err := a.sdk.OpenMeterProductCatalog.UpdatePlan(ctx, id, req)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.BillingPlan == nil {
			return nil, fmt.Errorf("update %s response missing data", kind)
		}
		return billingObject(resp.BillingPlan)
	case BillingCurrency:
		return nil, fmt.Errorf("currencies cannot be updated")
	default:
		return nil, fmt.Errorf("%s cannot be updated", kind)
	}
}

func (a *billingCatalogSDK) Delete(ctx context.Context, kind BillingKind, id string) error {
	if !IsMeterULID(id) {
		return fmt.Errorf("billing resource id must be an uppercase ULID")
	}
	switch kind {
	case BillingTaxCode:
		_, err := a.sdk.OpenMeterTax.DeleteTaxCode(ctx, id)
		return err
	case BillingApp:
		_, err := a.sdk.OpenMeterApps.UninstallApp(ctx, id)
		return err
	case BillingProfile:
		_, err := a.sdk.OpenMeterBillingSettings.DeleteBillingProfile(ctx, id)
		return err
	case BillingPlan:
		_, err := a.sdk.OpenMeterProductCatalog.DeletePlan(ctx, id)
		return err
	case BillingCurrency:
		return fmt.Errorf("currencies cannot be deleted")
	default:
		return fmt.Errorf("%s cannot be deleted", kind)
	}
}

func (a *billingCatalogSDK) PlanAction(ctx context.Context, id, action string) (BillingObject, error) {
	if !IsMeterULID(id) {
		return nil, fmt.Errorf("plan id must be an uppercase ULID")
	}
	switch action {
	case "publish":
		resp, err := a.sdk.OpenMeterProductCatalog.PublishPlan(ctx, id)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.BillingPlan == nil {
			return nil, fmt.Errorf("publish response missing data")
		}
		return billingObject(resp.BillingPlan)
	case "archive":
		resp, err := a.sdk.OpenMeterProductCatalog.ArchivePlan(ctx, id)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.BillingPlan == nil {
			return nil, fmt.Errorf("archive response missing data")
		}
		return billingObject(resp.BillingPlan)
	default:
		return nil, fmt.Errorf("unsupported plan action %q", action)
	}
}

func (a *billingCatalogSDK) Catalog(ctx context.Context, kind string, size int64) (any, error) {
	if kind != "" {
		resp, err := a.sdk.OpenMeterApps.GetAppCatalogItem(ctx, c.BillingAppType(kind))
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.BillingAppCatalogItem == nil {
			return nil, fmt.Errorf("catalog response missing data")
		}
		return resp.BillingAppCatalogItem, nil
	}
	result := []any{}
	for number := int64(1); number <= 10000; number++ {
		resp, err := a.sdk.OpenMeterApps.ListAppCatalog(ctx, &c.PagePaginationQuery{Size: &size, Number: &number})
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.AppCatalogItemPagePaginatedResponse == nil {
			return nil, fmt.Errorf("catalog response missing data")
		}
		body := resp.AppCatalogItemPagePaginatedResponse
		for _, v := range body.Data {
			result = append(result, v)
		}
		if len(body.Data) == 0 || float64(len(result)) >= body.Meta.Page.Total {
			return result, nil
		}
	}
	return nil, fmt.Errorf("catalog pagination exceeded safety limit")
}

func (a *billingCatalogSDK) CostBases(ctx context.Context, id string, size int64) (any, error) {
	result := []any{}
	for number := int64(1); number <= 10000; number++ {
		resp, err := a.sdk.OpenMeterCurrencies.ListCostBases(
			ctx,
			o.ListCostBasesRequest{CurrencyID: id, Page: &c.PagePaginationQuery{Size: &size, Number: &number}},
		)
		if err != nil {
			return nil, err
		}
		if resp == nil || resp.CostBasisPagePaginatedResponse == nil {
			return nil, fmt.Errorf("cost basis response missing data")
		}
		body := resp.CostBasisPagePaginatedResponse
		for _, v := range body.Data {
			result = append(result, v)
		}
		if len(body.Data) == 0 || float64(len(result)) >= body.Meta.Page.Total {
			return result, nil
		}
	}
	return nil, fmt.Errorf("cost basis pagination exceeded safety limit")
}

func (a *billingCatalogSDK) CreateCostBasis(ctx context.Context, id string, req c.CreateCostBasisRequest) (any, error) {
	if strings.TrimSpace(req.FiatCode) == "" || strings.TrimSpace(req.Rate) == "" {
		return nil, fmt.Errorf("fiat_code and rate are required")
	}
	resp, err := a.sdk.OpenMeterCurrencies.CreateCostBasis(ctx, id, req)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.BillingCostBasis == nil {
		return nil, fmt.Errorf("cost basis response missing data")
	}
	return resp.BillingCostBasis, nil
}

// ValidateBillingFields validates presence separately from SDK zero values.
func ValidateBillingFields(kind BillingKind, fields map[string]any, update bool) error {
	required := []string{"name"}
	switch kind {
	case BillingTaxCode:
		required = append(required, "app_mappings")
		if !update {
			required = append(required, "key")
		}
	case BillingCurrency:
		required = append(required, "code", "precision", "decimal_mark", "thousand_separator")
	case BillingApp:
		required = append(required, "type")
		if !update {
			required = append(required, "create_billing_profile")
		}
		if fields["type"] == "stripe" && !update && fields["api_key"] == nil && fields["api_key_env"] == nil {
			return fmt.Errorf("stripe api_key_env is required (or api_key in imperative request files)")
		}
	case BillingProfile:
		required = append(required, "supplier", "workflow", "default")
		if !update {
			required = append(required, "apps")
		}
	case BillingPlan:
		required = append(required, "phases")
		if !update {
			required = append(required, "key", "currency", "billing_cadence")
		}
	default:
		return fmt.Errorf("unsupported billing resource %q", kind)
	}
	for _, field := range required {
		value, present := fields[field]
		if !present || value == nil {
			return fmt.Errorf("%s is required", field)
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			return fmt.Errorf("%s must not be empty", field)
		}
	}
	if kind == BillingPlan {
		phases, ok := fields["phases"].([]any)
		if !ok || len(phases) == 0 {
			return fmt.Errorf("at least one plan phase is required")
		}
	}
	if kind == BillingCurrency {
		precision, ok := fields["precision"].(float64)
		if !ok || precision < 0 || precision > 4294967295 || precision != float64(int64(precision)) {
			return fmt.Errorf("precision must be an integer between 0 and 4294967295")
		}
	}
	if kind == BillingCurrency {
		for _, key := range []string{"decimal_mark", "thousand_separator"} {
			value, _ := fields[key].(string)
			if utf8.RuneCountInString(value) != 1 {
				return fmt.Errorf("%s must contain one character", key)
			}
		}
		code, _ := fields["code"].(string)
		if n := utf8.RuneCountInString(code); n < 4 || n > 24 {
			return fmt.Errorf("custom currency code must contain 4 to 24 characters")
		}
	}
	_, err := NormalizeBillingFields(kind, fields, update)
	return err
}

// ValidateBillingRequired checks native SDK JSON contracts recursively, including
// the selected member of pricing and entitlement unions.
func ValidateBillingRequired(input map[string]any, target any) error {
	return billingRequired(reflect.ValueOf(target), input, "")
}

func billingRequired(value reflect.Value, input any, path string) error {
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	if !value.IsValid() {
		return nil
	}
	if value.Kind() == reflect.Slice {
		items, _ := input.([]any)
		for i := 0; i < value.Len() && i < len(items); i++ {
			if err := billingRequired(value.Index(i), items[i], fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
		return nil
	}
	if value.Kind() != reflect.Struct {
		return nil
	}
	fields, ok := input.(map[string]any)
	if !ok {
		return nil
	}
	typ := value.Type()
	known := map[string]bool{}
	union := false
	for i := range value.NumField() {
		field := typ.Field(i)
		if field.PkgPath != "" {
			continue
		}
		if field.Tag.Get("union") == "member" {
			union = true
			if err := billingRequired(value.Field(i), input, path); err != nil {
				return err
			}
			continue
		}
		tag := field.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		key, _, _ := strings.Cut(tag, ",")
		known[key] = true
		in, present := fields[key]
		if (!present || in == nil) && !strings.Contains(tag, "omitempty") && field.Tag.Get("default") == "" {
			return fmt.Errorf("%s%s is required", path, key)
		}
		if present {
			if text, ok := in.(string); ok && field.Type.Kind() == reflect.String &&
				!strings.Contains(tag, "omitempty") &&
				field.Tag.Get("default") == "" &&
				strings.TrimSpace(text) == "" {
				return fmt.Errorf("%s%s must not be empty", path, key)
			}
			if err := billingRequired(value.Field(i), in, path+key+"."); err != nil {
				return err
			}
		}
	}
	if !union {
		for key := range fields {
			if !known[key] {
				return fmt.Errorf("unsupported billing request field %s%s", path, key)
			}
		}
	}
	return nil
}

// BillingConfiguration strips response-only fields through the corresponding
// native request type. It also removes server-resolved prices and app summaries
// while preserving their requestable configuration and reference IDs.
func BillingConfiguration(kind BillingKind, obj BillingObject) (map[string]any, error) {
	var target any
	switch kind {
	case BillingTaxCode:
		target = &c.CreateTaxCodeRequest{}
	case BillingCurrency:
		target = &c.CreateCurrencyCustomRequest{}
	case BillingProfile:
		target = &c.CreateBillingProfileRequest{}
	case BillingPlan:
		target = &c.CreatePlanRequest{}
	case BillingApp:
		result := map[string]any{}
		for _, key := range []string{
			"name",
			"type",
			"description",
			"labels",
			"enable_draft_sync_hook",
			"enable_issuing_sync_hook",
		} {
			if value, ok := obj[key]; ok {
				result[key] = value
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported billing resource %q", kind)
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(data, target); err != nil {
		return nil, fmt.Errorf("invalid billing response configuration")
	}
	return billingObject(target)
}

func billingCurrenciesRequest(page *c.PagePaginationQuery, settings BillingListOptions) o.ListCurrenciesRequest {
	request := o.ListCurrenciesRequest{Page: page}
	if settings.CurrencyType != "" {
		kind := c.BillingCurrencyType(settings.CurrencyType)
		request.Filter = &c.ListCurrenciesParamsFilter{Type: &kind}
	}
	if settings.ExpandCostBasis {
		request.Expand = []c.BillingCurrencyExpand{c.BillingCurrencyExpandCostBasis}
	}
	return request
}
