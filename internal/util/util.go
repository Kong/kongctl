package util

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
)

// InitDir initializes a directory with the given mode
func InitDir(path string, mode fs.FileMode) error {
	expandedDir := os.ExpandEnv(path)
	fullPath := filepath.Dir(expandedDir)
	err := os.MkdirAll(fullPath, mode)
	return err
}

// GetString returns the string value pointed to by value, or an empty string if value is nil.
func GetString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// GetStringFromReflectValue safely extracts a string value from a reflect.Value that may be
// a string, *string, or nil pointer. Returns empty string if nil or invalid.
func GetStringFromReflectValue(v reflect.Value) (string, error) {
	if !v.IsValid() {
		return "", fmt.Errorf("invalid reflect value")
	}

	//nolint: exhaustive
	switch v.Kind() {
	case reflect.String:
		return v.String(), nil
	case reflect.Ptr:
		if v.IsNil() {
			return "", nil
		}
		elem := v.Elem()
		if elem.Kind() != reflect.String {
			return "", fmt.Errorf("reflect value is not a string or *string")
		}
		return elem.String(), nil
	default:
		return "", fmt.Errorf("reflect value is not a string or *string")
	}
}

// IsPreviewEnabled returns true if preview features (like Event Gateway) are enabled
// via the KONGCTL_ENABLE_EVENT_GATEWAY environment variable.
func IsPreviewEnabled() bool {
	preview := os.Getenv("KONGCTL_ENABLE_EVENT_GATEWAY")
	return preview != ""
}
