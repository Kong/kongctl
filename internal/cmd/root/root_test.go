package root

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/common"
	configpkg "github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
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
	}
	if strings.Contains(outputFlag.Usage, "helm") {
		t.Fatalf("expected merged usage rendering not to mutate root output flag usage, got:\n%s", outputFlag.Usage)
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
			name: "bare root requires command",
			args: []string{},
			wantErr: []string{
				`Error: command "kongctl" requires a subcommand`,
				`Run 'kongctl --help' for usage.`,
			},
			wantExit:     1,
			forbidErr:    []string{"Usage:"},
			expectStderr: true,
		},
		{
			name: "bare command group requires subcommand",
			args: []string{"get"},
			wantErr: []string{
				`Error: command "kongctl get" requires a subcommand`,
				`Run 'kongctl get --help' for usage.`,
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
				`Run 'kongctl --help' for usage.`,
				"Did you mean this command?",
				"  apply",
			},
			wantExit:     1,
			forbidErr:    []string{"Usage:"},
			expectStderr: true,
		},
		{
			name: "unknown nested command suggests close match",
			args: []string{"get", "gatewy"},
			wantErr: []string{
				`Error: unknown command "gatewy" for "kongctl get"`,
				`Run 'kongctl get --help' for usage.`,
				"Did you mean this command?",
				"  gateway",
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
				`Run 'kongctl version --help' for usage.`,
				"Did you mean this flag?",
				"  --log-level",
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
				`Run 'kongctl scaffold --help' for usage.`,
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
				`Run 'kongctl plan --help' for usage.`,
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
	paths := collectRequiresSubcommandPathsForTest(t)
	for _, path := range paths {
		t.Run(commandPathForTest(path), func(t *testing.T) {
			result := executeRootForTest(t, path...)
			if result.exitCode != 1 {
				t.Fatalf("expected exit code 1, got %d\nstdout:\n%s\nstderr:\n%s",
					result.exitCode, result.stdout, result.stderr)
			}
			assertConciseErrorUX(t, result.stderr, commandPathForTest(path))
			if !strings.Contains(result.stderr, "requires a subcommand") {
				t.Fatalf("expected missing subcommand error\nstderr:\n%s", result.stderr)
			}
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
	oldEnableTraverseRunHooks := cobra.EnableTraverseRunHooks
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
		if logFile != nil && logFile != oldLogFile {
			_ = logFile.Close()
		}
		logFile = oldLogFile
		cobra.EnableTraverseRunHooks = oldEnableTraverseRunHooks
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

func collectRequiresSubcommandPathsForTest(t *testing.T) [][]string {
	t.Helper()
	items := collectRequiresSubcommandCommandsForTest(t)
	paths := make([][]string, 0, len(items))
	for _, item := range items {
		paths = append(paths, item.path)
	}
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
	help := fmt.Sprintf("Run '%s --help' for usage.", commandPath)
	if !strings.Contains(stderr, help) {
		t.Fatalf("expected help hint %q\nstderr:\n%s", help, stderr)
	}
}

func commandPathForTest(path []string) string {
	if len(path) == 0 {
		return "kongctl"
	}
	return "kongctl " + strings.Join(path, " ")
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
