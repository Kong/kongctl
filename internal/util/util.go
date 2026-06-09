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

	switch v.Kind() {
	case reflect.String:
		return v.String(), nil
	case reflect.Pointer:
		if v.IsNil() {
			return "", nil
		}
		elem := v.Elem()
		if elem.Kind() != reflect.String {
			return "", fmt.Errorf("reflect value is not a string or *string")
		}
		return elem.String(), nil
	case reflect.Invalid,
		reflect.Bool,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128,
		reflect.Array,
		reflect.Chan,
		reflect.Func,
		reflect.Interface,
		reflect.Map,
		reflect.Slice,
		reflect.Struct,
		reflect.UnsafePointer:
		return "", fmt.Errorf("reflect value is not a string or *string")
	default:
		return "", fmt.Errorf("reflect value is not a string or *string")
	}
}
