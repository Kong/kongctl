package resources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"strconv"
	"strings"

	c "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/maturity"
)

// BillingResource retains native API fields while exposing shared identity and
// ownership information to the scoped catalog reconciler.
type BillingResource interface {
	Resource
	BillingID() string
	BillingMeta() *KongctlMeta
	BillingFields() (map[string]any, error)
}

func billingFields(value any) (map[string]any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var fields map[string]any
	if err = json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func validateBillingResource(r BillingResource) error {
	if err := ValidateRef(r.GetRef()); err != nil {
		return err
	}
	if r.BillingID() != "" && !helpers.IsMeterULID(r.BillingID()) {
		return fmt.Errorf("%s id must be an uppercase ULID", r.GetType())
	}
	fields, err := r.BillingFields()
	if err != nil {
		return err
	}
	for key := range helpers.BillingObject(fields).Labels() {
		if labels.IsKongctlLabel(key) {
			return fmt.Errorf("management labels belong in kongctl, not labels")
		}
	}
	kind := BillingKindForResourceType(r.GetType())
	if kind == helpers.BillingApp && r.BillingID() != "" && fields["type"] == "stripe" && fields["api_key_env"] == nil {
		fields["api_key_env"] = "OBSERVE_ONLY"
	}
	if err := helpers.ValidateBillingFields(kind, fields, false); err != nil {
		return fmt.Errorf("%s %q: %w", r.GetType(), r.GetRef(), err)
	}
	if kind == helpers.BillingCurrency && r.BillingMeta() != nil && r.BillingMeta().Protected != nil {
		return fmt.Errorf("currency ownership and protection labels are unsupported")
	}
	if kind != helpers.BillingCurrency {
		return helpers.ValidateMeterLabels(
			labels.BuildCreateLabels(helpers.BillingObject(fields).Labels(), GetNamespace(r.BillingMeta()), false),
		)
	}
	return nil
}

func billingCollectionValidation[R any](rs *ResourceSet) func(*R) error {
	refs, keys, ids := map[string]bool{}, map[string]bool{}, map[string]bool{}
	return func(value *R) error {
		r := any(value).(BillingResource)
		if err := validateCollectionResource(rs, r); err != nil {
			return err
		}
		fields, err := r.BillingFields()
		if err != nil {
			return err
		}
		for path, kind := range BillingReferenceTargets(r.GetType(), fields) {
			value, _, err := BillingPointer(fields, path)
			if err != nil {
				return err
			}
			text, _ := value.(string)
			if helpers.IsMeterULID(text) {
				if kind == ResourceTypeCurrency {
					return fmt.Errorf("currency fields require a code, not a ULID")
				}
				continue
			}
			if tags.IsExternalPlaceholder(text) {
				return fmt.Errorf(
					"%s requires a declared %s ref or explicit remote identifier; dynamic billing lookups are unsupported",
					path,
					kind,
				)
			}
			tagged := tags.IsRefPlaceholder(text)
			if tagged {
				ref, selector, ok := tags.ParseRefPlaceholder(text)
				expected := "id"
				if kind == ResourceTypeCurrency {
					expected = "code"
				}
				if !ok || selector != expected {
					return fmt.Errorf("%s must select %s #%s", path, kind, expected)
				}
				text = ref
			}
			target, ok := rs.GetResourceByRef(text)
			if !ok && kind == ResourceTypeCurrency && !tagged {
				continue
			}
			if !ok || target.GetType() != kind {
				return fmt.Errorf(
					"%s %q field %s must reference a declared %s or remote identifier",
					r.GetType(),
					r.GetRef(),
					path,
					kind,
				)
			}
		}
		key := r.GetMoniker()
		if refs[r.GetRef()] {
			return fmt.Errorf("duplicate %s ref %q", r.GetType(), r.GetRef())
		}
		if keys[key] {
			return fmt.Errorf("duplicate %s identity %q", r.GetType(), key)
		}
		if r.BillingID() != "" && ids[r.BillingID()] {
			return fmt.Errorf("duplicate %s id %q", r.GetType(), r.BillingID())
		}
		refs[r.GetRef()], keys[key], ids[r.BillingID()] = true, true, true
		return nil
	}
}

// TaxCodeResource models tax_codes without introducing implicit API defaults.
type TaxCodeResource struct {
	BaseResource
	ID          string                       `json:"id,omitempty" yaml:"id,omitempty"`
	Name        string                       `json:"name" yaml:"name"`
	Description *string                      `json:"description,omitempty" yaml:"description,omitempty"`
	Labels      map[string]string            `json:"labels,omitempty" yaml:"labels,omitempty"`
	Key         string                       `json:"key" yaml:"key"`
	AppMappings []c.BillingTaxCodeAppMapping `json:"app_mappings" yaml:"app_mappings"`
}

func (r TaxCodeResource) GetType() ResourceType     { return ResourceTypeTaxCode }
func (r TaxCodeResource) GetMoniker() string        { return r.Key }
func (r TaxCodeResource) BillingID() string         { return r.ID }
func (r TaxCodeResource) BillingMeta() *KongctlMeta { return r.Kongctl }
func (r TaxCodeResource) BillingFields() (map[string]any, error) {
	return billingFields(
		c.CreateTaxCodeRequest{
			Name:        r.Name,
			Description: r.Description,
			Labels:      r.Labels,
			Key:         r.Key,
			AppMappings: r.AppMappings,
		},
	)
}
func (r *TaxCodeResource) SetDefaults()                   {}
func (r TaxCodeResource) GetDependencies() []ResourceRef  { return nil }
func (r TaxCodeResource) GetKonnectMonikerFilter() string { return r.GetMoniker() }
func (r *TaxCodeResource) TryMatchKonnectResource(value any) bool {
	if r.ID != "" {
		if id := tryMatchByField(value, "ID", r.ID); id != "" {
			r.SetKonnectID(id)
			return true
		}
		return false
	}
	return false
}
func (r TaxCodeResource) Validate() error                { return validateBillingResource(&r) }
func (r TaxCodeResource) GetLabels() map[string]string   { return r.Labels }
func (r *TaxCodeResource) SetLabels(v map[string]string) { r.Labels = v }
func init() {
	registerResourceType(
		ResourceTypeTaxCode,
		func(rs *ResourceSet) *[]TaxCodeResource { return &rs.TaxCodes },
		AutoExplain[TaxCodeResource](
			WithExplainAliases("tax_codes"),
			withBillingRequiredExplain[TaxCodeResource]("ref", "name", "key", "app_mappings"),
			WithExplainFieldHint(
				"app_mappings.app_type",
				ExplainFieldHint{Literal: "sandbox", Enum: []any{"sandbox", "stripe", "external_invoicing"}},
			),
		),
		WithNamespace(150, func(r *TaxCodeResource) NamespaceParticipant {
			return NamespaceParticipant{Ref: r.Ref, Label: "tax_code", SupportsProtected: true, Meta: &r.Kongctl}
		}),
		WithRootSyncScope(),
		WithExternalUnsupportedReason(
			"billing targets accept declared refs and explicit remote ULIDs; dynamic external name lookup is ambiguous",
		),
		withCollectionValidation(210, billingCollectionValidation[TaxCodeResource]),
	)
}

// CurrencyResource models currencies without introducing implicit API defaults.
type CurrencyResource struct {
	BaseResource
	ID                string  `json:"id,omitempty" yaml:"id,omitempty"`
	Name              string  `json:"name" yaml:"name"`
	Symbol            *string `json:"symbol,omitempty" yaml:"symbol,omitempty"`
	Precision         *int64  `json:"precision" yaml:"precision"`
	DecimalMark       *string `json:"decimal_mark" yaml:"decimal_mark"`
	ThousandSeparator *string `json:"thousand_separator" yaml:"thousand_separator"`
	Code              string  `json:"code" yaml:"code"`
}

func (r CurrencyResource) GetType() ResourceType     { return ResourceTypeCurrency }
func (r CurrencyResource) GetMoniker() string        { return r.Code }
func (r CurrencyResource) BillingID() string         { return r.ID }
func (r CurrencyResource) BillingMeta() *KongctlMeta { return r.Kongctl }
func (r CurrencyResource) BillingFields() (map[string]any, error) {
	if r.Precision == nil || r.DecimalMark == nil || r.ThousandSeparator == nil {
		return nil, fmt.Errorf("precision, decimal_mark and thousand_separator are required")
	}
	return billingFields(
		c.CreateCurrencyCustomRequest{
			Name:              r.Name,
			Symbol:            r.Symbol,
			Precision:         *r.Precision,
			DecimalMark:       *r.DecimalMark,
			ThousandSeparator: *r.ThousandSeparator,
			Code:              r.Code,
		},
	)
}
func (r *CurrencyResource) SetDefaults()                   {}
func (r CurrencyResource) GetDependencies() []ResourceRef  { return nil }
func (r CurrencyResource) GetKonnectMonikerFilter() string { return r.GetMoniker() }
func (r *CurrencyResource) TryMatchKonnectResource(value any) bool {
	if r.ID != "" {
		if id := tryMatchByField(value, "ID", r.ID); id != "" {
			r.SetKonnectID(id)
			return true
		}
		return false
	}
	return false
}
func (r CurrencyResource) Validate() error { return validateBillingResource(&r) }
func init() {
	registerResourceType(
		ResourceTypeCurrency,
		func(rs *ResourceSet) *[]CurrencyResource { return &rs.Currencies },
		AutoExplain[CurrencyResource](
			WithExplainAliases("currencies"),
			withBillingRequiredExplain[CurrencyResource](
				"ref",
				"name",
				"code",
				"precision",
				"decimal_mark",
				"thousand_separator",
			),
			WithExplainFieldHint("code", ExplainFieldHint{Literal: "CREDITS"}),
			WithExplainFieldHint("precision", ExplainFieldHint{Literal: "0"}),
			WithExplainFieldHint("decimal_mark", ExplainFieldHint{Literal: "'.'"}),
			WithExplainFieldHint("thousand_separator", ExplainFieldHint{Literal: "','"}),
		),
		WithNamespace(160, func(r *CurrencyResource) NamespaceParticipant {
			return NamespaceParticipant{Ref: r.Ref, Label: "currency", SupportsProtected: false, Meta: &r.Kongctl}
		}),
		WithRootSyncScope(),
		WithExternalUnsupportedReason(
			"billing targets accept declared refs and explicit remote ULIDs; dynamic external name lookup is ambiguous",
		),
		WithMaturity(maturity.Metadata{Level: maturity.LevelBeta}),
		withCollectionValidation(220, billingCollectionValidation[CurrencyResource]),
	)
}

// BillingAppResource models billing_apps without introducing implicit API defaults.
type BillingAppResource struct {
	BaseResource
	ID                    string            `json:"id,omitempty" yaml:"id,omitempty"`
	Name                  string            `json:"name" yaml:"name"`
	Type                  string            `json:"type" yaml:"type"`
	Description           *string           `json:"description,omitempty" yaml:"description,omitempty"`
	Labels                map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	CreateBillingProfile  *bool             `json:"create_billing_profile" yaml:"create_billing_profile"`
	APIKeyEnv             string            `json:"api_key_env,omitempty" yaml:"api_key_env,omitempty"`
	EnableDraftSyncHook   *bool             `json:"enable_draft_sync_hook,omitempty" yaml:"enable_draft_sync_hook,omitempty"`     //nolint:lll // Native API field tags.
	EnableIssuingSyncHook *bool             `json:"enable_issuing_sync_hook,omitempty" yaml:"enable_issuing_sync_hook,omitempty"` //nolint:lll // Native API field tags.
}

func (r BillingAppResource) GetType() ResourceType     { return ResourceTypeBillingApp }
func (r BillingAppResource) GetMoniker() string        { return r.Name }
func (r BillingAppResource) BillingID() string         { return r.ID }
func (r BillingAppResource) BillingMeta() *KongctlMeta { return r.Kongctl }
func (r BillingAppResource) BillingFields() (map[string]any, error) {
	if r.CreateBillingProfile == nil {
		return nil, fmt.Errorf("create_billing_profile is required")
	}
	fields := map[string]any{"name": r.Name, "type": r.Type, "create_billing_profile": *r.CreateBillingProfile}
	if r.Description != nil {
		fields["description"] = *r.Description
	}
	if len(r.Labels) > 0 {
		fields["labels"] = r.Labels
	}
	if r.APIKeyEnv != "" {
		fields["api_key_env"] = r.APIKeyEnv
	}
	if r.EnableDraftSyncHook != nil {
		fields["enable_draft_sync_hook"] = *r.EnableDraftSyncHook
	}
	if r.EnableIssuingSyncHook != nil {
		fields["enable_issuing_sync_hook"] = *r.EnableIssuingSyncHook
	}
	return billingFields(fields)
}
func (r *BillingAppResource) SetDefaults()                   {}
func (r BillingAppResource) GetDependencies() []ResourceRef  { return nil }
func (r BillingAppResource) GetKonnectMonikerFilter() string { return r.GetMoniker() }
func (r *BillingAppResource) TryMatchKonnectResource(value any) bool {
	if r.ID != "" {
		if id := tryMatchByField(value, "ID", r.ID); id != "" {
			r.SetKonnectID(id)
			return true
		}
		return false
	}
	return false
}

func (r BillingAppResource) Validate() error {
	if r.Type == "external_invoicing" && (r.EnableDraftSyncHook == nil || r.EnableIssuingSyncHook == nil) {
		return fmt.Errorf(
			"managed external_invoicing apps require explicit enable_draft_sync_hook and enable_issuing_sync_hook",
		)
	}

	if r.CreateBillingProfile == nil || *r.CreateBillingProfile {
		return fmt.Errorf(
			"declarative billing apps require create_billing_profile: false; declare billing_profiles explicitly",
		)
	}
	return validateBillingResource(&r)
}
func (r BillingAppResource) GetLabels() map[string]string   { return r.Labels }
func (r *BillingAppResource) SetLabels(v map[string]string) { r.Labels = v }
func init() {
	registerResourceType(
		ResourceTypeBillingApp,
		func(rs *ResourceSet) *[]BillingAppResource { return &rs.BillingApps },
		AutoExplain[BillingAppResource](
			WithExplainAliases("billing_apps"),
			withBillingRequiredExplain[BillingAppResource]("ref", "name", "type", "create_billing_profile"),
			WithExplainFieldHint(
				"type",
				ExplainFieldHint{Literal: "sandbox", Enum: []any{"sandbox", "stripe", "external_invoicing"}},
			),
			WithExplainFieldHint("create_billing_profile", ExplainFieldHint{Literal: "false"}),
		),
		WithNamespace(170, func(r *BillingAppResource) NamespaceParticipant {
			return NamespaceParticipant{Ref: r.Ref, Label: "billing_app", SupportsProtected: true, Meta: &r.Kongctl}
		}),
		WithRootSyncScope(),
		WithExternalUnsupportedReason(
			"billing targets accept declared refs and explicit remote ULIDs; dynamic external name lookup is ambiguous",
		),
		WithOperationMaturity(OperationCreate, maturity.Metadata{Level: maturity.LevelBeta}),
		WithOperationMaturity(OperationUpdate, maturity.Metadata{Level: maturity.LevelBeta}),
		WithOperationMaturity(OperationDelete, maturity.Metadata{Level: maturity.LevelBeta}),
		WithOperationMaturity(OperationAdopt, maturity.Metadata{Level: maturity.LevelBeta}),
		withCollectionValidation(230, billingCollectionValidation[BillingAppResource]),
	)
}

// BillingProfileResource models billing_profiles without introducing implicit API defaults.
type BillingProfileResource struct {
	BaseResource
	ID          string                                 `json:"id,omitempty" yaml:"id,omitempty"`
	Name        string                                 `json:"name" yaml:"name"`
	Description *string                                `json:"description,omitempty" yaml:"description,omitempty"`
	Labels      map[string]string                      `json:"labels,omitempty" yaml:"labels,omitempty"`
	Supplier    *c.CreateBillingProfileRequestSupplier `json:"supplier" yaml:"supplier"`
	Workflow    *c.CreateBillingProfileRequestWorkflow `json:"workflow" yaml:"workflow"`
	Apps        *c.CreateBillingProfileRequestApps     `json:"apps" yaml:"apps"`
	Default     *bool                                  `json:"default" yaml:"default"`
}

func (r BillingProfileResource) GetType() ResourceType     { return ResourceTypeBillingProfile }
func (r BillingProfileResource) GetMoniker() string        { return r.Name }
func (r BillingProfileResource) BillingID() string         { return r.ID }
func (r BillingProfileResource) BillingMeta() *KongctlMeta { return r.Kongctl }
func (r BillingProfileResource) BillingFields() (map[string]any, error) {
	if r.Supplier == nil || r.Workflow == nil || r.Apps == nil || r.Default == nil {
		return nil, fmt.Errorf("supplier, workflow, apps and default are required")
	}
	return billingFields(
		c.CreateBillingProfileRequest{
			Name:        r.Name,
			Description: r.Description,
			Labels:      r.Labels,
			Supplier:    *r.Supplier,
			Workflow:    *r.Workflow,
			Apps:        *r.Apps,
			Default:     r.Default != nil && *r.Default,
		},
	)
}
func (r *BillingProfileResource) SetDefaults()                   {}
func (r BillingProfileResource) GetDependencies() []ResourceRef  { return nil }
func (r BillingProfileResource) GetKonnectMonikerFilter() string { return r.GetMoniker() }
func (r *BillingProfileResource) TryMatchKonnectResource(value any) bool {
	if r.ID != "" {
		if id := tryMatchByField(value, "ID", r.ID); id != "" {
			r.SetKonnectID(id)
			return true
		}
		return false
	}
	return false
}

func (r BillingProfileResource) Validate() error {
	if r.Default == nil {
		return fmt.Errorf("default is required")
	}
	return validateBillingResource(&r)
}
func (r BillingProfileResource) GetLabels() map[string]string   { return r.Labels }
func (r *BillingProfileResource) SetLabels(v map[string]string) { r.Labels = v }
func init() {
	registerResourceType(
		ResourceTypeBillingProfile,
		func(rs *ResourceSet) *[]BillingProfileResource { return &rs.BillingProfiles },
		AutoExplain[BillingProfileResource](
			WithExplainFieldHint("workflow", ExplainFieldHint{Literal: "{}"}),
			WithExplainAliases("billing_profiles"),
			withBillingRequiredExplain[BillingProfileResource](
				"ref",
				"name",
				"supplier",
				"workflow",
				"apps",
				"default",
			),
			WithExplainFieldHint("apps.tax.id", ExplainFieldHint{Literal: "01G65Z755AFWAKHE12NY0C0001"}),
			WithExplainFieldHint("apps.invoicing.id", ExplainFieldHint{Literal: "01G65Z755AFWAKHE12NY0C0001"}),
			WithExplainFieldHint("apps.payment.id", ExplainFieldHint{Literal: "01G65Z755AFWAKHE12NY0C0001"}),
		),
		WithNamespace(180, func(r *BillingProfileResource) NamespaceParticipant {
			return NamespaceParticipant{Ref: r.Ref, Label: "billing_profile", SupportsProtected: true, Meta: &r.Kongctl}
		}),
		WithRootSyncScope(),
		WithExternalUnsupportedReason(
			"billing targets accept declared refs and explicit remote ULIDs; dynamic external name lookup is ambiguous",
		),
		withCollectionValidation(240, billingCollectionValidation[BillingProfileResource]),
	)
}

// BillingPlanResource models billing_plans without introducing implicit API defaults.
type BillingPlanResource struct {
	BaseResource
	ID               string                             `json:"id,omitempty" yaml:"id,omitempty"`
	Name             string                             `json:"name" yaml:"name"`
	Description      *string                            `json:"description,omitempty" yaml:"description,omitempty"`
	Labels           map[string]string                  `json:"labels,omitempty" yaml:"labels,omitempty"`
	Key              string                             `json:"key" yaml:"key"`
	Currency         string                             `json:"currency" yaml:"currency"`
	BillingCadence   string                             `json:"billing_cadence" yaml:"billing_cadence"`
	ProRatingEnabled *bool                              `json:"pro_rating_enabled" yaml:"pro_rating_enabled"`
	Phases           []c.BillingPlanPhase               `json:"phases" yaml:"phases"`
	SettlementMode   *c.CreatePlanRequestSettlementMode `json:"settlement_mode" yaml:"settlement_mode"`
	Version          *int64                             `json:"version,omitempty" yaml:"version,omitempty"`
}

func (r BillingPlanResource) GetType() ResourceType     { return ResourceTypeBillingPlan }
func (r BillingPlanResource) GetMoniker() string        { return billingPlanMoniker(r.Key, r.Version) }
func (r BillingPlanResource) BillingID() string         { return r.ID }
func (r BillingPlanResource) BillingMeta() *KongctlMeta { return r.Kongctl }
func (r BillingPlanResource) BillingFields() (map[string]any, error) {
	return billingFields(
		c.CreatePlanRequest{
			Name:             r.Name,
			Description:      r.Description,
			Labels:           r.Labels,
			Key:              r.Key,
			Currency:         r.Currency,
			BillingCadence:   r.BillingCadence,
			ProRatingEnabled: r.ProRatingEnabled,
			Phases:           r.Phases,
			SettlementMode:   r.SettlementMode,
		},
	)
}
func (r *BillingPlanResource) SetDefaults()                   {}
func (r BillingPlanResource) GetDependencies() []ResourceRef  { return nil }
func (r BillingPlanResource) GetKonnectMonikerFilter() string { return r.GetMoniker() }
func (r *BillingPlanResource) TryMatchKonnectResource(value any) bool {
	if r.ID != "" {
		if id := tryMatchByField(value, "ID", r.ID); id != "" {
			r.SetKonnectID(id)
			return true
		}
		return false
	}
	return false
}

func (r BillingPlanResource) Validate() error {
	if r.Version != nil && *r.Version < 1 {
		return fmt.Errorf("plan version must be positive")
	}
	return validateBillingResource(&r)
}
func (r BillingPlanResource) GetLabels() map[string]string   { return r.Labels }
func (r *BillingPlanResource) SetLabels(v map[string]string) { r.Labels = v }
func init() {
	registerResourceType(
		ResourceTypeBillingPlan,
		func(rs *ResourceSet) *[]BillingPlanResource { return &rs.BillingPlans },
		AutoExplain[BillingPlanResource](
			WithExplainAliases("billing_plans"),
			withBillingRequiredExplain[BillingPlanResource](
				"ref",
				"name",
				"key",
				"currency",
				"billing_cadence",
				"phases",
				"phases.name",
				"phases.key",
				"phases.rate_cards",
				"phases.rate_cards.name",
				"phases.rate_cards.key",
				"phases.rate_cards.price",
			),
			WithExplainFieldHint("currency", ExplainFieldHint{Literal: "USD"}),
			WithExplainFieldHint("billing_cadence", ExplainFieldHint{Literal: "P1M"}),
			WithExplainFieldHint("phases.rate_cards.price.amount", ExplainFieldHint{Literal: "'1.0'"}),
			WithExplainFieldHint(
				"phases.rate_cards.feature.id",
				ExplainFieldHint{Literal: "01G65Z755AFWAKHE12NY0C0001"},
			),
		),
		WithNamespace(190, func(r *BillingPlanResource) NamespaceParticipant {
			return NamespaceParticipant{Ref: r.Ref, Label: "billing_plan", SupportsProtected: true, Meta: &r.Kongctl}
		}),
		WithRootSyncScope(),
		WithExternalUnsupportedReason(
			"billing targets accept declared refs and explicit remote ULIDs; dynamic external name lookup is ambiguous",
		),
		withCollectionValidation(250, billingCollectionValidation[BillingPlanResource]),
	)
}

func billingPlanMoniker(key string, version *int64) string {
	if version == nil {
		return key
	}
	return fmt.Sprintf("%s@%d", key, *version)
}

// BillingResources selects the catalog roots for one namespace, including
// create/observe-only currencies whose omission must never cause deletion.
func (rs *ResourceSet) BillingResources(kind ResourceType, namespace string) []BillingResource {
	result := []BillingResource{}
	rs.ForEachResource(func(r Resource) bool {
		if r.GetType() == kind {
			if b, ok := r.(BillingResource); ok && GetNamespace(b.BillingMeta()) == namespace {
				result = append(result, b)
			}
		}
		return true
	})
	return result
}

// BillingReferenceTargets identifies API-native reference fields. It does not
// mistake provider tax codes, display names, or arbitrary metadata for refs.
func BillingReferenceTargets(kind ResourceType, fields map[string]any) map[string]ResourceType {
	result := map[string]ResourceType{}
	var walk func(any, string)
	walk = func(value any, path string) {
		switch v := value.(type) {
		case map[string]any:
			for k, child := range v {
				next := path + "/" + k
				if k == "id" {
					switch {
					case kind == ResourceTypeBillingProfile && strings.HasPrefix(path, "/apps/"):
						result[next] = ResourceTypeBillingApp
					case strings.HasSuffix(path, "/feature"):
						result[next] = ResourceTypeFeature
					case strings.HasSuffix(path, "/tax_config/code"), strings.HasSuffix(path, "/tax_code"):
						result[next] = ResourceTypeTaxCode
					}
				}
				if k == "tax_code_id" {
					result[next] = ResourceTypeTaxCode
				}
				if k == "currency" && kind == ResourceTypeBillingPlan {
					result[next] = ResourceTypeCurrency
				}
				walk(child, next)
			}
		case []any:
			for i, child := range v {
				walk(child, fmt.Sprintf("%s/%d", path, i))
			}
		}
	}
	walk(fields, "")
	if kind == ResourceTypeBillingPlan {
		result["/currency"] = ResourceTypeCurrency
	}
	return result
}

// billingPointer accesses only paths generated by BillingReferenceTargets.
func BillingPointer(fields map[string]any, path string) (any, func(any), error) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	var parent any = fields
	for i, key := range parts {
		switch value := parent.(type) {
		case map[string]any:
			if i == len(parts)-1 {
				return value[key], func(v any) { value[key] = v }, nil
			}
			parent = value[key]
		case []any:
			index, err := strconv.Atoi(key)
			if err != nil || index < 0 || index >= len(value) {
				return nil, nil, fmt.Errorf("invalid billing reference path %s", path)
			}
			if i == len(parts)-1 {
				return value[index], func(v any) { value[index] = v }, nil
			}
			parent = value[index]
		default:
			return nil, nil, fmt.Errorf("invalid billing reference path %s", path)
		}
	}
	return nil, nil, fmt.Errorf("invalid billing reference path %s", path)
}

// Decode presence before typed SDK unmarshalling fills absent required fields
// with Go zero values. Native optional defaults remain the SDK's responsibility.
func decodeBillingResource(data []byte, target any, kind helpers.BillingKind) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("invalid billing declaration")
	}
	fields := make(map[string]any, len(raw))
	maps.Copy(fields, raw)
	for _, field := range []string{"ref", "id", "kongctl"} {
		delete(fields, field)
	}
	if kind == helpers.BillingPlan {
		delete(fields, "version")
	}
	if kind == helpers.BillingApp && raw["id"] != nil && fields["type"] == "stripe" && fields["api_key_env"] == nil {
		fields["api_key_env"] = "OBSERVE_ONLY"
	}
	if err := helpers.ValidateBillingFields(kind, fields, false); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func (r *TaxCodeResource) UnmarshalJSON(data []byte) error {
	type plain TaxCodeResource
	var value plain
	if err := decodeBillingResource(data, &value, helpers.BillingTaxCode); err != nil {
		return err
	}
	*r = TaxCodeResource(value)
	return nil
}

func (r *CurrencyResource) UnmarshalJSON(data []byte) error {
	type plain CurrencyResource
	var value plain
	if err := decodeBillingResource(data, &value, helpers.BillingCurrency); err != nil {
		return err
	}
	*r = CurrencyResource(value)
	return nil
}

func (r *BillingAppResource) UnmarshalJSON(data []byte) error {
	type plain BillingAppResource
	var value plain
	if err := decodeBillingResource(data, &value, helpers.BillingApp); err != nil {
		return err
	}
	*r = BillingAppResource(value)
	return nil
}

func (r *BillingProfileResource) UnmarshalJSON(data []byte) error {
	type plain BillingProfileResource
	var value plain
	if err := decodeBillingResource(data, &value, helpers.BillingProfile); err != nil {
		return err
	}
	*r = BillingProfileResource(value)
	return nil
}

func (r *BillingPlanResource) UnmarshalJSON(data []byte) error {
	type plain BillingPlanResource
	var value plain
	if err := decodeBillingResource(data, &value, helpers.BillingPlan); err != nil {
		return err
	}
	*r = BillingPlanResource(value)
	return nil
}

// BillingKindForResourceType maps canonical declarative identifiers to SDK API
// collections. Keep API path identifiers out of saved-plan resource types.
func BillingKindForResourceType(kind ResourceType) helpers.BillingKind {
	// This inventory covers the catalog subset only.
	//exhaustive:ignore
	switch kind {
	case ResourceTypeTaxCode:
		return helpers.BillingTaxCode
	case ResourceTypeCurrency:
		return helpers.BillingCurrency
	case ResourceTypeBillingApp:
		return helpers.BillingApp
	case ResourceTypeBillingProfile:
		return helpers.BillingProfile
	case ResourceTypeBillingPlan:
		return helpers.BillingPlan
	default:
		return ""
	}
}

func BillingResourceType(kind helpers.BillingKind) ResourceType {
	switch kind {
	case helpers.BillingTaxCode:
		return ResourceTypeTaxCode
	case helpers.BillingCurrency:
		return ResourceTypeCurrency
	case helpers.BillingApp:
		return ResourceTypeBillingApp
	case helpers.BillingProfile:
		return ResourceTypeBillingProfile
	case helpers.BillingPlan:
		return ResourceTypeBillingPlan
	default:
		return ""
	}
}

func withBillingRequiredExplain[R any](paths ...string) ExplainOption {
	return WithExplainSchemaBuilder(func(ctx ExplainBuildContext) (*ExplainNode, error) {
		hints := defaultExplainHints(ctx.ResourceType)
		maps.Copy(hints, ctx.Registration.fieldHints)
		node, err := autoExplainConcreteNode[R](hints)
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			current := node
			for part := range strings.SplitSeq(path, ".") {
				if current.Kind == explainKindArray {
					current = current.Items
				}
				field, ok := current.property(part)
				if !ok {
					return nil, fmt.Errorf("billing explain field %s not found", path)
				}
				field.Required = true
				field.Node.Nullable = false
				current = field.Node
			}
		}
		return node, nil
	})
}
