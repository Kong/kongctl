package extensions

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

type CompatibilityResult struct {
	Compatible     bool
	Unknown        bool
	CurrentVersion string
	Constraint     string
}

func ValidateCompatibility(compatibility Compatibility) error {
	_, err := buildCompatibilityConstraint(compatibility)
	return err
}

func CheckCompatibility(manifest Manifest, cliVersion string) (CompatibilityResult, error) {
	constraint, err := buildCompatibilityConstraint(manifest.Compatibility)
	if err != nil {
		return CompatibilityResult{}, err
	}
	result := CompatibilityResult{
		Compatible:     true,
		CurrentVersion: strings.TrimSpace(cliVersion),
		Constraint:     constraint,
	}
	if constraint == "" {
		return result, nil
	}
	if isUnknownCLIVersion(cliVersion) {
		result.Unknown = true
		return result, nil
	}

	version, err := semver.NewVersion(cliVersion)
	if err != nil {
		return CompatibilityResult{}, fmt.Errorf("parse kongctl version %q: %w", cliVersion, err)
	}
	parsed, err := semver.NewConstraint(constraint)
	if err != nil {
		return CompatibilityResult{}, fmt.Errorf("parse compatibility constraint %q: %w", constraint, err)
	}
	parsed.IncludePrerelease = true
	result.Compatible = parsed.Check(version)
	return result, nil
}

func EnsureCompatible(manifest Manifest, cliVersion string) error {
	result, err := CheckCompatibility(manifest, cliVersion)
	if err != nil {
		return err
	}
	if result.Compatible {
		return nil
	}
	return fmt.Errorf(
		"extension %s is not compatible with this kongctl version\n\nRequired: %s\nCurrent:  %s",
		ExtensionID(manifest.Publisher, manifest.Name),
		result.Constraint,
		result.CurrentVersion,
	)
}

func buildCompatibilityConstraint(compatibility Compatibility) (string, error) {
	minVersion := strings.TrimSpace(compatibility.MinVersion)
	maxVersion := strings.TrimSpace(compatibility.MaxVersion)
	parts := make([]string, 0, 2)

	if minVersion != "" {
		if _, err := semver.NewVersion(minVersion); err != nil {
			return "", fmt.Errorf("compatibility.min_version %q is not a valid semantic version: %w", minVersion, err)
		}
		parts = append(parts, ">= "+minVersion)
	}
	if maxVersion != "" {
		if compatibilityVersionHasWildcard(maxVersion) {
			if _, err := semver.NewConstraint(maxVersion); err != nil {
				return "", fmt.Errorf("compatibility.max_version %q is not a valid semantic version range: %w",
					maxVersion,
					err,
				)
			}
			parts = append(parts, maxVersion)
		} else {
			if _, err := semver.NewVersion(maxVersion); err != nil {
				return "", fmt.Errorf("compatibility.max_version %q is not a valid semantic version: %w",
					maxVersion,
					err,
				)
			}
			parts = append(parts, "<= "+maxVersion)
		}
	}

	constraint := strings.Join(parts, ", ")
	if constraint == "" {
		return "", nil
	}
	parsed, err := semver.NewConstraint(constraint)
	if err != nil {
		return "", fmt.Errorf("compatibility range %q is invalid: %w", constraint, err)
	}
	parsed.IncludePrerelease = true
	if minVersion != "" {
		version, err := semver.NewVersion(minVersion)
		if err != nil {
			return "", fmt.Errorf("compatibility.min_version %q is not a valid semantic version: %w", minVersion, err)
		}
		if !parsed.Check(version) {
			return "", fmt.Errorf("compatibility range %q does not include min_version %q", constraint, minVersion)
		}
	}
	return constraint, nil
}

func compatibilityVersionHasWildcard(version string) bool {
	return strings.ContainsAny(version, "xX*")
}

func isUnknownCLIVersion(version string) bool {
	version = strings.TrimSpace(strings.ToLower(version))
	return version == "" || version == "dev" || version == "unknown"
}
