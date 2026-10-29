package helpers

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
)

var (
	meterKeyPattern        = regexp.MustCompile(`^[a-z0-9]+(?:_[a-z0-9]+)*$`)
	meterULIDPattern       = regexp.MustCompile(`^[0-7][0-9A-HJKMNP-TV-Z]{25}$`)
	meterLabelValuePattern = regexp.MustCompile(`^[a-z0-9A-Z]([a-z0-9A-Z-._]*[a-z0-9A-Z])?$`)
	meterDimensionPattern  = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
)

func IsMeterULID(value string) bool { return meterULIDPattern.MatchString(value) }

// ValidateMeterLabels follows the M&B public Labels schema. Reserved prefixes
// are lowercase; the existing uppercase KONGCTL-* management keys are allowed.
func ValidateMeterLabels(labels map[string]string) error {
	if len(labels) > 50 {
		return fmt.Errorf("meter labels must contain at most 50 entries, including management labels")
	}
	for key, value := range labels {
		if len(key) < 1 || len(key) > 63 {
			return fmt.Errorf("meter label keys must be 1-63 characters")
		}
		for _, prefix := range []string{"kong", "konnect", "mesh", "kic", "_"} {
			if strings.HasPrefix(key, prefix) {
				return fmt.Errorf("meter label key %q uses reserved prefix %q", key, prefix)
			}
		}
		if len(value) < 1 || len(value) > 63 || !meterLabelValuePattern.MatchString(value) {
			return fmt.Errorf(
				"meter label %q must have a 1-63 character value with alphanumeric ends "+
					"and only alphanumerics, hyphens, dots or underscores",
				key,
			)
		}
	}
	return nil
}

func ValidateMeterCreate(req kkComps.CreateMeterRequest) error {
	if req.Name == "" {
		return fmt.Errorf("meter name is required")
	}
	if req.Key == "" {
		return fmt.Errorf("meter key is required")
	}
	if len(req.Key) > 64 || !meterKeyPattern.MatchString(req.Key) {
		return fmt.Errorf("meter key must be 1-64 lowercase alphanumeric characters separated by single underscores")
	}
	if req.ValueProperty != nil && *req.ValueProperty == "" {
		return fmt.Errorf("meter value_property must not be empty")
	}
	if req.EventType == "" {
		return fmt.Errorf("meter event_type is required")
	}
	if !req.Aggregation.IsExact() {
		return fmt.Errorf("meter aggregation must be sum, count, unique_count, avg, min, max or latest")
	}
	return ValidateMeterUpdate(
		kkComps.UpdateMeterRequest{
			Name:        &req.Name,
			Description: req.Description,
			Labels:      req.Labels,
			Dimensions:  req.Dimensions,
		},
	)
}

func ValidateMeterUpdate(req kkComps.UpdateMeterRequest) error {
	if req.Name != nil && (utf8.RuneCountInString(*req.Name) < 1 || utf8.RuneCountInString(*req.Name) > 256) {
		return fmt.Errorf("meter name must be 1-256 characters")
	}
	if req.Description != nil && utf8.RuneCountInString(*req.Description) > 1024 {
		return fmt.Errorf("meter description must be at most 1024 characters")
	}
	for key := range req.Dimensions {
		if !meterDimensionPattern.MatchString(key) {
			return fmt.Errorf("meter dimension keys must contain only alphanumerics and underscores")
		}
	}
	return ValidateMeterLabels(req.Labels)
}
