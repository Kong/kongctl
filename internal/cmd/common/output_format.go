package common

import (
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// BaseOutputFormats are the --output values accepted by every command.
var BaseOutputFormats = []string{"json", "yaml", "text"}

// AllowExtraOutputFormats marks cobraCmd as accepting these additional
// --output values beyond the base set. Values are stored as a comma-separated
// list in the command's Annotations under ExtraOutputFormatsAnnotation and
// are merged with any previously-declared extras.
func AllowExtraOutputFormats(cmd *cobra.Command, formats ...string) {
	if cmd == nil || len(formats) == 0 {
		return
	}
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	seen := map[string]struct{}{}
	merged := []string{}
	if existing := cmd.Annotations[ExtraOutputFormatsAnnotation]; existing != "" {
		for v := range strings.SplitSeq(existing, ",") {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			if _, ok := seen[v]; !ok {
				seen[v] = struct{}{}
				merged = append(merged, v)
			}
		}
	}
	for _, v := range formats {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			merged = append(merged, v)
		}
	}
	cmd.Annotations[ExtraOutputFormatsAnnotation] = strings.Join(merged, ",")
}

// SkipOutputFormatValidation marks cmd (and its descendants) as opting out
// of root-level --output validation. Use this for commands that do their own
// flag handling (e.g. plan, scaffold) so they can surface command-specific
// error messages instead of the generic "invalid value" error.
func SkipOutputFormatValidation(cmd *cobra.Command) {
	if cmd == nil {
		return
	}
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations[SkipOutputFormatValidationAnnotation] = "true"
}

// IsOutputFormatValidationSkipped reports whether cmd or any of its ancestors
// has opted out of root-level --output validation.
func IsOutputFormatValidationSkipped(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Annotations[SkipOutputFormatValidationAnnotation] == "true" {
			return true
		}
	}
	return false
}

// AllowedOutputFormats returns the full list of --output values accepted by
// cmd: the base set plus any extras declared via AllowExtraOutputFormats on
// cmd itself or any of its ancestors.
func AllowedOutputFormats(cmd *cobra.Command) []string {
	allowed := slices.Clone(BaseOutputFormats)
	seen := map[string]struct{}{}
	for _, v := range allowed {
		seen[v] = struct{}{}
	}
	for c := cmd; c != nil; c = c.Parent() {
		extras, ok := c.Annotations[ExtraOutputFormatsAnnotation]
		if !ok || extras == "" {
			continue
		}
		for v := range strings.SplitSeq(extras, ",") {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			if _, dup := seen[v]; dup {
				continue
			}
			seen[v] = struct{}{}
			allowed = append(allowed, v)
		}
	}
	return allowed
}

// ValidateOutputFormat returns nil if value is in the base set or in the
// extras declared on cmd or one of its ancestors. Commands that have opted
// out via SkipOutputFormatValidation are always considered valid here.
func ValidateOutputFormat(cmd *cobra.Command, value string) error {
	if IsOutputFormatValidationSkipped(cmd) {
		return nil
	}
	allowed := AllowedOutputFormats(cmd)
	if slices.Contains(allowed, value) {
		return nil
	}
	return fmt.Errorf("invalid value %q for --%s, must be one of %v",
		value, OutputFlagName, allowed)
}
