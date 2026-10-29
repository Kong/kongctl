package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"

	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/konnect/helpers"
)

type (
	billingCatalogPlanner struct {
		*BasePlanner
		kind resources.ResourceType
	}
	desiredBilling struct {
		resource resources.BillingResource
		fields   map[string]any
	}
)

func (p *billingCatalogPlanner) PlannerComponent() string { return string(p.kind) }

func (p *Planner) listBilling(ctx context.Context, kind resources.ResourceType) ([]helpers.BillingObject, error) {
	if cached, ok := p.resourceCache.billingCatalog[kind]; ok {
		return cached, nil
	}
	values, err := p.client.ListBilling(ctx, resources.BillingKindForResourceType(kind))
	if err != nil {
		return nil, err
	}
	p.resourceCache.billingCatalog[kind] = values
	return values, nil
}

func (p *billingCatalogPlanner) PlanChanges(ctx context.Context, cfg *Config, plan *Plan) error {
	desired := p.planner.resources.BillingResources(p.kind, cfg.Namespace)
	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}
	current, err := p.planner.listBilling(ctx, p.kind)
	if err != nil {
		return err
	}
	kind := resources.BillingKindForResourceType(p.kind)
	root := func(c helpers.BillingObject) managedRoot[helpers.BillingObject] {
		return managedRoot[helpers.BillingObject]{
			resource:  c,
			name:      billingIdentity(kind, c),
			protected: labels.IsProtectedResource(c.Labels()),
		}
	}
	write := func(
		action ActionType,
		c helpers.BillingObject,
		d desiredBilling,
		fields map[string]any,
		changed map[string]FieldChange,
		plan *Plan,
	) string {
		change := PlannedChange{
			ID:            p.planner.nextChangeID(action, string(p.kind), d.resource.GetRef()),
			Action:        action,
			ResourceType:  string(p.kind),
			ResourceRef:   d.resource.GetRef(),
			Fields:        fields,
			ChangedFields: changed,
			Namespace:     cfg.Namespace,
			Protection:    billingProtected(d.resource),
		}
		if c != nil {
			change.ResourceID = c.ID()
			change.Protection = ProtectionChange{
				Old: labels.IsProtectedResource(c.Labels()),
				New: billingProtected(d.resource),
			}
		}
		plan.AddChange(change)
		return change.ID
	}
	reconciler := newManagedRootReconciler(
		p.BasePlanner,
		string(p.kind),
		managedRootOperations[desiredBilling, helpers.BillingObject]{
			diff: func(c helpers.BillingObject, d desiredBilling) (bool, map[string]any, map[string]FieldChange) {
				return billingDiff(kind, c, d.fields)
			},
			create: func(d desiredBilling, plan *Plan) string { return write(ActionCreate, nil, d, d.fields, nil, plan) },
			update: func(
				c helpers.BillingObject,
				d desiredBilling,
				f map[string]any,
				changed map[string]FieldChange,
				plan *Plan,
			) {
				write(ActionUpdate, c, d, f, changed, plan)
			},
			changeProtection: func(
				c helpers.BillingObject,
				d desiredBilling,
				_, _ bool,
				f map[string]any,
				changed map[string]FieldChange,
				plan *Plan,
			) {
				write(ActionUpdate, c, d, f, changed, plan)
			},
			remove: func(c helpers.BillingObject, plan *Plan) {
				plan.AddChange(
					PlannedChange{
						ID:           p.planner.nextChangeID(ActionDelete, string(p.kind), c.ID()),
						Action:       ActionDelete,
						ResourceType: string(p.kind),
						ResourceRef:  billingIdentity(kind, c),
						ResourceID:   c.ID(),
						Namespace:    cfg.Namespace,
						Fields:       map[string]any{FieldName: c.String(FieldName)},
					},
				)
			},
		},
		plan,
	)
	retained := map[string]bool{}
	for _, r := range desired {
		fields, err := r.BillingFields()
		if err != nil {
			return err
		}
		c, err := matchBilling(kind, r, fields, current)
		if err != nil {
			return err
		}
		if c != nil {
			if c[FieldDeletedAt] != nil {
				return fmt.Errorf(
					"%s %q is soft-deleted; use a new key rather than implicitly recreating it",
					kind,
					r.GetRef(),
				)
			}
			if kind == helpers.BillingApp && fields[FieldAPIKeyEnv] != nil {
				plan.AddWarning(
					"",
					fmt.Sprintf(
						"billing app %q: api_key_env is install-only; rotate credentials explicitly with update mb apps",
						r.GetRef(),
					),
				)
			}
			if kind != helpers.BillingCurrency &&
				(!labels.IsManagedResource(c.Labels()) || c.Labels()[labels.NamespaceKey] != cfg.Namespace) {
				return fmt.Errorf(
					"%s %q is not owned by namespace %q; explicitly adopt its ULID first",
					kind,
					r.GetRef(),
					cfg.Namespace,
				)
			}
			retained[c.ID()] = true
			if target, ok := r.(interface{ SetKonnectID(string) }); ok {
				target.SetKonnectID(c.ID())
			}
		}
		if plan.Metadata.Mode == PlanModeDelete {
			if kind == helpers.BillingCurrency {
				return fmt.Errorf("currencies cannot be deleted; remove the declaration to stop observing")
			}
			if err := validateBillingRemoval(kind, c); err != nil {
				return err
			}
			var observed *managedRoot[helpers.BillingObject]
			if c != nil {
				value := root(c)
				observed = &value
			}
			reconciler.deleteDesired(r.GetMoniker(), observed)
			continue
		}
		if err := p.resolveReferences(ctx, fields); err != nil {
			return fmt.Errorf("%s %q: %w", kind, r.GetRef(), err)
		}
		// Normalize against the native install contract without retaining its shortcut in desired state.
		if kind == helpers.BillingApp {
			fields[FieldCreateBillingProfile] = false
		}
		fields, err = helpers.NormalizeBillingFields(kind, fields, false)
		if err != nil {
			return err
		}
		delete(fields, FieldCreateBillingProfile)
		if c != nil {
			if kind == helpers.BillingPlan && c.String(FieldCurrency) != fields[FieldCurrency] {
				return fmt.Errorf("plan currency is immutable")
			}
			changed, _, _ := billingDiff(kind, c, fields)
			if kind == helpers.BillingCurrency {
				if changed {
					return fmt.Errorf("currency %q differs: custom currencies cannot be updated", r.GetMoniker())
				}
				continue
			}
			if kind == helpers.BillingProfile &&
				!reflect.DeepEqual(canonicalBillingApps(c[FieldApps]), canonicalBillingApps(fields[FieldApps])) {
				return fmt.Errorf("billing profile %q app associations are immutable; create a new profile", r.GetRef())
			}
			if kind == helpers.BillingPlan && c.String(FieldStatus) != "draft" {
				if changed || labels.IsProtectedResource(c.Labels()) != billingProtected(r) {
					return fmt.Errorf(
						"plan %q version %v is %s; update an explicit draft, never a published version",
						r.GetRef(),
						c[FieldVersion],
						c.String(FieldStatus),
					)
				}
				continue
			}
		}
		var observed *managedRoot[helpers.BillingObject]
		if c != nil {
			value := root(c)
			observed = &value
		}
		reconciler.reconcileLifecycle(
			managedRoot[desiredBilling]{
				resource:  desiredBilling{r, fields},
				name:      r.GetMoniker(),
				protected: billingProtected(r),
			},
			observed,
		)
	}
	if plan.Metadata.Mode == PlanModeSync && kind != helpers.BillingCurrency {
		for _, c := range current {
			if c[FieldDeletedAt] != nil {
				continue
			}
			if labels.IsManagedResource(c.Labels()) && c.Labels()[labels.NamespaceKey] == cfg.Namespace &&
				!retained[c.ID()] {
				if kind == helpers.BillingPlan && c.String(FieldStatus) != "draft" {
					plan.AddWarning(
						"",
						fmt.Sprintf(
							"retaining %s plan %s; archive plan versions explicitly",
							c.String(FieldStatus),
							c.ID(),
						),
					)
					continue
				}
				if err := validateBillingRemoval(kind, c); err != nil {
					return err
				}
				reconciler.remove(root(c))
			}
		}
	}
	return reconciler.errors.Error()
}

func billingProtected(r resources.BillingResource) bool {
	return r.BillingMeta() != nil && r.BillingMeta().Protected != nil && *r.BillingMeta().Protected
}

func validateBillingRemoval(kind helpers.BillingKind, c helpers.BillingObject) error {
	if c == nil {
		return nil
	}
	if kind == helpers.BillingPlan && c.String(FieldStatus) != "draft" {
		return fmt.Errorf(
			"plan %s is %s; archive it explicitly instead of declarative deletion",
			c.ID(),
			c.String(FieldStatus),
		)
	}
	if kind == helpers.BillingProfile && c[FieldDefault] == true {
		return fmt.Errorf("default billing profile %s cannot be deleted; designate another default first", c.ID())
	}
	return nil
}

func billingIdentity(kind helpers.BillingKind, c helpers.BillingObject) string {
	// Scoped billing lifecycle classification; other kinds follow the default path.
	//exhaustive:ignore
	switch kind {
	case helpers.BillingTaxCode:
		return c.String(FieldKey)
	case helpers.BillingCurrency:
		return c.String(FieldCode)
	case helpers.BillingPlan:
		return fmt.Sprintf("%s@%v", c.String(FieldKey), c[FieldVersion])
	default:
		return c.String(FieldName)
	}
}

func matchBilling(
	kind helpers.BillingKind,
	r resources.BillingResource,
	fields map[string]any,
	current []helpers.BillingObject,
) (helpers.BillingObject, error) {
	matches := []helpers.BillingObject{}
	for _, c := range current {
		if r.BillingID() != "" {
			if c.ID() == r.BillingID() {
				matches = append(matches, c)
			}
			continue
		}
		// Scoped billing lifecycle classification; other kinds follow the default path.
		//exhaustive:ignore
		switch kind {
		case helpers.BillingTaxCode:
			if c.String(FieldKey) == fields[FieldKey] {
				matches = append(matches, c)
			}
		case helpers.BillingCurrency:
			if c.String(FieldCode) == fields[FieldCode] {
				matches = append(matches, c)
			}
		case helpers.BillingPlan:
			d := r.(*resources.BillingPlanResource)
			if c.String(FieldKey) != d.Key {
				continue
			}
			if d.Version != nil && c[FieldVersion] != float64(*d.Version) {
				continue
			}
			matches = append(matches, c)
		default:
			if c.String(FieldName) == fields[FieldName] {
				matches = append(matches, c)
			}
		}
	}
	if r.BillingID() != "" && len(matches) == 0 {
		return nil, fmt.Errorf("%s %q id %q not found; remove id to create", kind, r.GetRef(), r.BillingID())
	}
	if kind == helpers.BillingPlan && len(matches) > 1 {
		drafts := []helpers.BillingObject{}
		for _, c := range matches {
			if c.String(FieldStatus) == "draft" {
				drafts = append(drafts, c)
			}
		}
		if len(drafts) == 1 {
			matches = drafts
		}
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("%s %q has ambiguous remote identity; specify id (and plan version)", kind, r.GetRef())
	}
	if len(matches) == 0 {
		return nil, nil
	}
	c := matches[0]
	if kind == helpers.BillingTaxCode && c.String(FieldKey) != fields[FieldKey] {
		return nil, fmt.Errorf("tax code key is immutable")
	}
	if kind == helpers.BillingCurrency &&
		(c.String(FieldCode) != fields[FieldCode] || c.String(FieldAppType) != "custom") {
		return nil, fmt.Errorf("currency code must select a custom currency; fiat currencies are read-only")
	}
	if kind == helpers.BillingPlan {
		d := r.(*resources.BillingPlanResource)
		if c.String(FieldKey) != d.Key || c.String(FieldBillingCadence) != d.BillingCadence {
			return nil, fmt.Errorf("plan key, currency and billing_cadence are immutable")
		}
		if d.Version != nil && c[FieldVersion] != float64(*d.Version) {
			return nil, fmt.Errorf("plan id and version select different identities")
		}
	}
	if kind == helpers.BillingApp && c.String(FieldAppType) != fields[FieldAppType] {
		return nil, fmt.Errorf("app type is immutable; install a new app")
	}
	return c, nil
}

func canonicalBillingApps(value any) map[string]any {
	result := map[string]any{}
	if apps, ok := value.(map[string]any); ok {
		for role, v := range apps {
			if app, ok := v.(map[string]any); ok {
				result[role] = app[FieldID]
			}
		}
	}
	return result
}

func billingDiff(
	kind helpers.BillingKind,
	c helpers.BillingObject,
	desired map[string]any,
) (bool, map[string]any, map[string]FieldChange) {
	current, err := helpers.BillingConfiguration(kind, c)
	if err != nil {
		return true, map[string]any{FieldError: err.Error()}, nil
	}
	oldLabels := labels.GetUserLabels(c.Labels())
	if len(oldLabels) > 0 {
		current[FieldLabels] = oldLabels
	} else {
		delete(current, FieldLabels)
	}
	compare := maps.Clone(desired)
	delete(compare, FieldAPIKeyEnv)
	if kind == helpers.BillingApp {
		if current[FieldDescription] == nil {
			delete(current, FieldDescription)
		}
	}
	oldJSON, _ := json.Marshal(current)
	newJSON, _ := json.Marshal(compare)
	changed := map[string]FieldChange{}
	if string(oldJSON) != string(newJSON) {
		for key, value := range compare {
			if !billingJSONEqual(current[key], value) {
				changed[key] = FieldChange{Old: current[key], New: value}
			}
		}
		for key, value := range current {
			if _, ok := compare[key]; !ok {
				changed[key] = FieldChange{Old: value, New: nil}
			}
		}
	}
	update := maps.Clone(compare)
	// Scoped billing lifecycle classification; other kinds follow the default path.
	//exhaustive:ignore
	switch kind {
	case helpers.BillingTaxCode:
		delete(update, FieldKey)
	case helpers.BillingPlan:
		for _, key := range []string{FieldKey, FieldCurrency, FieldBillingCadence} {
			delete(update, key)
		}
	case helpers.BillingProfile:
		delete(update, FieldApps)
	}
	return string(oldJSON) != string(newJSON), update, changed
}

func billingJSONEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func (p *billingCatalogPlanner) resolveReferences(ctx context.Context, fields map[string]any) error {
	for path, kind := range resources.BillingReferenceTargets(p.kind, fields) {
		value, set, err := resources.BillingPointer(fields, path)
		if err != nil {
			return err
		}
		text, _ := value.(string)
		if text == "" {
			return fmt.Errorf("%s must specify a ref or remote identifier", path)
		}
		if kind == resources.ResourceTypeCurrency {
			if helpers.IsMeterULID(text) {
				return fmt.Errorf("%s requires a currency code or declared currency ref, not a ULID", path)
			}
			if tags.IsRefPlaceholder(text) {
				ref, selector, ok := tags.ParseRefPlaceholder(text)
				if !ok || selector != FieldCode {
					return fmt.Errorf("currency references must select #code")
				}
				text = ref
			}
		} else if helpers.IsMeterULID(text) {
			continue
		} else if tags.IsRefPlaceholder(text) {
			ref, selector, ok := tags.ParseRefPlaceholder(text)
			if !ok || selector != FieldID {
				return fmt.Errorf("%s must select a %s #id", path, kind)
			}
			text = ref
		}
		target, ok := p.planner.resources.GetResourceByRef(text)
		if !ok {
			if kind == resources.ResourceTypeCurrency {
				values, err := p.planner.listBilling(ctx, kind)
				if err != nil {
					return err
				}
				found := false
				for _, v := range values {
					if v.String(FieldCode) == text {
						found = true
						break
					}
				}
				if found {
					continue
				}
				return fmt.Errorf("currency %q not found; declare a custom currency or use an existing code", text)
			}
			return fmt.Errorf("%s must reference a declared %s or remote ULID", path, kind)
		}
		if target.GetType() != kind {
			return fmt.Errorf("%s ref %q must target %s", path, text, kind)
		}
		if kind == resources.ResourceTypeCurrency {
			set(target.(*resources.CurrencyResource).Code)
			continue
		}
		id := ""
		if feature, ok := target.(*resources.FeatureResource); ok {
			values, err := p.planner.listFeatures(ctx)
			if err != nil {
				return err
			}
			for _, v := range values {
				if (feature.ID != "" && feature.ID == v.ID) || (feature.ID == "" && feature.Key == v.Key) {
					id = v.ID
					break
				}
			}
		} else if b, ok := target.(resources.BillingResource); ok {
			values, err := p.planner.listBilling(ctx, kind)
			if err != nil {
				return err
			}
			f, err := b.BillingFields()
			if err != nil {
				return err
			}
			c, err := matchBilling(resources.BillingKindForResourceType(kind), b, f, values)
			if err != nil {
				return err
			}
			if c != nil {
				id = c.ID()
			}
		}
		if id != "" {
			set(id)
		} else {
			set(tags.RefPlaceholderPrefix + text + "#" + FieldID)
		}
	}
	return nil
}

func bindBillingReferences(plan *Plan, rs *resources.ResourceSet) error {
	creates := map[string]string{}
	for _, c := range plan.Changes {
		if c.Action == ActionCreate {
			creates[c.ResourceRef] = c.ID
		}
	}
	for i := range plan.Changes {
		change := &plan.Changes[i]
		if change.Action == ActionDelete {
			continue
		}
		// Scoped billing lifecycle classification; other kinds follow the default path.
		//exhaustive:ignore
		switch resources.ResourceType(change.ResourceType) {
		case resources.ResourceTypeBillingPlan, resources.ResourceTypeBillingProfile:
		default:
			continue
		}
		targets := resources.BillingReferenceTargets(resources.ResourceType(change.ResourceType), change.Fields)
		for path, kind := range targets {
			value, _, err := resources.BillingPointer(change.Fields, path)
			if err != nil {
				return err
			}
			text, _ := value.(string)
			if tags.IsRefPlaceholder(text) {
				ref, _, ok := tags.ParseRefPlaceholder(text)
				if !ok || creates[ref] == "" {
					return fmt.Errorf("%s has no creation for reference %q", change.ResourceRef, ref)
				}
				change.PayloadReferences = append(
					change.PayloadReferences,
					PayloadReference{Path: path, ResourceType: string(kind), Ref: ref, Selector: FieldID},
				)
				change.DependsOn = appendDependsOn(change.DependsOn, creates[ref])
			}
		}
		if change.ResourceType == ResourceTypeBillingPlan {
			if r, ok := rs.GetResourceByRef(change.ResourceRef); ok {
				if desired, ok := r.(resources.BillingResource); ok {
					original, err := desired.BillingFields()
					if err != nil {
						return err
					}
					for path, kind := range resources.BillingReferenceTargets(resources.ResourceTypeBillingPlan, original) {
						if kind != resources.ResourceTypeCurrency {
							continue
						}
						value, _, err := resources.BillingPointer(original, path)
						if err != nil {
							return err
						}
						ref, _ := value.(string)
						if tags.IsRefPlaceholder(ref) {
							parsed, _, ok := tags.ParseRefPlaceholder(ref)
							if ok {
								ref = parsed
							}
						}
						if target, ok := rs.GetResourceByRef(ref); ok &&
							target.GetType() == resources.ResourceTypeCurrency {
							if dependency := creates[ref]; dependency != "" {
								change.DependsOn = appendDependsOn(change.DependsOn, dependency)
							}
						}
					}
				}
			}
		}
	}
	return nil
}
