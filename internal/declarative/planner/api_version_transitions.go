package planner

import (
	"slices"

	"github.com/kong/kongctl/internal/declarative/resources"
)

// Authorization belongs to the desired parent API and is persisted separately
// from payload fields. Child versions inherit it, including extracted versions.
func applyAPIOperationRemovalAuthorization(changes []PlannedChange, rs *resources.ResourceSet) {
	for i := range changes {
		change := &changes[i]
		if change.Action != ActionUpdate {
			continue
		}
		var apiRef string
		switch change.ResourceType {
		case ResourceTypeAPI:
			apiRef = change.ResourceRef
		case ResourceTypeAPIVersion:
			if change.Parent != nil {
				apiRef = change.Parent.Ref
			}
		default:
			continue
		}
		if api := rs.GetAPIByRef(apiRef); api != nil {
			change.ForceDelete = api.ForceDelete != nil && *api.ForceDelete
		}
	}
}

// A current-version switch must wait for its target spec, and releasing the
// previous current version must wait for the switch. Other APIs stay concurrent.
func adjustAPIVersionTransitionDependencies(changes []PlannedChange) {
	for i := range changes {
		api := &changes[i]
		if api.ResourceType != ResourceTypeAPI || api.Action != ActionUpdate {
			continue
		}
		versionChange, switching := api.ChangedFields[FieldVersion]
		if !switching {
			continue
		}
		target, ok := api.Fields[FieldVersion].(string)
		if !ok {
			continue
		}
		for j := range changes {
			child := &changes[j]
			if child.ResourceType != ResourceTypeAPIVersion || child.Parent == nil {
				continue
			}
			if !unresolvedReferenceID(api.ResourceID) && !unresolvedReferenceID(child.Parent.ID) {
				if api.ResourceID != child.Parent.ID {
					continue
				}
			} else if child.Parent.Ref != api.ResourceRef || child.Namespace != api.Namespace {
				continue
			}
			version, ok := child.Fields[FieldVersion].(string)
			if !ok {
				continue
			}
			if (child.Action == ActionCreate || child.Action == ActionUpdate) && version == target {
				if !slices.Contains(api.DependsOn, child.ID) {
					api.DependsOn = append(api.DependsOn, child.ID)
				}
			}
			if child.Action == ActionDelete && version == versionChange.Old {
				if !slices.Contains(child.DependsOn, api.ID) {
					child.DependsOn = append(child.DependsOn, api.ID)
				}
			}
		}
	}
}
