package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

// BillingCatalogAdapter uses the shared executor for ownership, protection,
// payload contracts and reference resolution. API-specific request decoding and
// pagination live behind the SDK catalog boundary.
type BillingCatalogAdapter struct {
	client *state.Client
	kind   helpers.BillingKind
}

func (a *BillingCatalogAdapter) ResourceType() string {
	return string(resources.BillingResourceType(a.kind))
}
func (a *BillingCatalogAdapter) RequiredFields() []string { return []string{planner.FieldName} }
func (a *BillingCatalogAdapter) SupportsUpdate() bool     { return a.kind != helpers.BillingCurrency }

func (a *BillingCatalogAdapter) MapCreateFields(
	_ context.Context,
	ctx *ExecutionContext,
	fields map[string]any,
	request *map[string]any,
) error {
	*request = maps.Clone(fields)
	if a.kind != helpers.BillingCurrency {
		(*request)[planner.FieldLabels] = labels.BuildCreateLabels(
			helpers.BillingObject(fields).Labels(),
			ctx.Namespace,
			ctx.Protection,
		)
	}
	if a.kind == helpers.BillingApp {
		for _, field := range []string{"api_key", "secret_api_key"} {
			if _, ok := fields[field]; ok {
				return fmt.Errorf("declarative app credentials must use api_key_env; literal credentials are forbidden")
			}
		}
		// Profiles are declared and reconciled independently of app installation.
		(*request)[planner.FieldCreateBillingProfile] = false
	}
	return helpers.ValidateBillingFields(a.kind, *request, false)
}

func (a *BillingCatalogAdapter) MapUpdateFields(
	_ context.Context,
	_ *ExecutionContext,
	fields map[string]any,
	request *map[string]any,
	_ map[string]string,
) error {
	*request = maps.Clone(fields)
	delete(*request, planner.FieldCurrentLabels)
	if a.kind == helpers.BillingApp {
		if _, ok := fields[planner.FieldAPIKeyEnv]; ok {
			return fmt.Errorf("rotate app credentials explicitly using update mb apps")
		}
	}
	if a.kind == helpers.BillingApp {
		for _, key := range []string{"api_key", "secret_api_key"} {
			if _, ok := fields[key]; ok {
				return fmt.Errorf("rotate app credentials explicitly using update mb apps")
			}
		}
	}
	return helpers.ValidateBillingFields(a.kind, *request, true)
}

func (a *BillingCatalogAdapter) MapUpdateLabels(
	ctx *ExecutionContext,
	request *map[string]any,
	desired, current map[string]string,
) {
	if *request == nil {
		*request = map[string]any{}
	}
	(*request)[planner.FieldLabels] = labels.BuildUpdateStringLabels(desired, current, ctx.Namespace, ctx.Protection)
}

func (a *BillingCatalogAdapter) Create(
	ctx context.Context,
	request map[string]any,
	_ string,
	_ *ExecutionContext,
) (string, error) {
	api, err := a.client.BillingCatalog()
	if err != nil {
		return "", err
	}
	current, err := a.client.ListBilling(ctx, a.kind)
	if err != nil {
		return "", err
	}
	identity := planner.FieldName
	// Billing command/action subset.
	//exhaustive:ignore
	switch a.kind {
	case helpers.BillingTaxCode, helpers.BillingPlan:
		identity = planner.FieldKey
	case helpers.BillingCurrency:
		identity = planner.FieldCode
	}
	for _, value := range current {
		if value[identity] == request[identity] {
			return "", fmt.Errorf(
				"%s with %s %q now exists; regenerate the plan before creation",
				a.kind,
				identity,
				request[identity],
			)
		}
	}
	value, err := api.Create(ctx, a.kind, request)
	if err != nil {
		return "", err
	}
	return value.ID(), nil
}

func (a *BillingCatalogAdapter) Update(
	ctx context.Context,
	id string,
	request map[string]any,
	_ string,
	execCtx *ExecutionContext,
) (string, error) {
	// Recheck identity, ownership and lifecycle immediately before the write.
	if _, err := a.GetByID(ctx, id, execCtx); err != nil {
		return "", err
	}
	api, err := a.client.BillingCatalog()
	if err != nil {
		return "", err
	}
	value, err := api.Update(ctx, a.kind, id, request)
	if err != nil {
		return "", err
	}
	return value.ID(), nil
}

func (a *BillingCatalogAdapter) Delete(ctx context.Context, id string, execCtx *ExecutionContext) error {
	if a.kind == helpers.BillingCurrency {
		return fmt.Errorf("currencies cannot be deleted")
	}
	if _, err := a.GetByID(ctx, id, execCtx); err != nil {
		return err
	}
	if err := a.client.ValidateBillingDeletion(ctx, resources.BillingResourceType(a.kind), id); err != nil {
		return err
	}
	api, err := a.client.BillingCatalog()
	if err != nil {
		return err
	}
	return api.Delete(ctx, a.kind, id)
}

func (a *BillingCatalogAdapter) GetByName(context.Context, string) (ResourceInfo, error) {
	return nil, nil
}

func (a *BillingCatalogAdapter) GetForUpdate(ctx context.Context, change planner.PlannedChange) (ResourceInfo, error) {
	return a.GetByID(ctx, change.ResourceID, NewExecutionContext(&change))
}

func (a *BillingCatalogAdapter) GetByID(
	ctx context.Context,
	id string,
	execCtx *ExecutionContext,
) (ResourceInfo, error) {
	api, err := a.client.BillingCatalog()
	if err != nil {
		return nil, err
	}
	obj, err := api.Get(ctx, a.kind, id)
	if err != nil {
		return nil, err
	}
	if execCtx != nil {
		if a.kind == helpers.BillingCurrency {
			return nil, fmt.Errorf("currencies cannot be modified")
		}
		if !labels.IsManagedResource(obj.Labels()) || obj.Labels()[labels.NamespaceKey] != execCtx.Namespace {
			return nil, fmt.Errorf(
				"%s %s is no longer owned by namespace %q; regenerate the plan",
				a.kind,
				id,
				execCtx.Namespace,
			)
		}
		if a.kind == helpers.BillingPlan && obj.String(planner.FieldStatus) != "draft" {
			return nil, fmt.Errorf("plan %s is no longer a draft; regenerate the plan", id)
		}
		if a.kind == helpers.BillingProfile && obj[planner.FieldDefault] == true && execCtx.PlannedChange != nil &&
			execCtx.PlannedChange.Action == planner.ActionDelete {
			return nil, fmt.Errorf("default billing profile cannot be deleted")
		}
		if labels.IsProtectedResource(obj.Labels()) && !a.unprotectOnly(execCtx.PlannedChange, obj) {
			return nil, fmt.Errorf("%s %s is protected", a.kind, id)
		}
	}
	return billingResourceInfo{obj}, nil
}

// Unprotection is allowed only when a fresh observation proves that the full
// PUT preserves every public field. ChangedFields alone is not that proof.
func (a *BillingCatalogAdapter) unprotectOnly(change *planner.PlannedChange, current helpers.BillingObject) bool {
	if change == nil || change.Action != planner.ActionUpdate {
		return false
	}
	allowed := false
	switch value := change.Protection.(type) {
	case planner.ProtectionChange:
		allowed = value.Old && !value.New
	case map[string]any:
		allowed = value["old"] == true && value["new"] == false
	}
	if !allowed {
		return false
	}
	observed, err := helpers.BillingConfiguration(a.kind, current)
	if err != nil {
		return false
	}
	switch a.kind {
	case helpers.BillingTaxCode:
		delete(observed, planner.FieldKey)
	case helpers.BillingApp:
	case helpers.BillingProfile:
		delete(observed, planner.FieldApps)
	case helpers.BillingPlan:
		for _, field := range []string{planner.FieldKey, planner.FieldCurrency, planner.FieldBillingCadence} {
			delete(observed, field)
		}
	case helpers.BillingCurrency:
		return false
	}
	desired := maps.Clone(change.Fields)
	delete(desired, planner.FieldCurrentLabels)
	if labels.CompareUserLabels(current.Labels(), helpers.BillingObject(desired).Labels()) {
		return false
	}
	delete(observed, planner.FieldLabels)
	delete(desired, planner.FieldLabels)
	expected, err := helpers.NormalizeBillingFields(a.kind, observed, true)
	if err != nil {
		return false
	}
	actual, err := helpers.NormalizeBillingFields(a.kind, desired, true)
	if err != nil {
		return false
	}
	oldJSON, _ := json.Marshal(expected)
	newJSON, _ := json.Marshal(actual)
	return string(oldJSON) == string(newJSON)
}

type billingResourceInfo struct{ object helpers.BillingObject }

func (b billingResourceInfo) GetID() string                          { return b.object.ID() }
func (b billingResourceInfo) GetName() string                        { return b.object.String(planner.FieldName) }
func (b billingResourceInfo) GetLabels() map[string]string           { return b.object.Labels() }
func (b billingResourceInfo) GetNormalizedLabels() map[string]string { return b.object.Labels() }
func (e *Executor) registerBillingCatalogExecutors() {
	for _, kind := range []helpers.BillingKind{
		helpers.BillingTaxCode,
		helpers.BillingCurrency,
		helpers.BillingApp,
		helpers.BillingProfile,
		helpers.BillingPlan,
	} {
		adapter := &BillingCatalogAdapter{e.client, kind}
		base := NewManagedLabelBaseExecutor[map[string]any, map[string]any](adapter, e.client, e.dryRun)
		e.registerResourceExecutor(crudResourceExecutor(base))
	}
}
