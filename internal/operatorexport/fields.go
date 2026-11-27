package operatorexport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// toFieldMap encodes an SDK entity as JSON and decodes it into a generic map,
// keeping numbers exact so that values round-trip unchanged.
func toFieldMap(entity any) (map[string]any, error) {
	raw, err := json.Marshal(entity)
	if err != nil {
		return nil, fmt.Errorf("encode entity: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	fields := map[string]any{}
	if err := decoder.Decode(&fields); err != nil {
		return nil, fmt.Errorf("decode entity: %w", err)
	}
	return fields, nil
}

// serverManagedFields are Konnect fields that never belong in a spec.
var serverManagedFields = []string{"id", "created_at", "updated_at"}

// pickFields copies the allowed keys from fields into a new spec map. It
// returns the sorted names of keys that carry a value but are neither allowed
// nor handled by the caller, so the caller can warn that they cannot be
// represented. Tags are always handled separately.
func pickFields(fields map[string]any, allowed, handled []string) (map[string]any, []string) {
	spec := map[string]any{}
	var unrepresented []string
	for key, value := range fields {
		switch {
		case key == "tags":
			continue
		case slices.Contains(allowed, key):
			if value != nil {
				spec[key] = value
			}
		case slices.Contains(handled, key), slices.Contains(serverManagedFields, key):
			continue
		case !isEmptyValue(value):
			unrepresented = append(unrepresented, key)
		}
	}
	slices.Sort(unrepresented)
	return spec, unrepresented
}

func isEmptyValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case bool:
		return !v
	case []any:
		return len(v) == 0
	case map[string]any:
		return len(v) == 0
	default:
		return false
	}
}

// cleanTags removes tags the CRDs reject. It reports the removed tags and
// whether a k8s-uid tag shows the entity is already owned by another resource.
func cleanTags(tags []string) (kept, removed []string, ownedByK8s bool) {
	for _, tag := range tags {
		reserved := false
		for _, prefix := range reservedTagPrefixes {
			if strings.HasPrefix(tag, prefix) {
				reserved = true
				break
			}
		}
		if !reserved {
			kept = append(kept, tag)
			continue
		}
		removed = append(removed, tag)
		if strings.HasPrefix(tag, k8sUIDTagPrefix) {
			ownedByK8s = true
		}
	}
	return kept, removed, ownedByK8s
}

func strValue(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
