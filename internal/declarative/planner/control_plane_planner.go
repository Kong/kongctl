package planner

import (
	"context"
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/util/normalizers"
)

// controlPlanePlannerImpl implements planning logic for control plane resources
type controlPlanePlannerImpl struct {
	*BasePlanner
}

// NewControlPlanePlanner creates a new control plane planner
func NewControlPlanePlanner(base *BasePlanner) ControlPlanePlanner {
	return &controlPlanePlannerImpl{BasePlanner: base}
}

func (p *controlPlanePlannerImpl) PlannerComponent() string {
	return string(resources.ResourceTypeControlPlane)
}

// PlanChanges generates changes for control plane resources
func (p *controlPlanePlannerImpl) PlanChanges(ctx context.Context, plannerCtx *Config, plan *Plan) error {
	namespace := plannerCtx.Namespace
	desired := p.GetDesiredControlPlanes(namespace)

	if len(desired) == 0 && plan.Metadata.Mode != PlanModeSync {
		return nil
	}

	currentControlPlanes, err := p.GetClient().ListManagedControlPlanes(ctx, []string{namespace})
	if err != nil {
		if state.IsAPIClientError(err) {
			return nil
		}
		return fmt.Errorf("failed to list current control planes in namespace %s: %w", namespace, err)
	}

	currentByName := make(map[string]state.ControlPlane)
	for i := range currentControlPlanes {
		cp := currentControlPlanes[i]
		if cp.Config.ClusterType == kkComps.ControlPlaneClusterTypeClusterTypeControlPlaneGroup {
			memberIDs, err := p.GetClient().ListControlPlaneGroupMemberships(ctx, cp.ID)
			if err != nil {
				return fmt.Errorf("failed to list control plane group memberships for %s: %w", cp.Name, err)
			}
			cp.GroupMembers = normalizers.NormalizeMemberIDs(memberIDs)
		}
		currentByName[cp.Name] = cp
	}

	protectionErrors := &ProtectionErrorCollector{}

	for _, desiredCP := range desired {
		if desiredCP.IsExternal() {
			p.planner.logger.Debug("Skipping external control plane", "ref", desiredCP.GetRef(), "name", desiredCP.Name)
			continue
		}
		current, exists := currentByName[desiredCP.Name]
		desiredProtected := isProtected(desiredCP)

		if !exists {
			p.planControlPlaneCreate(desiredCP, desiredProtected, plan)
			continue
		}

		currentProtected := labels.IsProtectedResource(current.NormalizedLabels)
		needsUpdate, updateFields := p.shouldUpdateControlPlane(current, desiredCP)

		if currentProtected != desiredProtected {
			protectionChange := &ProtectionChange{Old: currentProtected, New: desiredProtected}
			err := p.ValidateProtectionWithChange(
				"control_plane", desiredCP.Name, currentProtected, ActionUpdate, protectionChange, needsUpdate,
			)
			protectionErrors.Add(err)
			if err == nil {
				p.planControlPlaneProtectionChangeWithFields(current, desiredCP, protectionChange, updateFields, plan)
			}
			continue
		}

		if needsUpdate {
			err := p.ValidateProtection("control_plane", desiredCP.Name, currentProtected, ActionUpdate)
			protectionErrors.Add(err)
			if err == nil {
				p.planControlPlaneUpdate(current, desiredCP, updateFields, plan)
			}
		}
	}

	if plan.Metadata.Mode == PlanModeSync {
		desiredNames := make(map[string]struct{})
		for _, cp := range desired {
			if cp.IsExternal() {
				continue
			}
			desiredNames[cp.Name] = struct{}{}
		}

		for name, current := range currentByName {
			if _, ok := desiredNames[name]; ok {
				continue
			}

			isProtected := labels.IsProtectedResource(current.NormalizedLabels)
			err := p.ValidateProtection("control_plane", name, isProtected, ActionDelete)
			protectionErrors.Add(err)
			if err == nil {
				p.planControlPlaneDelete(current, plan)
			}
		}
	}

	if protectionErrors.HasErrors() {
		return protectionErrors.Error()
	}

	return nil
}

func (p *controlPlanePlannerImpl) planControlPlaneCreate(
	desired resources.ControlPlaneResource,
	protected bool,
	plan *Plan,
) {
	fields := extractControlPlaneFields(desired)
	memberIDs := normalizers.NormalizeMemberIDs(desired.MemberIDs())

	namespace := resources.GetNamespace(desired.Kongctl)
	config := CreateConfig{
		ResourceType:   "control_plane",
		ResourceName:   desired.Name,
		ResourceRef:    desired.GetRef(),
		RequiredFields: []string{"name"},
		FieldExtractor: func(_ any) map[string]any { return fields },
		Namespace:      namespace,
	}

	generic := p.GetGenericPlanner()
	if generic != nil {
		change, err := generic.PlanCreate(context.Background(), config)
		if err == nil {
			change.Protection = protected
			if desired.IsGroup() && len(memberIDs) > 0 {
				if change.References == nil {
					change.References = make(map[string]ReferenceInfo)
				}
				change.References["members"] = p.buildMemberReferenceInfo(memberIDs)
			}
			plan.AddChange(change)
			return
		}

		p.planner.logger.Error("Failed to plan control plane create", "error", err.Error())
	}

	changeID := p.NextChangeID(ActionCreate, "control_plane", desired.GetRef())
	change := PlannedChange{
		ID:           changeID,
		ResourceType: "control_plane",
		ResourceRef:  desired.GetRef(),
		Action:       ActionCreate,
		Fields:       fields,
		Namespace:    namespace,
		Protection:   protected,
	}

	if desired.IsGroup() && len(memberIDs) > 0 {
		change.References = map[string]ReferenceInfo{
			"members": p.buildMemberReferenceInfo(memberIDs),
		}
	}

	plan.AddChange(change)
}

func (p *controlPlanePlannerImpl) planControlPlaneUpdate(
	current state.ControlPlane,
	desired resources.ControlPlaneResource,
	updateFields map[string]any,
	plan *Plan,
) {
	var memberIDs []string
	if rawMembers, ok := updateFields["members"]; ok {
		if ids, ok := rawMembers.([]string); ok {
			memberIDs = ids
			updateFields["members"] = formatMemberField(ids)
		}
	}

	// Always include name for identification
	updateFields["name"] = current.Name

	if _, hasLabels := updateFields["labels"]; hasLabels {
		updateFields[FieldCurrentLabels] = current.NormalizedLabels
	}

	namespace := resources.GetNamespace(desired.Kongctl)
	config := UpdateConfig{
		ResourceType:   "control_plane",
		ResourceName:   desired.Name,
		ResourceRef:    desired.GetRef(),
		ResourceID:     current.ID,
		DesiredFields:  updateFields,
		RequiredFields: []string{"name"},
		Namespace:      namespace,
	}

	generic := p.GetGenericPlanner()
	if generic != nil {
		change, err := generic.PlanUpdate(context.Background(), config)
		if err == nil {
			if labels.IsProtectedResource(current.NormalizedLabels) {
				change.Protection = true
			}
			if len(memberIDs) > 0 {
				if change.References == nil {
					change.References = make(map[string]ReferenceInfo)
				}
				change.References["members"] = p.buildMemberReferenceInfo(memberIDs)
			}
			plan.AddChange(change)
			return
		}
		p.planner.logger.Error("Failed to plan control plane update", "error", err.Error())
	}

	fields := make(map[string]any)
	for key, value := range updateFields {
		fields[key] = value
	}

	changeID := p.NextChangeID(ActionUpdate, "control_plane", desired.GetRef())
	change := PlannedChange{
		ID:           changeID,
		ResourceType: "control_plane",
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		Action:       ActionUpdate,
		Fields:       fields,
		Namespace:    namespace,
	}

	if labels.IsProtectedResource(current.NormalizedLabels) {
		change.Protection = true
	}

	if len(memberIDs) > 0 {
		change.References = map[string]ReferenceInfo{
			"members": p.buildMemberReferenceInfo(memberIDs),
		}
	}

	plan.AddChange(change)
}

func (p *controlPlanePlannerImpl) planControlPlaneProtectionChangeWithFields(
	current state.ControlPlane,
	desired resources.ControlPlaneResource,
	protectionChange *ProtectionChange,
	updateFields map[string]any,
	plan *Plan,
) {
	var memberIDs []string
	if rawMembers, ok := updateFields["members"]; ok {
		if ids, ok := rawMembers.([]string); ok {
			memberIDs = ids
			updateFields["members"] = formatMemberField(ids)
		}
	}

	namespace := resources.GetNamespace(desired.Kongctl)
	config := ProtectionChangeConfig{
		ResourceType: "control_plane",
		ResourceName: desired.Name,
		ResourceRef:  desired.GetRef(),
		ResourceID:   current.ID,
		OldProtected: protectionChange.Old,
		NewProtected: protectionChange.New,
		Namespace:    namespace,
	}

	generic := p.GetGenericPlanner()
	change := PlannedChange{}
	if generic != nil {
		change = generic.PlanProtectionChange(context.Background(), config)
	} else {
		changeID := p.NextChangeID(ActionUpdate, "control_plane", desired.GetRef())
		change = PlannedChange{
			ID:           changeID,
			ResourceType: "control_plane",
			ResourceRef:  desired.GetRef(),
			ResourceID:   current.ID,
			Action:       ActionUpdate,
			Protection:   *protectionChange,
			Namespace:    namespace,
		}
	}

	fields := map[string]any{"name": current.Name}

	if protectionChange.Old && !protectionChange.New && len(updateFields) > 0 {
		for key, value := range updateFields {
			fields[key] = value
		}
		if _, hasLabels := updateFields["labels"]; hasLabels {
			fields[FieldCurrentLabels] = current.NormalizedLabels
		}
	}

	change.Fields = fields
	if len(memberIDs) > 0 {
		if change.References == nil {
			change.References = make(map[string]ReferenceInfo)
		}
		change.References["members"] = p.buildMemberReferenceInfo(memberIDs)
	}
	plan.AddChange(change)
}

func (p *controlPlanePlannerImpl) planControlPlaneDelete(current state.ControlPlane, plan *Plan) {
	namespace := DefaultNamespace
	if ns, ok := current.NormalizedLabels[labels.NamespaceKey]; ok {
		namespace = ns
	}

	generic := p.GetGenericPlanner()
	if generic != nil {
		config := DeleteConfig{
			ResourceType: "control_plane",
			ResourceName: current.Name,
			ResourceRef:  current.Name,
			ResourceID:   current.ID,
			Namespace:    namespace,
		}
		change := generic.PlanDelete(context.Background(), config)
		change.Fields = map[string]any{"name": current.Name}
		plan.AddChange(change)
		return
	}

	changeID := p.NextChangeID(ActionDelete, "control_plane", current.Name)
	plan.AddChange(PlannedChange{
		ID:           changeID,
		ResourceType: "control_plane",
		ResourceRef:  current.Name,
		ResourceID:   current.ID,
		Action:       ActionDelete,
		Fields:       map[string]any{"name": current.Name},
		Namespace:    namespace,
	})
}

func (p *controlPlanePlannerImpl) shouldUpdateControlPlane(
	current state.ControlPlane,
	desired resources.ControlPlaneResource,
) (bool, map[string]any) {
	updates := make(map[string]any)

	if desired.Description != nil {
		currentDesc := ""
		if current.Description != nil {
			currentDesc = *current.Description
		}
		if currentDesc != *desired.Description {
			updates["description"] = *desired.Description
		}
	}

	if desired.AuthType != nil {
		desiredAuth := string(*desired.AuthType)
		if desiredAuth != "" && desiredAuth != string(current.Config.AuthType) {
			updates["auth_type"] = desiredAuth
		}
	}

	if desired.ProxyUrls != nil {
		if !proxyURLsEqual(current.Config.ProxyUrls, desired.ProxyUrls) {
			updates["proxy_urls"] = desired.ProxyUrls
		}
	}

	if desired.Labels != nil {
		if labels.CompareUserLabels(current.NormalizedLabels, desired.Labels) {
			updates["labels"] = desired.Labels
		}
	}

	if desired.IsGroup() {
		desiredMembers := p.resolveDesiredGroupMemberIDs(desired)
		currentMembers := normalizers.NormalizeMemberIDs(current.GroupMembers)
		if !equalStringSlices(currentMembers, desiredMembers) {
			updates["members"] = desiredMembers
		}
	}

	return len(updates) > 0, updates
}

func extractControlPlaneFields(cp resources.ControlPlaneResource) map[string]any {
	fields := make(map[string]any)
	fields["name"] = cp.Name

	if cp.Description != nil {
		fields["description"] = *cp.Description
	}

	if cp.ClusterType != nil {
		fields["cluster_type"] = string(*cp.ClusterType)
	}

	if cp.AuthType != nil {
		fields["auth_type"] = string(*cp.AuthType)
	}

	if cp.CloudGateway != nil {
		fields["cloud_gateway"] = *cp.CloudGateway
	}

	if cp.ProxyUrls != nil {
		fields["proxy_urls"] = cp.ProxyUrls
	}

	if cp.Labels != nil {
		fields["labels"] = cp.Labels
	}

	if cp.IsGroup() {
		members := normalizers.NormalizeMemberIDs(cp.MemberIDs())
		fields["members"] = formatMemberField(members)
	}

	return fields
}

func proxyURLsEqual(current, desired []kkComps.ProxyURL) bool {
	if len(current) != len(desired) {
		return false
	}

	for i := range current {
		if current[i].Host != desired[i].Host ||
			current[i].Port != desired[i].Port ||
			current[i].Protocol != desired[i].Protocol {
			return false
		}
	}

	return true
}

func isProtected(cp resources.ControlPlaneResource) bool {
	if cp.Kongctl != nil && cp.Kongctl.Protected != nil {
		return *cp.Kongctl.Protected
	}
	return false
}

func formatMemberField(ids []string) []map[string]string {
	formatted := make([]map[string]string, 0, len(ids))
	for _, id := range ids {
		formatted = append(formatted, map[string]string{"id": id})
	}
	return formatted
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (p *controlPlanePlannerImpl) buildMemberReferenceInfo(ids []string) ReferenceInfo {
	info := ReferenceInfo{
		Refs:    make([]string, len(ids)),
		IsArray: true,
	}

	var names []string
	for i, id := range ids {
		info.Refs[i] = id

		if tags.IsRefPlaceholder(id) {
			ref, field, ok := tags.ParseRefPlaceholder(id)
			if ok && field == "id" && ref != "" && p.planner != nil && p.planner.resources != nil {
				if cp := p.planner.resources.GetControlPlaneByRef(ref); cp != nil {
					if names == nil {
						names = make([]string, len(ids))
					}
					names[i] = cp.Name
				}
			}
		}
	}

	if names != nil {
		info.LookupArrays = map[string][]string{
			"names": names,
		}
	}

	return info
}

func (p *controlPlanePlannerImpl) resolveDesiredGroupMemberIDs(desired resources.ControlPlaneResource) []string {
	raw := desired.MemberIDs()
	if len(raw) == 0 {
		return []string{}
	}

	resolved := make([]string, 0, len(raw))
	for _, memberID := range raw {
		if !tags.IsRefPlaceholder(memberID) {
			resolved = append(resolved, memberID)
			continue
		}

		ref, field, ok := tags.ParseRefPlaceholder(memberID)
		if !ok || field != "id" || ref == "" || p.planner == nil || p.planner.resources == nil {
			resolved = append(resolved, memberID)
			continue
		}

		if cp := p.planner.resources.GetControlPlaneByRef(ref); cp != nil {
			if konnectID := cp.GetKonnectID(); konnectID != "" {
				resolved = append(resolved, konnectID)
				continue
			}
		}

		resolved = append(resolved, memberID)
	}

	return normalizers.NormalizeMemberIDs(resolved)
}
