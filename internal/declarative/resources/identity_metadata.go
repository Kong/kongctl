package resources

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
)

// ValidateIdentityMetadata checks the shared public value shapes and the
// decimal/binary64 model of the observed API round trip (Q31).
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
		if !identityMetadataIntegerStable(value) {
			return fmt.Errorf(
				"metadata %q contains an integer that the API would round (Q31); use a string for exact identifiers",
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

func identityMetadataIntegerStable(value any) bool {
	stable := func(number int64) bool {
		return strconv.FormatFloat(float64(number), 'f', -1, 64) == strconv.FormatInt(number, 10)
	}
	switch number := value.(type) {
	case int:
		return stable(int64(number))
	case int32:
		return stable(int64(number))
	case int64:
		return stable(number)
	case json.Number:
		integer, err := number.Int64()
		return err == nil && stable(integer)
	case float64:
		return stable(int64(number))
	default:
		if items := reflect.ValueOf(value); items.Kind() == reflect.Slice {
			for i := range items.Len() {
				if !identityMetadataIntegerStable(items.Index(i).Interface()) {
					return false
				}
			}
		}
		return true
	}
}
