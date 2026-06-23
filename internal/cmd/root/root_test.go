package root

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/common"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	configpkg "github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/log"
	"github.com/kong/kongctl/internal/profile"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestMergedFlagUsagesUsesCommandSpecificOutputFormats(t *testing.T) {
	output := cmdpkg.NewDeferredEnum([]string{
		common.JSON.String(),
		common.YAML.String(),
		common.TEXT.String(),
	}, common.TEXT.String())

	rootCmd := &cobra.Command{Use: "root"}
	rootCmd.PersistentFlags().VarP(output, common.OutputFlagName, common.OutputFlagShort,
		outputFlagUsage(output.Allowed))

	childCmd := &cobra.Command{Use: "child"}
	rootCmd.AddCommand(childCmd)
	common.AllowExtraOutputFormats(childCmd, common.HELM.String())

	rootUsage := mergedFlagUsages(rootCmd)
	if !strings.Contains(rootUsage, "Allowed    : [ json|yaml|text ]") {
		t.Fatalf("expected root usage to show base output formats, got:\n%s", rootUsage)
	}
	if strings.Contains(rootUsage, "json|yaml|text|helm") {
		t.Fatalf("expected root usage not to show helm, got:\n%s", rootUsage)
	}

	childUsage := mergedFlagUsages(childCmd)
	if !strings.Contains(childUsage, "Allowed    : [ json|yaml|text|helm ]") {
		t.Fatalf("expected child usage to show command-specific helm format, got:\n%s", childUsage)
	}

	outputFlag := rootCmd.PersistentFlags().Lookup(common.OutputFlagName)
	if outputFlag == nil {
		t.Fatal("expected root output flag")
		return
	}
	if strings.Contains(outputFlag.Usage, "helm") {
		t.Fatalf("expected merged usage rendering not to mutate root output flag usage, got:\n%s", outputFlag.Usage)
	}
}

func TestOutputFlagHelpVisibility(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantOut   []string
		forbidOut []string
	}{
		{
			name: "plan hides unsupported inherited output flag",
			args: []string{"plan", "--help"},
			forbidOut: []string{
				"-o, --output string",
				"Allowed    : [ json|yaml|text ]",
			},
		},
		{
			name: "plan konnect hides unsupported inherited output flag",
			args: []string{"plan", "konnect", "--help"},
			forbidOut: []string{
				"-o, --output string",
				"Allowed    : [ json|yaml|text ]",
			},
		},
		{
			name: "scaffold hides unsupported inherited output flag",
			args: []string{"scaffold", "--help"},
			forbidOut: []string{
				"-o, --output string",
				"Allowed    : [ json|yaml|text ]",
			},
		},
		{
			name: "dump hides unsupported inherited output flag",
			args: []string{"dump", "--help"},
			forbidOut: []string{
				"-o, --output string",
				"Allowed    : [ json|yaml|text ]",
			},
		},
		{
			name: "dump declarative hides unsupported inherited output flag",
			args: []string{"dump", "declarative", "--help"},
			forbidOut: []string{
				"-o, --output string",
				"Allowed    : [ json|yaml|text ]",
			},
		},
		{
			name: "dump tf-import hides unsupported inherited output flag",
			args: []string{"dump", "tf-import", "--help"},
			forbidOut: []string{
				"-o, --output string",
				"Allowed    : [ json|yaml|text ]",
			},
		},
		{
			name: "explain keeps supported output flag",
			args: []string{"explain", "--help"},
			wantOut: []string{
				"-o, --output string",
				"Allowed    : [ json|yaml|text ]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executeRootForTest(t, tt.args...)
			if result.exitCode != 0 {
				t.Fatalf("expected exit code 0, got %d\nstdout:\n%s\nstderr:\n%s",
					result.exitCode, result.stdout, result.stderr)
			}
			for _, want := range tt.wantOut {
				if !strings.Contains(result.stdout, want) {
					t.Fatalf("expected stdout to contain %q\nstdout:\n%s", want, result.stdout)
				}
			}
			for _, forbidden := range tt.forbidOut {
				if strings.Contains(result.stdout, forbidden) {
					t.Fatalf("expected stdout not to contain %q\nstdout:\n%s", forbidden, result.stdout)
				}
			}
		})
	}
}

func TestDumpRejectsOutputFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "long output flag on declarative dump",
			args: []string{"dump", "declarative", "--resources=apis", "--output", "json"},
		},
		{
			name: "missing long output flag value on declarative dump",
			args: []string{"dump", "declarative", "--resources=apis", "--output"},
		},
		{
			name: "short output flag on declarative dump",
			args: []string{"dump", "declarative", "--resources=apis", "-o", "json"},
		},
		{
			name: "long output flag on terraform import dump",
			args: []string{"dump", "tf-import", "--resources=portal", "--output", "json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executeRootForTest(t, tt.args...)
			if result.exitCode == 0 {
				t.Fatalf("expected command to fail\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
			}
			want := "flags -o/--output are not supported for the dump command; " +
				"use --output-file to save dump output to a file"
			if !strings.Contains(result.stderr, want) {
				t.Fatalf("expected stderr to contain %q\nstderr:\n%s", want, result.stderr)
			}
		})
	}
}

func TestDumpPreservesNonOutputFlagParseErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "declarative output-file missing value",
			args: []string{"dump", "declarative", "--resources=apis", "--output-file"},
			want: "flag needs an argument: --output-file",
		},
		{
			name: "tf-import output-file missing value",
			args: []string{"dump", "tf-import", "--resources=portal", "--output-file"},
			want: "flag needs an argument: --output-file",
		},
		{
			name: "unknown output-prefixed flag",
			args: []string{"dump", "declarative", "--resources=apis", "--output-format=json"},
			want: "unknown flag: --output-format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executeRootForTest(t, tt.args...)
			if result.exitCode == 0 {
				t.Fatalf("expected command to fail\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
			}
			if !strings.Contains(result.stderr, tt.want) {
				t.Fatalf("expected stderr to contain %q\nstderr:\n%s", tt.want, result.stderr)
			}
			if strings.Contains(result.stderr, outputFlagUnsupportedMsgForTest) {
				t.Fatalf("expected stderr not to contain output flag unsupported message\nstderr:\n%s", result.stderr)
			}
		})
	}
}

const outputFlagUnsupportedMsgForTest = "flags -o/--output are not supported for the dump command"

func TestKonnectFirstHelpExamplesMatchExplicitTarget(t *testing.T) {
	tests := []struct {
		name         string
		shorthand    []string
		explicitForm []string
	}{
		{
			name:         "apply",
			shorthand:    []string{"apply", "--help"},
			explicitForm: []string{"apply", "konnect", "--help"},
		},
		{
			name:         "diff",
			shorthand:    []string{"diff", "--help"},
			explicitForm: []string{"diff", "konnect", "--help"},
		},
		{
			name:         "plan",
			shorthand:    []string{"plan", "--help"},
			explicitForm: []string{"plan", "konnect", "--help"},
		},
		{
			name:         "sync",
			shorthand:    []string{"sync", "--help"},
			explicitForm: []string{"sync", "konnect", "--help"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shorthand := executeRootForTest(t, tt.shorthand...)
			explicitForm := executeRootForTest(t, tt.explicitForm...)
			if shorthand.exitCode != 0 || explicitForm.exitCode != 0 {
				t.Fatalf("expected help commands to succeed\nshorthand:\n%s\n%s\nexplicit:\n%s\n%s",
					shorthand.stdout, shorthand.stderr, explicitForm.stdout, explicitForm.stderr)
			}

			shorthandExamples := helpSectionForTest(t, shorthand.stdout, "Examples:")
			explicitExamples := helpSectionForTest(t, explicitForm.stdout, "Examples:")
			if shorthandExamples != explicitExamples {
				t.Fatalf("expected shorthand and explicit examples to match\nshorthand:\n%s\nexplicit:\n%s",
					shorthandExamples, explicitExamples)
			}
		})
	}
}

func TestDeleteHelpUsesDeclarativeExamples(t *testing.T) {
	result := executeRootForTest(t, "delete", "--help")
	if result.exitCode != 0 {
		t.Fatalf("expected delete help to succeed, got %d\nstdout:\n%s\nstderr:\n%s",
			result.exitCode, result.stdout, result.stderr)
	}

	examples := helpSectionForTest(t, result.stdout, "Examples:")
	for _, want := range []string{
		"# Delete Konnect resources defined in declarative configuration",
		"kongctl delete -f config.yaml",
		"# Preview deletions before executing them",
		"kongctl delete -f config.yaml --dry-run",
		"# Execute a reviewed delete plan without prompting",
		"kongctl delete --plan delete-plan.json --auto-approve",
	} {
		if !strings.Contains(examples, want) {
			t.Fatalf("expected delete examples to contain %q\nexamples:\n%s", want, examples)
		}
	}
	if strings.Contains(examples, "kongctl delete -f ./configs/ --recursive") {
		t.Fatalf("expected delete examples not to contain stale recursive example\nexamples:\n%s", examples)
	}
}

func TestCreateCommandIsTokenOnly(t *testing.T) {
	result := executeRootForTest(t, "create", "gateway", "control-plane", "cp")
	if result.exitCode == 0 {
		t.Fatalf("expected create gateway control-plane to fail\nstdout:\n%s", result.stdout)
	}
	if !strings.Contains(result.stderr, `unknown command "gateway"`) {
		t.Fatalf("expected unknown gateway command\nstderr:\n%s", result.stderr)
	}

	result = executeRootForTest(t, "create", "konnect", "gateway", "control-plane", "cp")
	if result.exitCode == 0 {
		t.Fatalf("expected create konnect gateway control-plane to fail\nstdout:\n%s", result.stdout)
	}
	if !strings.Contains(result.stderr, `unknown command "gateway"`) {
		t.Fatalf("expected unknown gateway command under konnect\nstderr:\n%s", result.stderr)
	}
}

func TestCreatePATHelpIncludesTokenExamplesAndFormats(t *testing.T) {
	result := executeRootForTest(t, "create", "pat", "--help")
	if result.exitCode != 0 {
		t.Fatalf("expected create pat help to succeed, got %d\nstdout:\n%s\nstderr:\n%s",
			result.exitCode, result.stdout, result.stderr)
	}
	for _, want := range []string{
		"kongctl create pat --name ci --expires-in 30d -o token",
		"kongctl create pat --name ci --expires-in 7d --jq -r '.token'",
		"Use a duration between 1 day and 365 days (12 months).",
		"Supported units are ns, us, ms, s, m, h, and d (days).",
		"Examples: 24h, 36h, 1d, 30d.",
		"Token expiration timestamp in RFC3339 format, for example 2026-06-24T12:00:00Z",
		"or 2026-06-24T12:00:00+02:00. Fractional seconds are accepted.",
		"Must be between 1 day and 365 days (12 months) from now.",
		"Allowed    : [ json|yaml|text|token|env ]",
	} {
		if !strings.Contains(result.stdout, want) {
			t.Fatalf("expected help to contain %q\nstdout:\n%s", want, result.stdout)
		}
	}
}

func TestCreatePATRejectsBelowMinDuration(t *testing.T) {
	result := executeRootForTest(t, "create", "pat", "--name", "ci", "--expires-in", "12h")
	if result.exitCode == 0 {
		t.Fatalf("expected create pat with below-min duration to fail\nstdout:\n%s", result.stdout)
	}
	for _, want := range []string{
		"minimum token lifetime is 1 day",
		"--expires-in must be at least 1d",
	} {
		if !strings.Contains(result.stderr, want) {
			t.Fatalf("expected stderr to contain %q\nstderr:\n%s", want, result.stderr)
		}
	}
}

func TestCreatePATRejectsOverMaxDuration(t *testing.T) {
	result := executeRootForTest(t, "create", "pat", "--name", "ci", "--expires-in", "366d")
	if result.exitCode == 0 {
		t.Fatalf("expected create pat with over-max duration to fail\nstdout:\n%s", result.stdout)
	}
	for _, want := range []string{
		"maximum token lifetime is 365 days (12 months)",
		"--expires-in must be at most 365d",
	} {
		if !strings.Contains(result.stderr, want) {
			t.Fatalf("expected stderr to contain %q\nstderr:\n%s", want, result.stderr)
		}
	}
}

func TestCreatePATRejectsExpiresAtOutsideBounds(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt string
		want      []string
	}{
		{
			name:      "too soon",
			expiresAt: time.Now().UTC().Add(12 * time.Hour).Format(time.RFC3339),
			want: []string{
				"minimum token lifetime is 1 day",
				"--expires-at must be at least 1 day from now",
			},
		},
		{
			name:      "too far",
			expiresAt: time.Now().UTC().Add(366 * 24 * time.Hour).Format(time.RFC3339),
			want: []string{
				"maximum token lifetime is 365 days (12 months)",
				"--expires-at must be at most 365 days from now",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executeRootForTest(t, "create", "pat", "--name", "ci", "--expires-at", tt.expiresAt)
			if result.exitCode == 0 {
				t.Fatalf("expected create pat with %s expires-at to fail\nstdout:\n%s", tt.name, result.stdout)
			}
			for _, want := range tt.want {
				if !strings.Contains(result.stderr, want) {
					t.Fatalf("expected stderr to contain %q\nstderr:\n%s", want, result.stderr)
				}
			}
		})
	}
}

func TestCreatePATTokenOutputAndJQ(t *testing.T) {
	patAPI := &rootTestPATAPI{token: "kpat_123", id: "pat-id"}
	installRootTokenSDK(t, &helpers.MockKonnectSDK{
		MeFactory: func() helpers.MeAPI {
			return rootTestMeAPI{userID: "user-1"}
		},
		PersonalAccessTokenFactory: func() helpers.PersonalAccessTokenAPI {
			return patAPI
		},
	})

	result := executeRootForTest(
		t,
		"create", "pat",
		"--name", "ci",
		"--expires-in", "24h",
		"-o", "token",
	)
	if result.exitCode != 0 {
		t.Fatalf("expected token output to succeed\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if result.stdout != "kpat_123\n" {
		t.Fatalf("expected token-only stdout, got %q", result.stdout)
	}
	if patAPI.createdUserID != "user-1" {
		t.Fatalf("expected current user id to be used, got %q", patAPI.createdUserID)
	}
	if got := patAPI.createdRequest.PersonalAccessTokenCreateRequestWithTTL.GetTTLSeconds(); got != 86400 {
		t.Fatalf("expected 24h ttl to be 86400 seconds, got %d", got)
	}

	result = executeRootForTest(
		t,
		"create", "pat",
		"--name", "ci",
		"--expires-in", "7d",
		"--user-id", "user-2",
		"--jq", "-r", ".token",
	)
	if result.exitCode != 0 {
		t.Fatalf("expected jq output to succeed\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if result.stdout != "kpat_123\n" {
		t.Fatalf("expected jq raw token stdout, got %q", result.stdout)
	}
	if patAPI.createdUserID != "user-2" {
		t.Fatalf("expected explicit user id to be used, got %q", patAPI.createdUserID)
	}
}

func TestCreateSPATEnvOutputResolvesSystemAccountName(t *testing.T) {
	spatToken := "spat_123"
	accountID := "9d0462e0-6a6b-4811-9b37-0ad7dd48d9f1"
	installRootTokenSDK(t, &helpers.MockKonnectSDK{
		SystemAccountFactory: func() helpers.SystemAccountAPI {
			return rootTestSystemAccountAPI{id: accountID, name: "ci-bot"}
		},
		SystemAccountAccessTokenFactory: func() helpers.SystemAccountAccessTokenAPI {
			return &rootTestSPATAPI{token: spatToken}
		},
	})

	result := executeRootForTest(
		t,
		"--profile", "team-a",
		"create", "spat",
		"--system-account-name", "ci-bot",
		"--name", "ci",
		"--expires-in", "30d",
		"-o", "env",
	)
	if result.exitCode != 0 {
		t.Fatalf("expected env output to succeed\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if result.stdout != "export KONGCTL_TEAM_A_KONNECT_PAT='spat_123'\n" {
		t.Fatalf("unexpected env output: %q", result.stdout)
	}
}

func TestCreateSPATRejectsBelowMinDuration(t *testing.T) {
	result := executeRootForTest(
		t,
		"create", "spat",
		"--system-account-id", "system-account-id",
		"--name", "ci",
		"--expires-in", "12h",
	)
	if result.exitCode == 0 {
		t.Fatalf("expected create spat with below-min duration to fail\nstdout:\n%s", result.stdout)
	}
	for _, want := range []string{
		"minimum token lifetime is 1 day",
		"--expires-in must be at least 1d",
	} {
		if !strings.Contains(result.stderr, want) {
			t.Fatalf("expected stderr to contain %q\nstderr:\n%s", want, result.stderr)
		}
	}
}

func TestCreateSPATRejectsOverMaxDuration(t *testing.T) {
	result := executeRootForTest(
		t,
		"create", "spat",
		"--system-account-id", "system-account-id",
		"--name", "ci",
		"--expires-in", "366d",
	)
	if result.exitCode == 0 {
		t.Fatalf("expected create spat with over-max duration to fail\nstdout:\n%s", result.stdout)
	}
	for _, want := range []string{
		"maximum token lifetime is 365 days (12 months)",
		"--expires-in must be at most 365d",
	} {
		if !strings.Contains(result.stderr, want) {
			t.Fatalf("expected stderr to contain %q\nstderr:\n%s", want, result.stderr)
		}
	}
}

func TestCreateSPATRejectsExpiresAtOutsideBounds(t *testing.T) {
	tests := []struct {
		name      string
		expiresAt string
		want      []string
	}{
		{
			name:      "too soon",
			expiresAt: time.Now().UTC().Add(12 * time.Hour).Format(time.RFC3339),
			want: []string{
				"minimum token lifetime is 1 day",
				"--expires-at must be at least 1 day from now",
			},
		},
		{
			name:      "too far",
			expiresAt: time.Now().UTC().Add(366 * 24 * time.Hour).Format(time.RFC3339),
			want: []string{
				"maximum token lifetime is 365 days (12 months)",
				"--expires-at must be at most 365 days from now",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executeRootForTest(
				t,
				"create", "spat",
				"--system-account-id", "system-account-id",
				"--name", "ci",
				"--expires-at", tt.expiresAt,
			)
			if result.exitCode == 0 {
				t.Fatalf("expected create spat with %s expires-at to fail\nstdout:\n%s", tt.name, result.stdout)
			}
			for _, want := range tt.want {
				if !strings.Contains(result.stderr, want) {
					t.Fatalf("expected stderr to contain %q\nstderr:\n%s", want, result.stderr)
				}
			}
		})
	}
}

func TestPATGetAndDeleteCommands(t *testing.T) {
	patID := "11111111-1111-1111-1111-111111111111"
	patAPI := &rootTestPATAPI{
		tokens: []kkComps.PersonalAccessToken{rootTestPAT(patID, "ci")},
	}
	installRootTokenSDK(t, &helpers.MockKonnectSDK{
		MeFactory: func() helpers.MeAPI {
			return rootTestMeAPI{userID: "user-1"}
		},
		PersonalAccessTokenFactory: func() helpers.PersonalAccessTokenAPI {
			return patAPI
		},
	})

	result := executeRootForTest(t, "get", "pat", "--user-id", "user-1", "-o", "json")
	if result.exitCode != 0 {
		t.Fatalf("expected get pat list to succeed\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if !strings.Contains(result.stdout, patID) || !strings.Contains(result.stdout, `"name": "ci"`) {
		t.Fatalf("expected PAT list output to include token metadata\nstdout:\n%s", result.stdout)
	}
	if strings.Contains(result.stdout, `"token"`) {
		t.Fatalf("expected PAT list output not to include token values\nstdout:\n%s", result.stdout)
	}

	result = executeRootForTest(t, "get", "pat", "--user-id", "user-1", "--jq", ".[] | {id,name,expires_at}")
	if result.exitCode != 0 {
		t.Fatalf("expected get pat jq to succeed\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if !strings.Contains(result.stdout, `"expires_at"`) || !strings.Contains(result.stdout, patID) {
		t.Fatalf("expected jq output to include selected PAT fields\nstdout:\n%s", result.stdout)
	}

	result = executeRootForTest(t, "get", "pat", patID, "--user-id", "user-1", "-o", "json")
	if result.exitCode != 0 {
		t.Fatalf("expected get pat by id to succeed\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if patAPI.gotTokenID != patID {
		t.Fatalf("expected get by id to call detail API with %q, got %q", patID, patAPI.gotTokenID)
	}

	result = executeRootForTest(t, "get", "pat", "ci", "--user-id", "user-1", "-o", "json")
	if result.exitCode != 0 {
		t.Fatalf("expected get pat by name to succeed\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}

	result = executeRootForTest(t, "delete", "pat", patID, "--user-id", "user-1", "--auto-approve", "-o", "json")
	if result.exitCode != 0 {
		t.Fatalf("expected delete pat by id to succeed\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if patAPI.deletedTokenID != patID {
		t.Fatalf("expected delete by id to delete %q, got %q", patID, patAPI.deletedTokenID)
	}
	if !strings.Contains(result.stdout, `"status": "deleted"`) {
		t.Fatalf("expected delete output to include deleted status\nstdout:\n%s", result.stdout)
	}

	patAPI.deletedTokenID = ""
	result = executeRootForTest(t, "delete", "pat", "ci", "--user-id", "user-1", "--auto-approve", "-o", "json")
	if result.exitCode != 0 {
		t.Fatalf("expected delete pat by name to succeed\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if patAPI.deletedTokenID != patID {
		t.Fatalf("expected delete by name to delete %q, got %q", patID, patAPI.deletedTokenID)
	}
}

func TestSPATGetAndDeleteCommands(t *testing.T) {
	accountID := "9d0462e0-6a6b-4811-9b37-0ad7dd48d9f1"
	spatID := "22222222-2222-2222-2222-222222222222"
	spatAPI := &rootTestSPATAPI{
		tokens: []kkComps.SystemAccountAccessToken{rootTestSPAT(spatID, "ci")},
	}
	installRootTokenSDK(t, &helpers.MockKonnectSDK{
		SystemAccountFactory: func() helpers.SystemAccountAPI {
			return rootTestSystemAccountAPI{id: accountID, name: "ci-bot"}
		},
		SystemAccountAccessTokenFactory: func() helpers.SystemAccountAccessTokenAPI {
			return spatAPI
		},
	})

	result := executeRootForTest(t, "get", "spat", "--system-account-id", accountID, "-o", "json")
	if result.exitCode != 0 {
		t.Fatalf("expected get spat list to succeed\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if !strings.Contains(result.stdout, spatID) || !strings.Contains(result.stdout, `"system_account_id":`) {
		t.Fatalf("expected SPAT list output to include token metadata\nstdout:\n%s", result.stdout)
	}
	if strings.Contains(result.stdout, `"token"`) {
		t.Fatalf("expected SPAT list output not to include token values\nstdout:\n%s", result.stdout)
	}

	result = executeRootForTest(t, "get", "spat", spatID, "--system-account-id", accountID, "-o", "json")
	if result.exitCode != 0 {
		t.Fatalf("expected get spat by id to succeed\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if spatAPI.gotTokenID != spatID {
		t.Fatalf("expected get by id to call detail API with %q, got %q", spatID, spatAPI.gotTokenID)
	}

	result = executeRootForTest(t, "get", "spat", "ci", "--system-account-name", "ci-bot", "-o", "json")
	if result.exitCode != 0 {
		t.Fatalf("expected get spat by name to succeed\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if !strings.Contains(result.stdout, `"system_account_name": "ci-bot"`) {
		t.Fatalf("expected SPAT by name output to include resolved system account name\nstdout:\n%s", result.stdout)
	}

	result = executeRootForTest(
		t,
		"delete", "spat", "ci",
		"--system-account-name", "ci-bot",
		"--auto-approve",
		"-o", "json",
	)
	if result.exitCode != 0 {
		t.Fatalf("expected delete spat by name to succeed\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if spatAPI.deletedAccountID != accountID || spatAPI.deletedTokenID != spatID {
		t.Fatalf("expected delete spat to delete %s/%s, got %s/%s",
			accountID, spatID, spatAPI.deletedAccountID, spatAPI.deletedTokenID)
	}
	if !strings.Contains(result.stdout, `"status": "deleted"`) {
		t.Fatalf("expected delete output to include deleted status\nstdout:\n%s", result.stdout)
	}

	unnamedID := "55555555-5555-5555-5555-555555555555"
	spatAPI.tokens = append(spatAPI.tokens, kkComps.SystemAccountAccessToken{ID: &unnamedID})
	result = executeRootForTest(
		t,
		"delete", "spat", unnamedID,
		"--system-account-id", accountID,
		"--auto-approve",
		"-o", "text",
	)
	if result.exitCode != 0 {
		t.Fatalf("expected delete unnamed spat by id to succeed\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if result.stdout != fmt.Sprintf("Deleted spat %q\n", unnamedID) {
		t.Fatalf("expected unnamed SPAT delete text to use token id, got %q", result.stdout)
	}
}

func TestTokenGetEmptyTextOutput(t *testing.T) {
	accountID := "9d0462e0-6a6b-4811-9b37-0ad7dd48d9f1"
	installRootTokenSDK(t, &helpers.MockKonnectSDK{
		MeFactory: func() helpers.MeAPI {
			return rootTestMeAPI{userID: "user-1"}
		},
		PersonalAccessTokenFactory: func() helpers.PersonalAccessTokenAPI {
			return &rootTestPATAPI{}
		},
		SystemAccountFactory: func() helpers.SystemAccountAPI {
			return rootTestSystemAccountAPI{id: accountID, name: "ci-bot"}
		},
		SystemAccountAccessTokenFactory: func() helpers.SystemAccountAccessTokenAPI {
			return &rootTestSPATAPI{}
		},
	})

	result := executeRootForTest(t, "get", "pat", "--user-id", "user-1", "-o", "text")
	if result.exitCode != 0 {
		t.Fatalf("expected empty get pat to succeed\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if result.stdout != "No resources found.\n" {
		t.Fatalf("expected no resources message for empty PAT list, got %q", result.stdout)
	}

	result = executeRootForTest(t, "get", "spats", "--system-account-name", "ci-bot", "-o", "text")
	if result.exitCode != 0 {
		t.Fatalf("expected empty get spats to succeed\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if result.stdout != "No resources found.\n" {
		t.Fatalf("expected no resources message for empty SPAT list, got %q", result.stdout)
	}
}

func TestTokenNameResolutionErrors(t *testing.T) {
	accountID := "9d0462e0-6a6b-4811-9b37-0ad7dd48d9f1"
	patAPI := &rootTestPATAPI{
		tokens: []kkComps.PersonalAccessToken{
			rootTestPAT("11111111-1111-1111-1111-111111111111", "dupe"),
			rootTestPAT("33333333-3333-3333-3333-333333333333", "dupe"),
		},
	}
	spatAPI := &rootTestSPATAPI{
		tokens: []kkComps.SystemAccountAccessToken{
			rootTestSPAT("22222222-2222-2222-2222-222222222222", "dupe"),
			rootTestSPAT("44444444-4444-4444-4444-444444444444", "dupe"),
		},
	}
	installRootTokenSDK(t, &helpers.MockKonnectSDK{
		MeFactory: func() helpers.MeAPI {
			return rootTestMeAPI{userID: "user-1"}
		},
		PersonalAccessTokenFactory: func() helpers.PersonalAccessTokenAPI {
			return patAPI
		},
		SystemAccountFactory: func() helpers.SystemAccountAPI {
			return rootTestSystemAccountAPI{id: accountID, name: "ci-bot"}
		},
		SystemAccountAccessTokenFactory: func() helpers.SystemAccountAccessTokenAPI {
			return spatAPI
		},
	})

	result := executeRootForTest(t, "get", "pat", "missing", "--user-id", "user-1")
	if result.exitCode == 0 || !strings.Contains(result.stderr, `personal access token "missing" not found`) {
		t.Fatalf("expected PAT not found error\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}

	result = executeRootForTest(t, "delete", "pat", "dupe", "--user-id", "user-1", "--auto-approve")
	if result.exitCode == 0 || !strings.Contains(result.stderr, `multiple personal access tokens found`) {
		t.Fatalf("expected duplicate PAT name error\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}

	result = executeRootForTest(t, "get", "spat", "missing", "--system-account-id", accountID)
	if result.exitCode == 0 || !strings.Contains(result.stderr, `system account access token "missing" not found`) {
		t.Fatalf("expected SPAT not found error\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}

	result = executeRootForTest(t, "delete", "spat", "dupe", "--system-account-id", accountID, "--auto-approve")
	if result.exitCode == 0 || !strings.Contains(result.stderr, `multiple system account access tokens found`) {
		t.Fatalf("expected duplicate SPAT name error\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
}

func TestListProfilesMatchesGetProfiles(t *testing.T) {
	getResult := executeRootForTest(t, "get", "profiles", "--output", "json")
	if getResult.exitCode != 0 {
		t.Fatalf("expected get profiles to succeed, got %d\nstdout:\n%s\nstderr:\n%s",
			getResult.exitCode, getResult.stdout, getResult.stderr)
	}

	listResult := executeRootForTest(t, "list", "profiles", "--output", "json")
	if listResult.exitCode != 0 {
		t.Fatalf("expected list profiles to succeed, got %d\nstdout:\n%s\nstderr:\n%s",
			listResult.exitCode, listResult.stdout, listResult.stderr)
	}

	if listResult.stdout != getResult.stdout {
		t.Fatalf("expected list profiles output to match get profiles\nget:\n%s\nlist:\n%s",
			getResult.stdout, listResult.stdout)
	}
}

func TestRootApplyHelpShowsExamples(t *testing.T) {
	oldRootCmd := rootCmd
	t.Cleanup(func() {
		rootCmd = oldRootCmd
	})

	rootCmd = newRootCmd()
	requireNoError(t, addCommands())

	var output bytes.Buffer
	rootCmd.SetOut(&output)
	rootCmd.SetErr(&output)
	rootCmd.SetArgs([]string{"apply", "--help"})

	requireNoError(t, rootCmd.Execute())
	help := output.String()

	if !strings.Contains(help, "Examples:") {
		t.Fatalf("expected apply help to show examples, got:\n%s", help)
	}
	if !strings.Contains(help, "kongctl apply -f api.yaml") {
		t.Fatalf("expected apply help to show shorthand example, got:\n%s", help)
	}
	if !strings.Contains(help, "kongctl apply konnect -f api.yaml") {
		t.Fatalf("expected apply help to show explicit Konnect example, got:\n%s", help)
	}
	if strings.Contains(help, "kongctl get konnect gateway control-planes") {
		t.Fatalf("expected apply help not to show get control-planes example, got:\n%s", help)
	}
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateOutputFormatUsesResolvedConfigValue(t *testing.T) {
	oldConfig := currConfig
	oldOutputFormat := outputFormat
	t.Cleanup(func() {
		currConfig = oldConfig
		outputFormat = oldOutputFormat
	})

	outputFormat = cmdpkg.NewDeferredEnum([]string{
		common.JSON.String(),
		common.YAML.String(),
		common.TEXT.String(),
	}, common.TEXT.String())
	currConfig = configpkg.BuildProfiledConfig("default", "", viper.New())
	currConfig.SetString(common.OutputConfigPath, common.HELM.String())

	cmd := &cobra.Command{Use: "leaf"}
	if err := validateOutputFormat(cmd); err == nil {
		t.Fatal("expected helm from config to be rejected without command opt-in")
	}

	common.AllowExtraOutputFormats(cmd, common.HELM.String())
	if err := validateOutputFormat(cmd); err != nil {
		t.Fatalf("expected helm from config to be allowed with command opt-in: %v", err)
	}
}

func TestRootErrorUX(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantErr      []string
		wantOut      []string
		wantExit     int
		forbidErr    []string
		forbidOut    []string
		expectStderr bool
		expectStdout bool
	}{
		{
			name: "bare root shows help",
			args: []string{},
			wantOut: []string{
				`kongctl is the official command line tool for the Kong Konnect API platform.`,
				"Find more information at:",
				"Available Commands:",
			},
			wantExit:     0,
			forbidErr:    []string{"Error:"},
			forbidOut:    []string{"Flags:", "Usage:"},
			expectStdout: true,
		},
		{
			name: "bare command group requires subcommand",
			args: []string{"get"},
			wantErr: []string{
				`Error: command "kongctl get" requires a subcommand`,
				"Available subcommands:",
				"  api",
				"  konnect",
				`Run 'kongctl get --help' for usage`,
			},
			wantExit:     1,
			forbidErr:    []string{"Usage:"},
			expectStderr: true,
		},
		{
			name: "unknown top level command suggests close match",
			args: []string{"aply"},
			wantErr: []string{
				`Error: unknown command "aply" for "kongctl"`,
				"Did you mean this command?",
				"  apply",
				`Run 'kongctl --help' for usage`,
			},
			wantExit:     1,
			forbidErr:    []string{"Usage:"},
			expectStderr: true,
		},
		{
			name: "unknown top level command before unsupported shorthand suggests command",
			args: []string{"synch", "-f", "config.yaml"},
			wantErr: []string{
				`Error: unknown command "synch" for "kongctl"`,
				"Did you mean this command?",
				"  sync",
				`Run 'kongctl --help' for usage`,
			},
			wantExit: 1,
			forbidErr: []string{
				"Usage:",
				"unknown shorthand flag",
				"Did you mean one of these flags?",
			},
			expectStderr: true,
		},
		{
			name: "unknown top level command typo before unsupported shorthand suggests command",
			args: []string{"syk", "-f", "config.yaml"},
			wantErr: []string{
				`Error: unknown command "syk" for "kongctl"`,
				"Did you mean this command?",
				"  sync",
				`Run 'kongctl --help' for usage`,
			},
			wantExit: 1,
			forbidErr: []string{
				"Usage:",
				"unknown shorthand flag",
				"Did you mean one of these flags?",
			},
			expectStderr: true,
		},
		{
			name: "unknown root flag before known command stays flag error",
			args: []string{"--definitely-not-a-real-kongctl-flag", "version"},
			wantErr: []string{
				`Error: unknown flag: --definitely-not-a-real-kongctl-flag`,
				`Run 'kongctl --help' for usage`,
			},
			wantExit: 1,
			forbidErr: []string{
				"Usage:",
				`unknown command "version"`,
			},
			expectStderr: true,
		},
		{
			name: "unknown nested command suggests close match",
			args: []string{"get", "gatewy"},
			wantErr: []string{
				`Error: unknown command "gatewy" for "kongctl get"`,
				"Did you mean this command?",
				"  gateway",
				`Run 'kongctl get --help' for usage`,
			},
			wantExit:     1,
			forbidErr:    []string{"Usage:"},
			expectStderr: true,
		},
		{
			name: "unknown flag suggests close match",
			args: []string{"version", "--log-leve", "error"},
			wantErr: []string{
				`Error: unknown flag: --log-leve`,
				"Did you mean this flag?",
				"  --log-level",
				`Run 'kongctl version --help' for usage`,
			},
			wantExit:     1,
			forbidErr:    []string{"Usage:"},
			expectStderr: true,
		},
		{
			name: "format flag suggests output when output is valid",
			args: []string{"version", "--format", "yaml"},
			wantErr: []string{
				`Error: unknown flag: --format`,
				"Did you mean this flag?",
				"--output, -o",
				"Configures the format of data written to STDOUT.",
				`Run 'kongctl version --help' for usage`,
			},
			wantExit:     1,
			forbidErr:    []string{"Usage:"},
			expectStderr: true,
		},
		{
			name: "format flag does not suggest output when output is unsupported",
			args: []string{"scaffold", "--format", "yaml", "api"},
			wantErr: []string{
				`Error: unknown flag: --format`,
				`Run 'kongctl scaffold --help' for usage`,
			},
			wantExit: 1,
			forbidErr: []string{
				"Usage:",
				"Did you mean this flag?",
				"--output",
			},
			expectStderr: true,
		},
		{
			name: "unknown shorthand flag suggestions include descriptions",
			args: []string{"diff", "-g", "config.yaml"},
			wantErr: []string{
				`Error: unknown shorthand flag: 'g' in -g`,
				"Did you mean one of these flags?",
				"-f, --filename",
				"File, directory, URL, or '-' to use to create the resource",
				"-R, --recursive",
				"Process the directory used in -f, --filename recursively",
				`Run 'kongctl diff --help' for usage`,
			},
			wantExit:     1,
			forbidErr:    []string{"Usage:"},
			expectStderr: true,
		},
		{
			name: "argument validation uses concise help hint",
			args: []string{"scaffold"},
			wantErr: []string{
				`Error: accepts 1 arg(s), received 0`,
				`Run 'kongctl scaffold --help' for usage`,
			},
			wantExit:     1,
			forbidErr:    []string{"Usage:"},
			expectStderr: true,
		},
		{
			name: "custom flag error remains actionable",
			args: []string{"plan", "-o", "plan.json"},
			wantErr: []string{
				`Error: flags -o/--output are not supported for the plan command; use --output-file to save the plan to a file`,
				`Run 'kongctl plan --help' for usage`,
			},
			wantExit:     1,
			forbidErr:    []string{"Usage:"},
			expectStderr: true,
		},
		{
			name: "bare declarative plan requires filename with concise help hint",
			args: []string{"plan"},
			wantErr: []string{
				`Error: no configuration sources specified; use -f to specify files, directories, or URLs`,
				"Error: no configuration sources specified; use -f to specify files, directories, or URLs\n\n" +
					`Run 'kongctl plan --help' for usage`,
			},
			wantExit:     1,
			forbidErr:    []string{"Usage:"},
			expectStderr: true,
		},
		{
			name: "explicit help still renders full help",
			args: []string{"get", "--help"},
			wantOut: []string{
				"Usage:",
				"kongctl get [command]",
			},
			wantExit:     0,
			forbidErr:    []string{"Error:"},
			expectStdout: true,
		},
		{
			name: "explicit root help still renders flags",
			args: []string{"--help"},
			wantOut: []string{
				`kongctl is the official command line tool for the Kong Konnect API platform.`,
				"Find more information at:",
				"Usage:",
				"Flags:",
			},
			wantExit:     0,
			forbidErr:    []string{"Error:"},
			expectStdout: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executeRootForTest(t, tt.args...)
			if result.exitCode != tt.wantExit {
				t.Fatalf("expected exit code %d, got %d\nstdout:\n%s\nstderr:\n%s",
					tt.wantExit, result.exitCode, result.stdout, result.stderr)
			}
			if tt.expectStderr && strings.TrimSpace(result.stderr) == "" {
				t.Fatalf("expected stderr output")
			}
			if tt.expectStdout && strings.TrimSpace(result.stdout) == "" {
				t.Fatalf("expected stdout output")
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(result.stderr, want) {
					t.Fatalf("expected stderr to contain %q\nstderr:\n%s", want, result.stderr)
				}
			}
			for _, want := range tt.wantOut {
				if !strings.Contains(result.stdout, want) {
					t.Fatalf("expected stdout to contain %q\nstdout:\n%s", want, result.stdout)
				}
			}
			for _, forbidden := range tt.forbidErr {
				if strings.Contains(result.stderr, forbidden) {
					t.Fatalf("expected stderr not to contain %q\nstderr:\n%s", forbidden, result.stderr)
				}
			}
			for _, forbidden := range tt.forbidOut {
				if strings.Contains(result.stdout, forbidden) {
					t.Fatalf("expected stdout not to contain %q\nstdout:\n%s", forbidden, result.stdout)
				}
			}
		})
	}
}

func TestKonnectEnvFlagIsHiddenFromHelp(t *testing.T) {
	result := executeRootForTest(t, "--help")
	if result.exitCode != 0 {
		t.Fatalf("expected help to succeed\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if strings.Contains(result.stdout, "--"+konnectcommon.KonnectEnvFlagName) {
		t.Fatalf("expected hidden konnect env flag not to appear in help\nstdout:\n%s", result.stdout)
	}
}

func TestKonnectEnvFlagAppliesDuringRootExecution(t *testing.T) {
	result := executeRootForTest(t, "--"+konnectcommon.KonnectEnvFlagName, konnectcommon.EnvironmentTech)
	if result.exitCode != 0 {
		t.Fatalf("expected command to succeed\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if currConfig == nil {
		t.Fatal("expected config to be initialized")
	}
	if got := currConfig.GetString(konnectcommon.BaseURLConfigPath); got != konnectcommon.TechBaseURLDefault {
		t.Fatalf("base URL = %q, want %q", got, konnectcommon.TechBaseURLDefault)
	}
	if got := currConfig.GetString(konnectcommon.EnvironmentConfigPath); got != konnectcommon.EnvironmentTech {
		t.Fatalf("environment = %q, want %q", got, konnectcommon.EnvironmentTech)
	}
	if got := currConfig.GetString(konnectcommon.AuthBaseURLConfigPath); got != konnectcommon.TechGlobalBaseURL {
		t.Fatalf("auth base URL = %q, want %q", got, konnectcommon.TechGlobalBaseURL)
	}
	if got := currConfig.GetString(konnectcommon.MachineClientIDConfigPath); got != konnectcommon.TechMachineClientID {
		t.Fatalf("machine client ID = %q, want %q", got, konnectcommon.TechMachineClientID)
	}
}

func TestKonnectEnvironmentProfileEnvAppliesDuringRootExecution(t *testing.T) {
	t.Setenv("KONGCTL_DEFAULT_KONNECT_ENVIRONMENT", konnectcommon.EnvironmentTech)

	result := executeRootForTest(t)
	if result.exitCode != 0 {
		t.Fatalf("expected command to succeed\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if currConfig == nil {
		t.Fatal("expected config to be initialized")
	}
	if got := currConfig.GetString(konnectcommon.BaseURLConfigPath); got != konnectcommon.TechBaseURLDefault {
		t.Fatalf("base URL = %q, want %q", got, konnectcommon.TechBaseURLDefault)
	}
	if got := currConfig.GetString(konnectcommon.EnvironmentConfigPath); got != konnectcommon.EnvironmentTech {
		t.Fatalf("environment = %q, want %q", got, konnectcommon.EnvironmentTech)
	}
}

func TestKonnectEnvFlagAppliesToKonnectFirstCommand(t *testing.T) {
	var capturedBaseURL string
	factoryCalls := 0
	original := helpers.DefaultSDKFactory
	t.Cleanup(func() {
		helpers.DefaultSDKFactory = original
	})
	helpers.DefaultSDKFactory = func(cfg configpkg.Hook, _ *slog.Logger) (helpers.SDKAPI, error) {
		factoryCalls++
		capturedBaseURL = cfg.GetString(konnectcommon.BaseURLConfigPath)
		orgID := "org-1"
		return &helpers.MockKonnectSDK{
			MeFactory: func() helpers.MeAPI {
				return rootTestMeAPI{
					organization: &kkComps.MeOrganization{
						ID: &orgID,
					},
				}
			},
		}, nil
	}

	result := executeRootForTest(
		t,
		"get", "org",
		"--"+konnectcommon.KonnectEnvFlagName, konnectcommon.EnvironmentTech,
		"--output", "json",
	)
	if result.exitCode != 0 {
		t.Fatalf("expected command to succeed\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if factoryCalls == 0 {
		t.Fatalf("expected SDK factory to be called\nstdout:\n%s\nstderr:\n%s", result.stdout, result.stderr)
	}
	if capturedBaseURL != konnectcommon.TechBaseURLDefault {
		t.Fatalf("base URL at SDK factory = %q, want %q", capturedBaseURL, konnectcommon.TechBaseURLDefault)
	}
}

func newKonnectEnvCommandForTest() *cobra.Command {
	command := &cobra.Command{Use: "root"}
	command.Flags().String(konnectcommon.KonnectEnvFlagName, "", "")
	return command
}

func TestApplyKonnectEnvironmentDefaultsNoSelectionDoesNotOverrideProfileConfig(t *testing.T) {
	cfg := configpkg.BuildProfiledConfig("default", "", viper.New())
	cfg.SetString(konnectcommon.BaseURLConfigPath, "https://custom.example.test")
	cfg.SetString(konnectcommon.AuthBaseURLConfigPath, "https://auth.example.test")
	cfg.SetString(konnectcommon.MachineClientIDConfigPath, "client-id")
	command := newKonnectEnvCommandForTest()

	requireNoError(t, konnectcommon.ApplyEnvironmentDefaults(command, cfg))

	if got := cfg.GetString(konnectcommon.BaseURLConfigPath); got != "https://custom.example.test" {
		t.Fatalf("base URL = %q, want profile config to remain unchanged", got)
	}
	if got := cfg.GetString(konnectcommon.AuthBaseURLConfigPath); got != "https://auth.example.test" {
		t.Fatalf("auth base URL = %q, want profile config to remain unchanged", got)
	}
	if got := cfg.GetString(konnectcommon.MachineClientIDConfigPath); got != "client-id" {
		t.Fatalf("machine client ID = %q, want profile config to remain unchanged", got)
	}
}

func TestApplyKonnectEnvironmentDefaultsFromFlagTech(t *testing.T) {
	cfg := configpkg.BuildProfiledConfig("default", "", viper.New())
	command := newKonnectEnvCommandForTest()
	requireNoError(t, command.Flags().Set(konnectcommon.KonnectEnvFlagName, konnectcommon.EnvironmentTech))

	requireNoError(t, konnectcommon.ApplyEnvironmentDefaults(command, cfg))

	if got := cfg.GetString(konnectcommon.BaseURLConfigPath); got != konnectcommon.TechBaseURLDefault {
		t.Fatalf("base URL = %q, want %q", got, konnectcommon.TechBaseURLDefault)
	}
	if got := cfg.GetString(konnectcommon.AuthBaseURLConfigPath); got != konnectcommon.TechGlobalBaseURL {
		t.Fatalf("auth base URL = %q, want %q", got, konnectcommon.TechGlobalBaseURL)
	}
	if got := cfg.GetString(konnectcommon.MachineClientIDConfigPath); got != konnectcommon.TechMachineClientID {
		t.Fatalf("machine client ID = %q, want %q", got, konnectcommon.TechMachineClientID)
	}
	if got := cfg.GetProfile(); got != "default" {
		t.Fatalf("profile = %q, want default", got)
	}
}

func TestApplyKonnectEnvironmentDefaultsFromProfileConfigTech(t *testing.T) {
	cfg := configpkg.BuildProfiledConfig("default", "", viper.New())
	cfg.SetString(konnectcommon.EnvironmentConfigPath, konnectcommon.EnvironmentTech)
	command := newKonnectEnvCommandForTest()

	requireNoError(t, konnectcommon.ApplyEnvironmentDefaults(command, cfg))

	if got := cfg.GetString(konnectcommon.BaseURLConfigPath); got != konnectcommon.TechBaseURLDefault {
		t.Fatalf("base URL = %q, want %q", got, konnectcommon.TechBaseURLDefault)
	}
	if got := cfg.GetString(konnectcommon.AuthBaseURLConfigPath); got != konnectcommon.TechGlobalBaseURL {
		t.Fatalf("auth base URL = %q, want %q", got, konnectcommon.TechGlobalBaseURL)
	}
	if got := cfg.GetString(konnectcommon.MachineClientIDConfigPath); got != konnectcommon.TechMachineClientID {
		t.Fatalf("machine client ID = %q, want %q", got, konnectcommon.TechMachineClientID)
	}
}

func TestApplyKonnectEnvironmentDefaultsFlagOverridesProfileConfig(t *testing.T) {
	cfg := configpkg.BuildProfiledConfig("default", "", viper.New())
	cfg.SetString(konnectcommon.EnvironmentConfigPath, konnectcommon.EnvironmentTech)
	cfg.SetString(konnectcommon.BaseURLConfigPath, konnectcommon.TechBaseURLDefault)
	cfg.SetString(konnectcommon.AuthBaseURLConfigPath, konnectcommon.TechGlobalBaseURL)
	cfg.SetString(konnectcommon.MachineClientIDConfigPath, konnectcommon.TechMachineClientID)
	command := newKonnectEnvCommandForTest()
	requireNoError(t, command.Flags().Set(konnectcommon.KonnectEnvFlagName, konnectcommon.EnvironmentCom))

	requireNoError(t, konnectcommon.ApplyEnvironmentDefaults(command, cfg))

	if got := cfg.GetString(konnectcommon.BaseURLConfigPath); got != konnectcommon.BaseURLDefault {
		t.Fatalf("base URL = %q, want %q", got, konnectcommon.BaseURLDefault)
	}
	if got := cfg.GetString(konnectcommon.AuthBaseURLConfigPath); got != konnectcommon.AuthBaseURLDefault {
		t.Fatalf("auth base URL = %q, want %q", got, konnectcommon.AuthBaseURLDefault)
	}
	if got := cfg.GetString(konnectcommon.MachineClientIDConfigPath); got != konnectcommon.MachineClientIDDefault {
		t.Fatalf("machine client ID = %q, want %q", got, konnectcommon.MachineClientIDDefault)
	}
}

func TestApplyKonnectEnvironmentDefaultsRespectsExplicitEndpointFlags(t *testing.T) {
	cfg := configpkg.BuildProfiledConfig("default", "", viper.New())
	cfg.SetString(konnectcommon.EnvironmentConfigPath, konnectcommon.EnvironmentTech)
	command := newKonnectEnvCommandForTest()
	command.Flags().String(konnectcommon.BaseURLFlagName, "", "")
	command.Flags().String(konnectcommon.AuthBaseURLFlagName, "", "")
	command.Flags().String(konnectcommon.MachineClientIDFlagName, "", "")
	requireNoError(t, command.Flags().Set(konnectcommon.BaseURLFlagName, "https://custom.example.test"))
	requireNoError(t, command.Flags().Set(konnectcommon.AuthBaseURLFlagName, "https://auth.example.test"))
	requireNoError(t, command.Flags().Set(konnectcommon.MachineClientIDFlagName, "client-id"))

	requireNoError(t, konnectcommon.ApplyEnvironmentDefaults(command, cfg))

	if got := cfg.GetString(konnectcommon.BaseURLConfigPath); got != "" {
		t.Fatalf("base URL = %q, want explicit flag to remain unset in config override", got)
	}
	if got := cfg.GetString(konnectcommon.AuthBaseURLConfigPath); got != "" {
		t.Fatalf("auth base URL = %q, want explicit flag to remain unset in config override", got)
	}
	if got := cfg.GetString(konnectcommon.MachineClientIDConfigPath); got != "" {
		t.Fatalf("machine client ID = %q, want explicit flag to remain unset in config override", got)
	}
}

func TestApplyKonnectEnvironmentDefaultsRespectsExplicitEndpointConfig(t *testing.T) {
	cfg := configpkg.BuildProfiledConfig("default", "", viper.New())
	cfg.SetString(konnectcommon.EnvironmentConfigPath, konnectcommon.EnvironmentTech)
	cfg.SetString(konnectcommon.BaseURLConfigPath, "https://regional.example.test")
	cfg.SetString(konnectcommon.AuthBaseURLConfigPath, "https://global.example.test")
	cfg.SetString(konnectcommon.MachineClientIDConfigPath, "client-id")
	command := newKonnectEnvCommandForTest()

	requireNoError(t, konnectcommon.ApplyEnvironmentDefaults(command, cfg))

	if got := cfg.GetString(konnectcommon.BaseURLConfigPath); got != "https://regional.example.test" {
		t.Fatalf("base URL = %q, want explicit config to remain unchanged", got)
	}
	if got := cfg.GetString(konnectcommon.AuthBaseURLConfigPath); got != "https://global.example.test" {
		t.Fatalf("auth base URL = %q, want explicit config to remain unchanged", got)
	}
	if got := cfg.GetString(konnectcommon.MachineClientIDConfigPath); got != "client-id" {
		t.Fatalf("machine client ID = %q, want explicit config to remain unchanged", got)
	}
}

func TestApplyKonnectEnvironmentDefaultsSurvivesLaterFlagBinding(t *testing.T) {
	cfg := configpkg.BuildProfiledConfig("default", "", viper.New())
	cfg.SetString(konnectcommon.RegionConfigPath, "global")
	cfg.SetString(konnectcommon.EnvironmentConfigPath, konnectcommon.EnvironmentTech)
	command := newKonnectEnvCommandForTest()
	command.Flags().String(konnectcommon.BaseURLFlagName, "", "")
	command.Flags().String(konnectcommon.AuthBaseURLFlagName, "", "")
	command.Flags().String(konnectcommon.MachineClientIDFlagName, "", "")

	requireNoError(t, konnectcommon.ApplyEnvironmentDefaults(command, cfg))
	requireNoError(t, cfg.BindFlag(konnectcommon.BaseURLConfigPath, command.Flags().Lookup(konnectcommon.BaseURLFlagName)))
	requireNoError(t, cfg.BindFlag(
		konnectcommon.AuthBaseURLConfigPath,
		command.Flags().Lookup(konnectcommon.AuthBaseURLFlagName),
	))
	requireNoError(t, cfg.BindFlag(
		konnectcommon.MachineClientIDConfigPath,
		command.Flags().Lookup(konnectcommon.MachineClientIDFlagName),
	))

	if got := cfg.GetString(konnectcommon.BaseURLConfigPath); got != konnectcommon.TechGlobalBaseURL {
		t.Fatalf("base URL = %q, want %q", got, konnectcommon.TechGlobalBaseURL)
	}
	if got := cfg.GetString(konnectcommon.AuthBaseURLConfigPath); got != konnectcommon.TechGlobalBaseURL {
		t.Fatalf("auth base URL = %q, want %q", got, konnectcommon.TechGlobalBaseURL)
	}
	if got := cfg.GetString(konnectcommon.MachineClientIDConfigPath); got != konnectcommon.TechMachineClientID {
		t.Fatalf("machine client ID = %q, want %q", got, konnectcommon.TechMachineClientID)
	}
}

func TestApplyKonnectEnvironmentDefaultsUsesExplicitRegion(t *testing.T) {
	cfg := configpkg.BuildProfiledConfig("default", "", viper.New())
	cfg.SetString(konnectcommon.EnvironmentConfigPath, konnectcommon.EnvironmentTech)
	command := newKonnectEnvCommandForTest()
	command.Flags().String(konnectcommon.RegionFlagName, "", "")
	requireNoError(t, command.Flags().Set(konnectcommon.RegionFlagName, "eu"))

	requireNoError(t, konnectcommon.ApplyEnvironmentDefaults(command, cfg))

	if got := cfg.GetString(konnectcommon.BaseURLConfigPath); got != "https://eu.api.konghq.tech" {
		t.Fatalf("base URL = %q, want https://eu.api.konghq.tech", got)
	}
}

func TestApplyKonnectEnvironmentDefaultsUsesConfiguredRegion(t *testing.T) {
	cfg := configpkg.BuildProfiledConfig("default", "", viper.New())
	cfg.SetString(konnectcommon.EnvironmentConfigPath, konnectcommon.EnvironmentTech)
	cfg.SetString(konnectcommon.RegionConfigPath, "eu")
	command := newKonnectEnvCommandForTest()

	requireNoError(t, konnectcommon.ApplyEnvironmentDefaults(command, cfg))

	if got := cfg.GetString(konnectcommon.BaseURLConfigPath); got != "https://eu.api.konghq.tech" {
		t.Fatalf("base URL = %q, want https://eu.api.konghq.tech", got)
	}
}

func TestApplyKonnectEnvironmentDefaultsRejectsInvalidEnv(t *testing.T) {
	cfg := configpkg.BuildProfiledConfig("default", "", viper.New())
	cfg.SetString(konnectcommon.EnvironmentConfigPath, "stage")
	command := newKonnectEnvCommandForTest()

	err := konnectcommon.ApplyEnvironmentDefaults(command, cfg)
	if err == nil {
		t.Fatal("expected invalid konnect environment error")
	}
	if !strings.Contains(err.Error(), "unsupported konnect environment") {
		t.Fatalf("expected unsupported environment error, got %v", err)
	}
	if !strings.Contains(err.Error(), "allowed: production, tech; aliases: com, prod") {
		t.Fatalf("expected allowed values in error, got %v", err)
	}
}

func TestApplyKonnectEnvironmentDefaultsAcceptsProductionEnvironment(t *testing.T) {
	cfg := configpkg.BuildProfiledConfig("default", "", viper.New())
	cfg.SetString(konnectcommon.EnvironmentConfigPath, konnectcommon.EnvironmentProduction)
	command := newKonnectEnvCommandForTest()

	requireNoError(t, konnectcommon.ApplyEnvironmentDefaults(command, cfg))

	if got := cfg.GetString(konnectcommon.BaseURLConfigPath); got != konnectcommon.BaseURLDefault {
		t.Fatalf("base URL = %q, want %q", got, konnectcommon.BaseURLDefault)
	}
	if got := cfg.GetString(konnectcommon.AuthBaseURLConfigPath); got != konnectcommon.AuthBaseURLDefault {
		t.Fatalf("auth base URL = %q, want %q", got, konnectcommon.AuthBaseURLDefault)
	}
}

func TestPlainCommandErrorDoesNotShowUsageHint(t *testing.T) {
	var stderr bytes.Buffer
	command := &cobra.Command{Use: "runtime"}

	renderCommandError(&stderr, command, errors.New("runtime operation failed"))

	output := stderr.String()
	if !strings.Contains(output, "Error: runtime operation failed") {
		t.Fatalf("expected plain error output, got:\n%s", output)
	}
	if strings.Contains(output, "Run '") {
		t.Fatalf("expected no usage hint for plain runtime error, got:\n%s", output)
	}
	if strings.Contains(output, "Usage:") {
		t.Fatalf("expected no usage text for plain runtime error, got:\n%s", output)
	}
}

func TestUnknownFlagErrorUXCoversCommandTree(t *testing.T) {
	paths := collectCommandPathsForTest(t)
	for _, path := range paths {
		t.Run(commandPathForTest(path), func(t *testing.T) {
			args := append([]string{}, path...)
			args = append(args, "--definitely-not-a-real-kongctl-flag")

			result := executeRootForTest(t, args...)
			if result.exitCode != 1 {
				t.Fatalf("expected exit code 1, got %d\nstdout:\n%s\nstderr:\n%s",
					result.exitCode, result.stdout, result.stderr)
			}
			assertConciseErrorUX(t, result.stderr, commandPathForTest(path))
			if !strings.Contains(result.stderr, "Error: unknown flag: --definitely-not-a-real-kongctl-flag") {
				t.Fatalf("expected unknown flag error\nstderr:\n%s", result.stderr)
			}
		})
	}
}

func TestRequiresSubcommandErrorUXCoversCommandGroups(t *testing.T) {
	commands := collectRequiresSubcommandCommandsForTest(t)
	for _, item := range commands {
		t.Run(commandPathForTest(item.path), func(t *testing.T) {
			result := executeRootForTest(t, item.path...)
			if result.exitCode != 1 {
				t.Fatalf("expected exit code 1, got %d\nstdout:\n%s\nstderr:\n%s",
					result.exitCode, result.stdout, result.stderr)
			}
			assertConciseErrorUX(t, result.stderr, commandPathForTest(item.path))
			if !strings.Contains(result.stderr, "requires a subcommand") {
				t.Fatalf("expected missing subcommand error\nstderr:\n%s", result.stderr)
			}
			assertAvailableSubcommands(t, result.stderr, item.command)
		})
	}
}

func TestUnknownSubcommandErrorUXCoversCommandGroups(t *testing.T) {
	commands := collectRequiresSubcommandCommandsForTest(t)
	for _, item := range commands {
		t.Run(commandPathForTest(item.path), func(t *testing.T) {
			child := firstAvailableChildName(item.command)
			if child == "" {
				t.Skip("command has no available children")
			}
			args := append([]string{}, item.path...)
			args = append(args, typoForTest(child))

			result := executeRootForTest(t, args...)
			if result.exitCode != 1 {
				t.Fatalf("expected exit code 1, got %d\nstdout:\n%s\nstderr:\n%s",
					result.exitCode, result.stdout, result.stderr)
			}
			assertConciseErrorUX(t, result.stderr, commandPathForTest(item.path))
			if !strings.Contains(result.stderr, "unknown command") {
				t.Fatalf("expected unknown command error\nstderr:\n%s", result.stderr)
			}
		})
	}
}

type rootCommandResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func executeRootForTest(t *testing.T, args ...string) rootCommandResult {
	t.Helper()

	oldRootCmd := rootCmd
	oldDefaultConfigFilePath := defaultConfigFilePath
	oldConfigFilePath := configFilePath
	oldCurrProfile := currProfile
	oldCurrConfig := currConfig
	oldStreams := streams
	oldLogger := logger
	oldBuildInfo := buildInfo
	oldOutputFormat := outputFormat
	oldLogLevel := logLevel
	oldLogFile := logFile
	oldKonnectEnv := konnectEnv
	oldEnableTraverseRunHooks := cobra.EnableTraverseRunHooks
	oldArgs := os.Args
	t.Cleanup(func() {
		rootCmd = oldRootCmd
		defaultConfigFilePath = oldDefaultConfigFilePath
		configFilePath = oldConfigFilePath
		currProfile = oldCurrProfile
		currConfig = oldCurrConfig
		streams = oldStreams
		logger = oldLogger
		buildInfo = oldBuildInfo
		outputFormat = oldOutputFormat
		logLevel = oldLogLevel
		konnectEnv = oldKonnectEnv
		if logFile != nil && logFile != oldLogFile {
			_ = logFile.Close()
		}
		logFile = oldLogFile
		cobra.EnableTraverseRunHooks = oldEnableTraverseRunHooks
		os.Args = oldArgs
	})

	cobra.EnableTraverseRunHooks = true
	configHome := filepath.Join(t.TempDir(), "config")
	t.Setenv("XDG_CONFIG_HOME", configHome)

	var err error
	defaultConfigFilePath, err = configpkg.GetDefaultConfigFilePath()
	requireNoError(t, err)
	configFilePath = ""
	currProfile = profile.DefaultProfile
	currConfig = nil
	konnectEnv = ""
	buildInfo = nil
	outputFormat = cmdpkg.NewDeferredEnum([]string{
		common.JSON.String(),
		common.YAML.String(),
		common.TEXT.String(),
	}, common.TEXT.String())
	logLevel = cmdpkg.NewEnum([]string{
		common.TRACE.String(),
		common.DEBUG.String(),
		common.INFO.String(),
		common.WARN.String(),
		common.ERROR.String(),
	}, common.ERROR.String())

	var stdout, stderr bytes.Buffer
	streams = &iostreams.IOStreams{
		In:     &bytes.Buffer{},
		Out:    &stdout,
		ErrOut: &stderr,
	}
	logger = slog.New(log.NewFriendlyErrorHandler(&stderr))

	os.Args = append([]string{"kongctl"}, args...)
	rootCmd = newRootCmd()
	requireNoError(t, addCommands())
	rootCmd.SetArgs(args)
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)

	executed, err := rootCmd.ExecuteContextC(context.Background())
	exitCode := 0
	if err != nil {
		renderCommandError(&stderr, executed, err)
		exitCode = 1
	}
	closeLogFile()

	return rootCommandResult{
		stdout:   stdout.String(),
		stderr:   stderr.String(),
		exitCode: exitCode,
	}
}

func collectCommandPathsForTest(t *testing.T) [][]string {
	t.Helper()
	root := newRootCmd()
	requireNoError(t, addCommandsWithRootForTest(root))

	paths := [][]string{{}}
	walkCommandsForTest(root, nil, func(command *cobra.Command, path []string) {
		if command == root || command.Hidden || command.DisableFlagParsing {
			return
		}
		paths = append(paths, append([]string{}, path...))
	})
	return paths
}

type commandPathItem struct {
	command *cobra.Command
	path    []string
}

func collectRequiresSubcommandCommandsForTest(t *testing.T) []commandPathItem {
	t.Helper()
	root := newRootCmd()
	requireNoError(t, addCommandsWithRootForTest(root))

	items := []commandPathItem{}
	walkCommandsForTest(root, nil, func(command *cobra.Command, path []string) {
		if command.Hidden || !cmdpkg.CommandRequiresSubcommand(command) {
			return
		}
		items = append(items, commandPathItem{
			command: command,
			path:    append([]string{}, path...),
		})
	})
	return items
}

func addCommandsWithRootForTest(command *cobra.Command) error {
	oldRootCmd := rootCmd
	rootCmd = command
	defer func() {
		rootCmd = oldRootCmd
	}()
	return addCommands()
}

func installRootTokenSDK(t *testing.T, sdk helpers.SDKAPI) {
	t.Helper()

	original := helpers.DefaultSDKFactory
	t.Cleanup(func() {
		helpers.DefaultSDKFactory = original
	})
	helpers.DefaultSDKFactory = func(configpkg.Hook, *slog.Logger) (helpers.SDKAPI, error) {
		return sdk, nil
	}
}

type rootTestMeAPI struct {
	userID       string
	organization *kkComps.MeOrganization
}

func (m rootTestMeAPI) GetUsersMe(context.Context, ...kkOps.Option) (*kkOps.GetUsersMeResponse, error) {
	return &kkOps.GetUsersMeResponse{
		User: &kkComps.User{ID: &m.userID},
	}, nil
}

func (m rootTestMeAPI) GetOrganizationsMe(
	context.Context,
	...kkOps.Option,
) (*kkOps.GetOrganizationsMeResponse, error) {
	return &kkOps.GetOrganizationsMeResponse{
		MeOrganization: m.organization,
	}, nil
}

func rootTestPAT(id, name string) kkComps.PersonalAccessToken {
	createdAt := time.Date(2026, time.May, 25, 12, 0, 0, 0, time.UTC)
	expiresAt := createdAt.Add(30 * 24 * time.Hour)
	return kkComps.PersonalAccessToken{
		ID:        id,
		UserID:    "user-1",
		Name:      name,
		State:     kkComps.PersonalAccessTokenStateActive,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
		ExpiresAt: &expiresAt,
	}
}

func rootTestSPAT(id, name string) kkComps.SystemAccountAccessToken {
	createdAt := time.Date(2026, time.May, 25, 12, 0, 0, 0, time.UTC)
	expiresAt := createdAt.Add(30 * 24 * time.Hour)
	return kkComps.SystemAccountAccessToken{
		ID:        &id,
		Name:      &name,
		CreatedAt: &createdAt,
		UpdatedAt: &createdAt,
		ExpiresAt: &expiresAt,
	}
}

type rootTestPATAPI struct {
	token          string
	id             string
	tokens         []kkComps.PersonalAccessToken
	createdUserID  string
	createdRequest *kkComps.PersonalAccessTokenCreateRequest
	gotTokenID     string
	deletedTokenID string
}

func (a *rootTestPATAPI) ListUsersPersonalAccessTokens(
	context.Context,
	string,
	...kkOps.Option,
) (*kkOps.ListUsersPersonalAccessTokensResponse, error) {
	return &kkOps.ListUsersPersonalAccessTokensResponse{
		PersonalAccessTokenListResponse: &kkComps.PersonalAccessTokenListResponse{
			Data: a.tokens,
		},
	}, nil
}

func (a *rootTestPATAPI) CreatePersonalAccessToken(
	_ context.Context,
	userID string,
	request *kkComps.PersonalAccessTokenCreateRequest,
	_ ...kkOps.Option,
) (*kkOps.CreatePersonalAccessTokenResponse, error) {
	a.createdUserID = userID
	a.createdRequest = request
	createdAt := time.Date(2026, time.May, 25, 12, 0, 0, 0, time.UTC)
	return &kkOps.CreatePersonalAccessTokenResponse{
		PersonalAccessTokenCreateResponse: &kkComps.PersonalAccessTokenCreateResponse{
			ID:           a.id,
			UserID:       userID,
			Name:         "ci",
			State:        kkComps.PersonalAccessTokenCreateResponseStateActive,
			KonnectToken: a.token,
			CreatedAt:    createdAt,
		},
	}, nil
}

func (a *rootTestPATAPI) GetPersonalAccessTokenDetails(
	_ context.Context,
	_ string,
	tokenID string,
	_ ...kkOps.Option,
) (*kkOps.GetPersonalAccessTokenDetailsResponse, error) {
	a.gotTokenID = tokenID
	for i := range a.tokens {
		if a.tokens[i].ID == tokenID {
			return &kkOps.GetPersonalAccessTokenDetailsResponse{
				PersonalAccessToken: &a.tokens[i],
			}, nil
		}
	}
	return &kkOps.GetPersonalAccessTokenDetailsResponse{
		PersonalAccessToken: &kkComps.PersonalAccessToken{
			ID:   tokenID,
			Name: "ci",
		},
	}, nil
}

func (a *rootTestPATAPI) DeletePersonalAccessToken(
	_ context.Context,
	_ string,
	tokenID string,
	_ ...kkOps.Option,
) (*kkOps.DeletePersonalAccessTokenResponse, error) {
	a.deletedTokenID = tokenID
	return &kkOps.DeletePersonalAccessTokenResponse{}, nil
}

type rootTestSystemAccountAPI struct {
	id   string
	name string
}

func (a rootTestSystemAccountAPI) ListSystemAccounts(
	context.Context,
	kkOps.GetSystemAccountsRequest,
) (*kkOps.GetSystemAccountsResponse, error) {
	return &kkOps.GetSystemAccountsResponse{
		SystemAccountCollection: &kkComps.SystemAccountCollection{
			Meta: &kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: 1}},
			Data: []kkComps.SystemAccount{
				{ID: &a.id, Name: &a.name},
			},
		},
	}, nil
}

func (a rootTestSystemAccountAPI) GetSystemAccount(
	context.Context,
	string,
) (*kkOps.GetSystemAccountsIDResponse, error) {
	return &kkOps.GetSystemAccountsIDResponse{}, nil
}

type rootTestSPATAPI struct {
	token            string
	tokens           []kkComps.SystemAccountAccessToken
	gotTokenID       string
	deletedAccountID string
	deletedTokenID   string
}

func (a rootTestSPATAPI) GetSystemAccountIDAccessTokens(
	_ context.Context,
	request kkOps.GetSystemAccountIDAccessTokensRequest,
	_ ...kkOps.Option,
) (*kkOps.GetSystemAccountIDAccessTokensResponse, error) {
	tokens := a.tokens
	if request.Filter != nil && request.Filter.Name != nil && request.Filter.Name.Eq != nil {
		name := *request.Filter.Name.Eq
		tokens = []kkComps.SystemAccountAccessToken{}
		for i := range a.tokens {
			if a.tokens[i].Name != nil && *a.tokens[i].Name == name {
				tokens = append(tokens, a.tokens[i])
			}
		}
	}
	return &kkOps.GetSystemAccountIDAccessTokensResponse{
		SystemAccountAccessTokenCollection: &kkComps.SystemAccountAccessTokenCollection{
			Meta: &kkComps.PaginatedMeta{Page: kkComps.PageMeta{Total: float64(len(tokens))}},
			Data: tokens,
		},
	}, nil
}

func (a rootTestSPATAPI) PostSystemAccountsIDAccessTokens(
	context.Context,
	string,
	*kkComps.CreateSystemAccountAccessToken,
	...kkOps.Option,
) (*kkOps.PostSystemAccountsIDAccessTokensResponse, error) {
	id := "spat-id"
	name := "ci"
	return &kkOps.PostSystemAccountsIDAccessTokensResponse{
		SystemAccountAccessTokenCreated: &kkComps.SystemAccountAccessTokenCreated{
			ID:    &id,
			Name:  &name,
			Token: &a.token,
		},
	}, nil
}

func (a *rootTestSPATAPI) GetSystemAccountsIDAccessTokensID(
	_ context.Context,
	_ string,
	tokenID string,
	_ ...kkOps.Option,
) (*kkOps.GetSystemAccountsIDAccessTokensIDResponse, error) {
	a.gotTokenID = tokenID
	for i := range a.tokens {
		if a.tokens[i].ID != nil && *a.tokens[i].ID == tokenID {
			return &kkOps.GetSystemAccountsIDAccessTokensIDResponse{
				SystemAccountAccessToken: &a.tokens[i],
			}, nil
		}
	}
	return &kkOps.GetSystemAccountsIDAccessTokensIDResponse{
		SystemAccountAccessToken: &kkComps.SystemAccountAccessToken{
			ID:   &tokenID,
			Name: &tokenID,
		},
	}, nil
}

func (a *rootTestSPATAPI) DeleteSystemAccountsIDAccessTokensID(
	_ context.Context,
	accountID string,
	tokenID string,
	_ ...kkOps.Option,
) (*kkOps.DeleteSystemAccountsIDAccessTokensIDResponse, error) {
	a.deletedAccountID = accountID
	a.deletedTokenID = tokenID
	return &kkOps.DeleteSystemAccountsIDAccessTokensIDResponse{}, nil
}

func walkCommandsForTest(command *cobra.Command, path []string, visit func(*cobra.Command, []string)) {
	visit(command, path)
	for _, child := range command.Commands() {
		if child.Hidden {
			continue
		}
		childPath := append(append([]string{}, path...), child.Name())
		walkCommandsForTest(child, childPath, visit)
	}
}

func assertConciseErrorUX(t *testing.T, stderr, commandPath string) {
	t.Helper()
	if !strings.Contains(stderr, "Error:") {
		t.Fatalf("expected Error line\nstderr:\n%s", stderr)
	}
	if strings.Contains(stderr, "Usage:") {
		t.Fatalf("expected no full usage text\nstderr:\n%s", stderr)
	}
	help := fmt.Sprintf("Run '%s --help' for usage", commandPath)
	if !strings.Contains(stderr, help) {
		t.Fatalf("expected help hint %q\nstderr:\n%s", help, stderr)
	}
	if strings.Contains(stderr, help+".") {
		t.Fatalf("expected help hint without trailing period\nstderr:\n%s", stderr)
	}
}

func commandPathForTest(path []string) string {
	if len(path) == 0 {
		return "kongctl"
	}
	return "kongctl " + strings.Join(path, " ")
}

func assertAvailableSubcommands(t *testing.T, stderr string, command *cobra.Command) {
	t.Helper()

	subcommands := cmdpkg.AvailableSubcommands(command)
	if len(subcommands) == 0 {
		t.Fatalf("expected available subcommands for %s", command.CommandPath())
	}
	if !strings.Contains(stderr, "Available subcommands:") {
		t.Fatalf("expected available subcommands header\nstderr:\n%s", stderr)
	}
	for _, subcommand := range subcommands {
		line := fmt.Sprintf("  %s\n", subcommand)
		if !strings.Contains(stderr, line) {
			t.Fatalf("expected subcommand %q in stderr\nstderr:\n%s", subcommand, stderr)
		}
	}

	help := fmt.Sprintf("Run '%s --help' for usage", command.CommandPath())
	lastSubcommandLine := fmt.Sprintf("  %s\n", subcommands[len(subcommands)-1])
	if strings.LastIndex(stderr, help) < strings.LastIndex(stderr, lastSubcommandLine) {
		t.Fatalf("expected help hint after subcommand list\nstderr:\n%s", stderr)
	}
}

func helpSectionForTest(t *testing.T, help, header string) string {
	t.Helper()
	start := strings.Index(help, header)
	if start < 0 {
		t.Fatalf("expected help to contain %q\nhelp:\n%s", header, help)
	}
	section := help[start:]
	if before, _, ok := strings.Cut(section, "\n\nAvailable Commands:"); ok {
		return strings.TrimSpace(before)
	}
	if before, _, ok := strings.Cut(section, "\n\nFlags:"); ok {
		return strings.TrimSpace(before)
	}
	if before, _, ok := strings.Cut(section, "\n\nUse \""); ok {
		return strings.TrimSpace(before)
	}
	return strings.TrimSpace(section)
}

func firstAvailableChildName(command *cobra.Command) string {
	for _, child := range command.Commands() {
		if child.IsAvailableCommand() {
			return child.Name()
		}
	}
	return ""
}

func typoForTest(value string) string {
	if len(value) == 0 {
		return "x"
	}
	return value + "x"
}
