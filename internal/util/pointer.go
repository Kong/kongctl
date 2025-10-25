package util

import "time"

// StringValue returns the dereferenced string or empty string when the pointer is nil.
func StringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// StringValueOr returns the dereferenced string or the provided fallback when the pointer is nil or empty.
func StringValueOr(value *string, fallback string) string {
	if s := StringValue(value); s != "" {
		return s
	}
	return fallback
}

// BoolValue returns the dereferenced bool or false when the pointer is nil.
func BoolValue(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
}

// BoolValueOr returns the dereferenced bool or the provided fallback when the pointer is nil.
func BoolValueOr(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

// TimeValue returns the dereferenced time or the zero value when the pointer is nil.
func TimeValue(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}
