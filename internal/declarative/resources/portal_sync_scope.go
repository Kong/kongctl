package resources

import (
	"fmt"
	"maps"
	"slices"
	"sync"
)

var cachedPortalSingletonKeys = sync.OnceValue(func() map[string]struct{} {
	keys := make(map[string]struct{})
	for _, child := range cachedSyncCollections() {
		if child.ParentType == ResourceTypePortal {
			for _, key := range child.nestedNullKeys {
				keys[key] = struct{}{}
			}
		}
	}
	return keys
})

// PortalSingletonScopeKeys shares the declaration-derived singleton/map keys
// with load-schema diagnostics without exposing mutable cached metadata.
func PortalSingletonScopeKeys() []string {
	return slices.Collect(maps.Keys(cachedPortalSingletonKeys()))
}

// Portal presence has coupled collections and singleton diagnostics that must
// be evaluated per portal, before moving on to the next declaration.
func capturePortalNestedSyncScope(
	scope *SyncScope,
	portalRef string,
	portal map[string]any,
	collections []SyncCollection,
) error {
	for key := range cachedPortalSingletonKeys() {
		if value, ok := portal[key]; ok && value == nil {
			return fmt.Errorf(
				"portal %q child singleton %q cannot be null; omit the key to ignore it or provide an object to manage it",
				portalRef,
				key,
			)
		}
	}
	for _, child := range collections {
		if child.ParentType != ResourceTypePortal {
			continue
		}
		for _, path := range child.NestedPaths {
			if len(path) != 2 {
				panic("portal child scope requires a direct declaration field")
			}
			if value, present := portal[path[1]]; present {
				scope.addNestedChild(ResourceTypePortal, portalRef, child.ResourceType)
				if child.ResourceType == ResourceTypePortalTeam && portalTeamsIncludeGroupMappings(value) {
					scope.AddChild(ResourceTypePortal, portalRef, ResourceTypePortalTeamGroupMapping)
				}
			}
		}
	}
	assetsValue, assetsPresent := portal["assets"]
	if assetsPresent && assetsValue == nil {
		return fmt.Errorf(
			"portal %q child singleton %q cannot be null; omit the key to ignore assets or provide an object to manage them",
			portalRef,
			"assets",
		)
	}
	if assets, ok := scopeMap(assetsValue); ok {
		if err := capturePortalAssetScope(
			scope, assets, "logo", "assets.logo", portalRef, ResourceTypePortalAssetLogo,
		); err != nil {
			return err
		}
		if err := capturePortalAssetScope(
			scope, assets, "favicon", "assets.favicon", portalRef, ResourceTypePortalAssetFavicon,
		); err != nil {
			return err
		}
	}
	return nil
}

func capturePortalAssetScope(
	scope *SyncScope,
	assets map[string]any,
	assetKey, qualifiedKey string,
	portalRef string,
	resourceType ResourceType,
) error {
	value, ok := assets[assetKey]
	if !ok {
		return nil
	}
	if value == nil {
		return fmt.Errorf(
			"portal %q child singleton %q cannot be null; omit the key to ignore it or provide a value",
			portalRef,
			qualifiedKey,
		)
	}
	scope.AddChild(ResourceTypePortal, portalRef, resourceType)
	return nil
}

func portalTeamsIncludeGroupMappings(value any) bool {
	teams, ok := scopeSlice(value)
	if !ok {
		return false
	}
	for _, item := range teams {
		team, ok := scopeMap(item)
		if !ok {
			continue
		}
		if _, ok := team["group_mappings"]; ok {
			return true
		}
	}
	return false
}

func scopeMap(value any) (map[string]any, bool) {
	m, ok := value.(map[string]any)
	return m, ok
}

func scopeSlice(value any) ([]any, bool) {
	items, ok := value.([]any)
	return items, ok
}
