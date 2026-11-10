package resources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
)

const (
	IdentityMetadataModeMerge = "merge"
	IdentityMetadataModeSync  = "sync"
)

// MergeIdentityMetadata retains unmentioned public keys, preserving integer
// precision when converting an SDK observation to declarative values.
func MergeIdentityMetadata(current any, updates map[string]any) (map[string]any, error) {
	data, err := json.Marshal(current)
	if err != nil {
		return nil, fmt.Errorf("invalid current principal metadata")
	}
	var merged map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&merged); err != nil {
		return nil, fmt.Errorf("invalid current principal metadata")
	}
	if merged == nil {
		merged = make(map[string]any)
	}
	maps.Copy(merged, updates)
	if err := ValidateIdentityMetadata(merged); err != nil {
		return nil, err
	}
	return merged, nil
}

// ValidateIdentityMetadata checks the shared public value shapes and the
// signed integer range supported by the API.
func ValidateIdentityMetadata(metadata map[string]any) error {
	if len(metadata) > 50 {
		return fmt.Errorf("metadata supports at most 50 entries")
	}
	for key, value := range metadata {
		if !validPrincipalMetadata(value) {
			return fmt.Errorf(
				"metadata %q must contain strings, booleans, integers, string arrays, or integer arrays",
				key,
			)
		}
		if items := reflect.ValueOf(value); items.Kind() == reflect.Slice {
			if items.Len() > 255 {
				return fmt.Errorf("metadata %q supports at most 255 array entries", key)
			}
			seen := make(map[string]bool)
			for i := range items.Len() {
				encoded, err := json.Marshal(items.Index(i).Interface())
				if err != nil {
					return fmt.Errorf("invalid metadata array %q", key)
				}
				if seen[string(encoded)] {
					return fmt.Errorf("metadata %q array entries must be unique", key)
				}
				seen[string(encoded)] = true
			}
		}
	}
	return nil
}
