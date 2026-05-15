package supportdata

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kong/kong-deployment-toolkit/pkg/collector"
)

// TestAllConfigFieldsMapped uses reflection to verify that every field in
// collector.Config is either:
//   - mapped to a kongctl flag or config path, OR
//   - explicitly listed as intentionally unmapped.
//
// This test acts as a living document of the flag parity between dct and
// kongctl. If a new field is added to collector.Config, this test will fail
// until the field is either mapped or added to the exclusion list with a
// documented reason.
func TestAllConfigFieldsMapped(t *testing.T) {
	// Fields that kongctl intentionally does NOT expose as flags.
	// Each entry documents why the field is excluded.
	intentionallyUnmapped := map[string]string{
		// DumpConfig is an internal deck configuration struct; kongctl
		// does not need to expose its individual fields as CLI flags.
		"DumpConfig": "internal deck dump.Config; not exposed as CLI flags",

		// Logger is set programmatically based on --log-level, not via
		// a user-facing flag.
		"Logger": "set programmatically from --log-level, not a user flag",

		// Debug is handled by kongctl's --log-level debug|trace flag
		// rather than a separate --debug flag.
		"Debug": "mapped to kongctl --log-level debug|trace",
	}

	// Fields mapped to kongctl flags or config, grouped by command.
	// The value describes how the field is exposed.
	mappedFields := map[string]string{
		// Common flags (on both on-prem and konnect)
		"OutputDir":           "--output-dir flag / config support_data.output_dir",
		"SanitizeConfigs":     "--sanitize flag / config support_data.sanitize",
		"LineLimit":           "--line-limit flag / config support_data.line_limit",
		"DockerLogsSince":     "--logs-since flag (unified) / config support_data.logs_since",
		"K8sLogsSinceSeconds": "--logs-since flag (unified, converted to seconds)",
		"RedactTerms":         "--redact flag / config support_data.redact_terms",
		"DisableKDD":          "--disable-kdd flag / config support_data.disable_kdd",
		"DumpWorkspaceConfigs": "--dump-workspaces flag / config support_data.dump_workspace_configs",

		// Runtime flags (on both on-prem and konnect)
		"Runtime":      "--runtime flag / config support_data.{on_prem,konnect}.runtime",
		"KongAddr":     "--kong-addr (on-prem) or --base-url (konnect)",
		"RBACHeaders":  "--rbac-header -H (on-prem) or --pat (konnect, mapped to RBACHeaders[0])",
		"PrefixDir":    "--prefix-dir -k flag / config support_data.{on_prem,konnect}.prefix_dir",
		"TargetImages": "--target-images flag / config support_data.{on_prem,konnect}.target_images",
		"TargetPods":   "--target-pods flag / config support_data.{on_prem,konnect}.target_pods",
		"Namespace":    "--namespace -n flag / config support_data.{on_prem,konnect}.namespace",

		// Konnect specific flags
		"KonnectMode":             "implicit: using konnect subcommand sets this to true",
		"KonnectControlPlaneName": "--control-plane flag / config support_data.konnect.control_plane",
	}

	configType := reflect.TypeOf(collector.Config{})
	for i := range configType.NumField() {
		field := configType.Field(i)
		name := field.Name

		_, isMapped := mappedFields[name]
		_, isExcluded := intentionallyUnmapped[name]

		assert.True(t, isMapped || isExcluded,
			"collector.Config field %q is not mapped to a kongctl flag/config "+
				"and not listed as intentionally unmapped. "+
				"Either add a flag mapping or document why it is excluded.",
			name)

		// A field should not appear in both maps
		assert.False(t, isMapped && isExcluded,
			"collector.Config field %q appears in both mapped and excluded lists",
			name)
	}

	// Verify our maps don't reference fields that don't exist
	for name := range mappedFields {
		_, found := configType.FieldByName(name)
		assert.True(t, found,
			"mappedFields references %q which does not exist in collector.Config",
			name)
	}
	for name := range intentionallyUnmapped {
		_, found := configType.FieldByName(name)
		assert.True(t, found,
			"intentionallyUnmapped references %q which does not exist in collector.Config",
			name)
	}
}

// TestFlagNameMapping documents the intentional renaming of dct flags
// in kongctl and verifies the commands expose the expected flag names.
func TestFlagNameMapping(t *testing.T) {
	onPremCmd := NewOnPremCmd()
	konnectCmd := NewKonnectCmd()

	// dct flag -> kongctl on-prem flag name
	onPremMappings := map[string]string{
		"--runtime":                "runtime",
		"--kong-addr":              "kong-addr",
		"--rbac-header":            "rbac-header",
		"--prefix-dir":             "prefix-dir",
		"--target-images":          "target-images",
		"--target-pods":            "target-pods",
		"--namespace":              "namespace",
		"--sanitize":               "sanitize",
		"--line-limit":             "line-limit",
		"--disable-kdd":            "disable-kdd",
		"--dump-workspace-configs": "dump-workspaces",
		"--redact-logs":            "redact",
	}

	for dctFlag, kongctlFlag := range onPremMappings {
		t.Run("on-prem/"+dctFlag, func(t *testing.T) {
			f := onPremCmd.Flags().Lookup(kongctlFlag)
			assert.NotNil(t, f,
				"dct %s should map to kongctl on-prem --%s", dctFlag, kongctlFlag)
		})
	}

	// dct flag -> kongctl konnect flag name
	konnectMappings := map[string]string{
		"--konnect-control-plane-name": "control-plane",
		"--runtime":                    "runtime",
		"--target-images":              "target-images",
		"--target-pods":                "target-pods",
		"--namespace":                  "namespace",
		"--prefix-dir":                 "prefix-dir",
		"--sanitize":                   "sanitize",
		"--line-limit":                 "line-limit",
		"--disable-kdd":                "disable-kdd",
		"--dump-workspace-configs":     "dump-workspaces",
		"--redact-logs":                "redact",
	}

	for dctFlag, kongctlFlag := range konnectMappings {
		t.Run("konnect/"+dctFlag, func(t *testing.T) {
			f := konnectCmd.Flags().Lookup(kongctlFlag)
			assert.NotNil(t, f,
				"dct %s should map to kongctl konnect --%s", dctFlag, kongctlFlag)
		})
	}

	// Flags that kongctl adds beyond dct
	konnectNewFlags := []string{"pat", "base-url", "region", "output-dir"}
	for _, flagName := range konnectNewFlags {
		t.Run("konnect-new/"+flagName, func(t *testing.T) {
			f := konnectCmd.Flags().Lookup(flagName)
			assert.NotNil(t, f,
				"kongctl konnect should have new flag --%s", flagName)
		})
	}

	onPremNewFlags := []string{"output-dir"}
	for _, flagName := range onPremNewFlags {
		t.Run("on-prem-new/"+flagName, func(t *testing.T) {
			f := onPremCmd.Flags().Lookup(flagName)
			assert.NotNil(t, f,
				"kongctl on-prem should have new flag --%s", flagName)
		})
	}

	// Flags intentionally removed from kongctl
	removedFlags := []string{"konnect-mode", "debug", "docker-since", "k8s-since-seconds"}
	for _, flagName := range removedFlags {
		t.Run("removed/on-prem/"+flagName, func(t *testing.T) {
			f := onPremCmd.Flags().Lookup(flagName)
			assert.Nil(t, f,
				"dct --%s should NOT be on kongctl on-prem (handled differently)", flagName)
		})
		t.Run("removed/konnect/"+flagName, func(t *testing.T) {
			f := konnectCmd.Flags().Lookup(flagName)
			assert.Nil(t, f,
				"dct --%s should NOT be on kongctl konnect (handled differently)", flagName)
		})
	}
}
