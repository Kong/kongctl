package resources

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kong/kongctl/internal/declarative/tags"
)

const SchemaFieldAICostBudget = "budget"

// AIResource is a manifest grouping. It does not represent an API resource.
type AIResource struct {
	CostManagement *AICostManagement `yaml:"cost_management,omitempty" json:"cost_management,omitempty"`
}

type AICostManagement struct {
	Budgets              []AICostBudget   `yaml:"budgets,omitempty"               json:"budgets,omitempty"`
	NotificationChannels []AICostChannel  `yaml:"notification_channels,omitempty" json:"notification_channels,omitempty"`
	ExportSchedules      []AICostSchedule `yaml:"export_schedules,omitempty"      json:"export_schedules,omitempty"`
	NotificationRules    []AICostRule     `yaml:"notification_rules,omitempty"    json:"notification_rules,omitempty"`
}

type AICostScope struct {
	Filters []map[string]any `yaml:"filters"        json:"filters"`
	Each    *string          `yaml:"each,omitempty" json:"each,omitempty"`
}

type AICostWindow struct {
	Type       string `yaml:"type"        json:"type"`
	Interval   string `yaml:"interval"    json:"interval"`
	AnchoredAt string `yaml:"anchored_at" json:"anchored_at"`
}

type AICostMoney struct {
	Value    string `yaml:"value"    json:"value"`
	Currency string `yaml:"currency" json:"currency"`
}

type AICostValidity struct {
	StartsAt *string `yaml:"starts_at,omitempty" json:"starts_at,omitempty"`
	EndsAt   string  `yaml:"ends_at"             json:"ends_at"`
}

type AICostBudget struct {
	BaseResource       `               yaml:",inline"                     json:",inline"`
	External           *ExternalBlock `yaml:"_external,omitempty"         json:"_external,omitempty"`
	AICostBudgetConfig `               yaml:",inline"                     json:",inline"`
	NotificationRule   *AICostRule `yaml:"notification_rule,omitempty" json:"notification_rule,omitempty"`
}

type AICostBudgetConfig struct {
	Name     string             `yaml:"name,omitempty"     json:"name,omitempty"`
	Effect   string             `yaml:"effect,omitempty"   json:"effect,omitempty"`
	Scope    *AICostScope       `yaml:"scope,omitempty"    json:"scope,omitempty"`
	Limit    *AICostMoney       `yaml:"limit,omitempty"    json:"limit,omitempty"`
	Window   *AICostWindow      `yaml:"window,omitempty"   json:"window,omitempty"`
	Posture  *string            `yaml:"posture,omitempty"  json:"posture,omitempty"`
	Priority *int64             `yaml:"priority,omitempty" json:"priority,omitempty"`
	Validity *AICostValidity    `yaml:"validity,omitempty" json:"validity,omitempty"`
	Labels   *map[string]string `yaml:"labels,omitempty"   json:"labels,omitempty"`
}

type AICostEmail struct {
	Recipients []string `yaml:"recipients" json:"recipients"`
}

type AICostSlack struct {
	WebhookURL *string `yaml:"webhook_url,omitempty" json:"webhook_url,omitempty"`
}

type AICostChannel struct {
	BaseResource        `               yaml:",inline"             json:",inline"`
	External            *ExternalBlock `yaml:"_external,omitempty" json:"_external,omitempty"`
	AICostChannelConfig `               yaml:",inline"             json:",inline"`
}

type AICostChannelConfig struct {
	Name   string             `yaml:"name,omitempty"   json:"name,omitempty"`
	Type   string             `yaml:"type,omitempty"   json:"type,omitempty"`
	Email  *AICostEmail       `yaml:"email,omitempty"  json:"email,omitempty"`
	Slack  *AICostSlack       `yaml:"slack,omitempty"  json:"slack,omitempty"`
	Labels *map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

type AICostSchedule struct {
	BaseResource         `yaml:",inline" json:",inline"`
	AICostScheduleConfig `yaml:",inline" json:",inline"`
}

type AICostScheduleConfig struct {
	Name          string             `yaml:"name"                     json:"name"`
	Cadence       string             `yaml:"cadence"                  json:"cadence"`
	TimeZone      *string            `yaml:"time_zone,omitempty"      json:"time_zone,omitempty"`
	Status        *string            `yaml:"status,omitempty"         json:"status,omitempty"`
	Dataset       string             `yaml:"dataset"                  json:"dataset"`
	Format        string             `yaml:"format"                   json:"format"`
	Compression   string             `yaml:"compression"              json:"compression"`
	SchemaVersion *int64             `yaml:"schema_version,omitempty" json:"schema_version,omitempty"`
	Focus         map[string]any     `yaml:"focus,omitempty"          json:"focus,omitempty"`
	Filters       *map[string]any    `yaml:"filters,omitempty"        json:"filters,omitempty"`
	Labels        *map[string]string `yaml:"labels,omitempty"         json:"labels,omitempty"`
}

type AICostRule struct {
	BaseResource     `       yaml:",inline"          json:",inline"`
	Budget           string `yaml:"budget,omitempty" json:"budget,omitempty"`
	AICostRuleConfig `       yaml:",inline"          json:",inline"`
}

type AICostThreshold struct {
	Percent int64 `yaml:"percent" json:"percent"`
}

type AICostChannelRef struct {
	ID string `yaml:"id" json:"id"`
}

type AICostMessage struct {
	Subject string `yaml:"subject" json:"subject"`
	Body    string `yaml:"body"    json:"body"`
}

type AICostRuleConfig struct {
	Thresholds []AICostThreshold  `yaml:"thresholds"         json:"thresholds"`
	Channels   []AICostChannelRef `yaml:"channels,omitempty" json:"channels"`
	Message    *AICostMessage     `yaml:"message,omitempty"  json:"message"`
}

func (rs *ResourceSet) CostManagement() *AICostManagement {
	if rs.AI == nil {
		return nil
	}
	return rs.AI.CostManagement
}

func (rs *ResourceSet) EnsureCostManagement() *AICostManagement {
	if rs.AI == nil {
		rs.AI = &AIResource{}
	}
	if rs.AI.CostManagement == nil {
		rs.AI.CostManagement = &AICostManagement{}
	}
	return rs.AI.CostManagement
}

func aiCostSlice[R any](get func(*AICostManagement) *[]R) (func(*ResourceSet) *[]R, func(*ResourceSet) *[]R) {
	return func(rs *ResourceSet) *[]R {
		if cost := rs.CostManagement(); cost != nil {
			return get(cost)
		}
		return nil
	}, func(rs *ResourceSet) *[]R { return get(rs.EnsureCostManagement()) }
}

func aiCostNamespace(kind ResourceType, base *BaseResource, external bool) NamespaceParticipant {
	return NamespaceParticipant{
		Ref:               base.Ref,
		Label:             string(kind),
		Meta:              &base.Kongctl,
		SupportsProtected: true,
		External:          external,
	}
}

func init() {
	budget, ensureBudget := aiCostSlice(func(cost *AICostManagement) *[]AICostBudget { return &cost.Budgets })
	registerExternalResourceTypeWithSliceAccessors(
		ResourceTypeAICostBudget, budget, ensureBudget,
		AutoExplain[AICostBudget](WithExplainAliases("ai.cost_management.budgets")),
		ExternalResolutionRegistration{Selectors: []string{SchemaFieldID}},
		WithNamespace(260, func(r *AICostBudget) NamespaceParticipant {
			return aiCostNamespace(r.GetType(), &r.BaseResource, r.IsExternal())
		}),
		WithManagedBy(ManagedByPolicy{MaxEntries: 5, OwnsLifecycle: true, PersistsRef: true}),
		WithRootSyncScope(),
		withCollectionValidation(271, aiCostCollectionValidation[AICostBudget]),
	)
	channel, ensureChannel := aiCostSlice(
		func(cost *AICostManagement) *[]AICostChannel { return &cost.NotificationChannels },
	)
	registerExternalResourceTypeWithSliceAccessors(
		ResourceTypeAICostChannel, channel, ensureChannel,
		AutoExplain[AICostChannel](WithExplainAliases("ai.cost_management.notification_channels")),
		ExternalResolutionRegistration{Selectors: []string{SchemaFieldID}},
		WithNamespace(261, func(r *AICostChannel) NamespaceParticipant {
			return aiCostNamespace(r.GetType(), &r.BaseResource, r.IsExternal())
		}),
		WithManagedBy(ManagedByPolicy{MaxEntries: 5, OwnsLifecycle: true, PersistsRef: true}),
		WithRootSyncScope(),
		withCollectionValidation(272, aiCostCollectionValidation[AICostChannel]),
	)
	schedule, ensureSchedule := aiCostSlice(
		func(cost *AICostManagement) *[]AICostSchedule { return &cost.ExportSchedules },
	)
	registerResourceTypeWithSliceAccessors(
		ResourceTypeAICostSchedule, schedule, ensureSchedule,
		AutoExplain[AICostSchedule](WithExplainAliases("ai.cost_management.export_schedules")),
		WithNamespace(262, func(r *AICostSchedule) NamespaceParticipant {
			return aiCostNamespace(r.GetType(), &r.BaseResource, r.IsExternal())
		}),
		WithManagedBy(ManagedByPolicy{MaxEntries: 5, OwnsLifecycle: true, PersistsRef: true}),
		WithRootSyncScope(),
		withCollectionValidation(273, aiCostCollectionValidation[AICostSchedule]),
	)
	rule, ensureRule := aiCostSlice(func(cost *AICostManagement) *[]AICostRule { return &cost.NotificationRules })
	registerResourceTypeWithSliceAccessors(
		ResourceTypeAICostRule,
		rule,
		ensureRule,
		AutoExplain[AICostRule](WithExplainAliases("ai.cost_management.notification_rules")),
		WithNamespace(263, func(r *AICostRule) NamespaceParticipant {
			return aiCostNamespace(r.GetType(), &r.BaseResource, r.IsExternal())
		}),
		WithManagedBy(ManagedByPolicy{MaxEntries: 5, OwnsLifecycle: true}),
		withCollectionValidation(274, aiCostCollectionValidation[AICostRule]),
		WithChildSyncScope(
			ResourceTypeAICostBudget,
			WithEmptyRootCollectionError(170, "each rule must select a budget"),
		),
	)
}

// Names can repeat. Validate each declaration without enforcing name uniqueness.
func aiCostCollectionValidation[R any, P interface {
	*R
	Resource
}](rs *ResourceSet) func(*R) error {
	return func(value *R) error { return validateCollectionResource(rs, P(value)) }
}

func (r AICostBudget) GetType() ResourceType               { return ResourceTypeAICostBudget }
func (r AICostBudget) GetMoniker() string                  { return r.Name }
func (r AICostBudget) Protected() bool                     { return r.Kongctl.IsProtected() }
func (r *AICostBudget) SetDefaults()                       {}
func (r *AICostBudget) TryMatchKonnectResource(_ any) bool { return false }
func (r AICostBudget) GetKonnectMonikerFilter() string     { return "" }
func (r AICostBudget) CostConfig() any                     { return r.AICostBudgetConfig }

func (r AICostBudget) GetDependencies() []ResourceRef { return nil }

func (r *AICostBudget) GetExternalBlock() *ExternalBlock { return r.External }
func (r *AICostBudget) IsExternal() bool                 { return r.External != nil }

func (r AICostChannel) GetType() ResourceType               { return ResourceTypeAICostChannel }
func (r AICostChannel) GetMoniker() string                  { return r.Name }
func (r AICostChannel) Protected() bool                     { return r.Kongctl.IsProtected() }
func (r *AICostChannel) SetDefaults()                       {}
func (r *AICostChannel) TryMatchKonnectResource(_ any) bool { return false }
func (r AICostChannel) GetKonnectMonikerFilter() string     { return "" }
func (r AICostChannel) CostConfig() any                     { return r.AICostChannelConfig }

func (r AICostChannel) GetDependencies() []ResourceRef { return nil }

func (r *AICostChannel) GetExternalBlock() *ExternalBlock { return r.External }
func (r *AICostChannel) IsExternal() bool                 { return r.External != nil }

func (r AICostSchedule) GetType() ResourceType               { return ResourceTypeAICostSchedule }
func (r AICostSchedule) GetMoniker() string                  { return r.Name }
func (r AICostSchedule) Protected() bool                     { return r.Kongctl.IsProtected() }
func (r *AICostSchedule) SetDefaults()                       {}
func (r *AICostSchedule) TryMatchKonnectResource(_ any) bool { return false }
func (r AICostSchedule) GetKonnectMonikerFilter() string     { return "" }
func (r AICostSchedule) CostConfig() any                     { return r.AICostScheduleConfig }

func (r AICostSchedule) GetDependencies() []ResourceRef { return nil }

func (r AICostRule) GetType() ResourceType               { return ResourceTypeAICostRule }
func (r AICostRule) GetMoniker() string                  { return r.Budget }
func (r AICostRule) Protected() bool                     { return r.Kongctl.IsProtected() }
func (r *AICostRule) SetDefaults()                       {}
func (r *AICostRule) TryMatchKonnectResource(_ any) bool { return false }
func (r AICostRule) GetKonnectMonikerFilter() string     { return "" }
func (r AICostRule) CostConfig() any                     { return r.AICostRuleConfig }

func (r AICostRule) GetParentRef() *ResourceRef {
	return &ResourceRef{Kind: ResourceTypeAICostBudget, Ref: NormalizeResourceRef(r.Budget)}
}

func (r AICostRule) GetDependencies() []ResourceRef {
	return []ResourceRef{{Kind: ResourceTypeAICostBudget, Ref: NormalizeResourceRef(r.Budget)}}
}

func validateCostExternal(block *ExternalBlock) error {
	if block.ID == "" {
		return fmt.Errorf("AI Cost Management external references require an explicit UUID")
	}
	return block.Validate()
}

func (r *AICostBudget) Validate() error {
	if err := ValidateRef(r.Ref); err != nil {
		return err
	}
	if r.IsExternal() {
		return validateCostExternal(r.External)
	}
	if r.Name == "" || r.Scope == nil || r.Scope.Filters == nil {
		return fmt.Errorf("budget requires name and scope.filters")
	}
	switch r.Effect {
	case "limit":
		if r.Limit == nil || r.Limit.Value == "" || r.Limit.Currency != "USD" ||
			r.Window == nil || r.Window.Type != "fixed" || r.Window.Interval == "" ||
			r.Window.AnchoredAt == "" || r.Validity != nil {
			return fmt.Errorf("limit budget requires a USD limit and fixed window")
		}
	case "exempt":
		if r.Validity == nil || r.Validity.EndsAt == "" || r.Limit != nil || r.Window != nil || r.Posture != nil ||
			r.Scope.Each != nil {
			return fmt.Errorf(
				"exempt budget requires validity.ends_at and cannot declare limit, window, posture, or scope.each",
			)
		}
	default:
		return fmt.Errorf("budget effect must be limit or exempt")
	}
	return nil
}

func (r *AICostChannel) Validate() error {
	if err := ValidateRef(r.Ref); err != nil {
		return err
	}
	if r.IsExternal() {
		return validateCostExternal(r.External)
	}
	if r.Name == "" {
		return fmt.Errorf("notification channel requires name")
	}
	switch r.Type {
	case "email":
		if r.Email == nil || len(r.Email.Recipients) == 0 || r.Slack != nil {
			return fmt.Errorf("email channel requires email.recipients")
		}
	case "slack":
		if r.Slack == nil || r.Email != nil {
			return fmt.Errorf("a Slack channel requires a slack object")
		}
		if url := r.Slack.WebhookURL; url != nil && !tags.IsSecretPlaceholder(*url) {
			return fmt.Errorf("the Slack webhook must be supplied with !secret")
		}
	default:
		return fmt.Errorf("notification channel type must be email or slack")
	}
	return nil
}

func (r *AICostSchedule) Validate() error {
	if err := ValidateRef(r.Ref); err != nil {
		return err
	}
	if r.Name == "" || r.Cadence == "" || r.Dataset == "" || r.Format == "" || r.Compression == "" {
		return fmt.Errorf("export schedule requires name, cadence, dataset, format, and compression")
	}
	return nil
}

func (r *AICostRule) Validate() error {
	if r.Budget == "" {
		return fmt.Errorf("notification rule requires a budget")
	}
	if len(r.Thresholds) < 1 || len(r.Thresholds) > 10 || len(r.Channels) > 10 {
		return fmt.Errorf("notification rule requires 1–10 thresholds and supports up to 10 channels")
	}
	seen := map[int64]bool{}
	for _, threshold := range r.Thresholds {
		if threshold.Percent < 1 || threshold.Percent > 1000 || seen[threshold.Percent] {
			return fmt.Errorf("notification rule thresholds must be distinct percentages from 1 to 1000")
		}
		seen[threshold.Percent] = true
	}
	return nil
}

func (rs *ResourceSet) NormalizeAICostRules() error {
	cost := rs.CostManagement()
	if cost == nil {
		return nil
	}
	for i := range cost.Budgets {
		budget := &cost.Budgets[i]
		if budget.NotificationRule == nil {
			continue
		}
		rule := *budget.NotificationRule
		if rule.Budget != "" {
			return fmt.Errorf("nested notification rule for %q cannot declare budget", budget.Ref)
		}
		rule.Budget = budget.Ref
		cost.NotificationRules = append(cost.NotificationRules, rule)
		budget.NotificationRule = nil
	}
	for i := range cost.NotificationRules {
		rule := &cost.NotificationRules[i]
		if rule.Ref == "" {
			rule.Ref = "notification-rule-" + NormalizeResourceRef(rule.Budget)
		}
	}
	return nil
}

func (r *AICostChannel) MutablePayloadMap() (map[string]any, error) { return AICostFields(r) }

func (s *SyncScope) addAICostNamespace(kind ResourceType, namespace string) {
	if s.AICostNamespaces == nil {
		s.AICostNamespaces = make(map[ResourceType]map[string]struct{})
	}
	if s.AICostNamespaces[kind] == nil {
		s.AICostNamespaces[kind] = make(map[string]struct{})
	}
	s.AICostNamespaces[kind][namespace] = struct{}{}
}

func (rs *ResourceSet) CaptureAICostNamespaces(defaultNamespace string) {
	if rs.CostManagement() == nil {
		return
	}
	scope := rs.EnsureSyncScope()
	for _, kind := range []ResourceType{ResourceTypeAICostBudget, ResourceTypeAICostChannel, ResourceTypeAICostSchedule} {
		declared := 0
		for _, item := range rs.AllResources() {
			if item.GetType() != kind {
				continue
			}
			declared++
			cost := item.(AICostResource)
			if !cost.IsExternal() {
				scope.addAICostNamespace(kind, GetNamespace(cost.CostMeta()))
			}
		}
		if scope.RootInScope(kind) && declared == 0 {
			scope.addAICostNamespace(kind, defaultNamespace)
		}
	}
}

func (r AICostBudget) CostMeta() *KongctlMeta   { return r.Kongctl }
func (r AICostChannel) CostMeta() *KongctlMeta  { return r.Kongctl }
func (r AICostSchedule) CostMeta() *KongctlMeta { return r.Kongctl }
func (r AICostRule) CostMeta() *KongctlMeta     { return r.Kongctl }

type AICostResource interface {
	Resource
	SetKonnectID(string)
	Protected() bool
	IsExternal() bool
	CostMeta() *KongctlMeta
	CostConfig() any
}

func AICostFields(r AICostResource) (map[string]any, error) {
	data, err := json.Marshal(r.CostConfig())
	if err != nil {
		return nil, err
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	aiCostDefaults(r.GetType(), fields)
	if _, explicit := fields["focus"]; !explicit &&
		r.GetType() == ResourceTypeAICostSchedule && fields["dataset"] == "usage_events" {
		// A schedule switching away from FOCUS must drop the observed FOCUS options.
		fields["focus"] = nil
	}
	return fields, nil
}

// AICostImmutableFields identifies fields the service refuses to replace.
func AICostImmutableFields(kind ResourceType) []string {
	//exhaustive:ignore
	switch kind {
	case ResourceTypeAICostBudget:
		return []string{"effect"}
	case ResourceTypeAICostChannel:
		return []string{"type"}
	case ResourceTypeAICostSchedule:
		return []string{"cadence", "time_zone"}
	}
	return nil
}

// AICostMatch uses the stored namespace and ref independently of the API name.
func AICostMatch(metadata map[string]string, namespace, ref string) bool {
	return metadata[ManagedByToolKey] == ManagedByTool && metadata[ChildNamespaceKey] == namespace &&
		metadata[ManagedByRefKey] == ref
}

func AICostRuleApplies(effect any) bool { return effect == "limit" }

// These documented replacement defaults express the desired state when the
// manifest omits a field. Schedule status and time zone preserve observed values.
func aiCostDefaults(kind ResourceType, fields map[string]any) {
	if kind != ResourceTypeAICostRule && fields["labels"] == nil {
		fields["labels"] = map[string]any{}
	}
	//exhaustive:ignore
	switch kind {
	case ResourceTypeAICostBudget:
		if fields["priority"] == nil {
			fields["priority"] = float64(100)
		}
		if AICostRuleApplies(fields["effect"]) && fields["posture"] == nil {
			fields["posture"] = "advisory"
		}
	case ResourceTypeAICostSchedule:
		if fields["filters"] == nil {
			fields["filters"] = map[string]any{}
		}
		if fields["schema_version"] == nil {
			fields["schema_version"] = float64(1)
		}
	}
}

// AICostComparable normalizes values the service canonicalizes on writes.
func AICostComparable(kind ResourceType, fields map[string]any) {
	aiCostDefaults(kind, fields)
	if name, ok := fields["name"].(string); ok {
		fields["name"] = strings.TrimSpace(name)
	}
	if kind == ResourceTypeAICostBudget {
		if limit, ok := fields["limit"].(map[string]any); ok {
			if value, ok := limit["value"].(string); ok && strings.Contains(value, ".") {
				limit["value"] = strings.TrimRight(strings.TrimRight(value, "0"), ".")
			}
		}
	}
	if kind != ResourceTypeAICostRule {
		return
	}
	for key, field := range map[string]string{"channels": "id", "thresholds": "percent"} {
		list, _ := fields[key].([]any)
		if len(list) == 0 {
			fields[key] = []any{}
			continue
		}
		slices.SortFunc(list, func(a, b any) int {
			left, _ := a.(map[string]any)
			right, _ := b.(map[string]any)
			return strings.Compare(fmt.Sprint(left[field]), fmt.Sprint(right[field]))
		})
	}
}

// AICostWriteFields projects an API response onto the fields accepted on PUT.
func AICostWriteFields(kind ResourceType, data map[string]any) map[string]any {
	fields := make(map[string]any)
	for key, value := range data {
		if value != nil && AICostWriteField(kind, key) {
			fields[key] = value
		}
	}
	if kind == ResourceTypeAICostChannel && fields["type"] == "slack" {
		slack, _ := fields["slack"].(map[string]any)
		slack = maps.Clone(slack)
		delete(slack, "webhook_url")
		if slack == nil {
			slack = map[string]any{}
		}
		fields["slack"] = slack
	}
	return fields
}

func AICostWriteField(kind ResourceType, key string) bool {
	switch key {
	case "managed_by":
		return true
	case "name", "labels":
		return kind != ResourceTypeAICostRule
	}
	//exhaustive:ignore
	switch kind {
	case ResourceTypeAICostBudget:
		return slices.Contains([]string{"effect", "scope", "limit", "window", "posture", "priority", "validity"}, key)
	case ResourceTypeAICostChannel:
		return slices.Contains([]string{"type", "email", "slack"}, key)
	case ResourceTypeAICostSchedule:
		return slices.Contains(
			[]string{
				"cadence",
				"time_zone",
				"status",
				"dataset",
				"format",
				"compression",
				"schema_version",
				"focus",
				"filters",
			},
			key,
		)
	case ResourceTypeAICostRule:
		return slices.Contains([]string{"thresholds", "channels", "message"}, key)
	}
	return false
}
