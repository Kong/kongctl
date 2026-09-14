package extensions

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kong/kongctl/internal/build"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

const persistentManifest = `schema_version: 1
publisher: kong
name: ai
runtime:
  command: run
command_paths:
  - path: [{name: ai, aliases: [assistant]}]
    persistent_flags:
      - {name: context, type: string, description: Invocation context}
      - {name: verbose, type: bool, description: Verbose output}
  - path: [{name: ai}, {name: status}]
  - path: [{name: ai}, {name: nested}, {name: status, aliases: [st]}]
  - path: [{name: get}, {name: ai}]
    persistent_flags:
      - {name: context, type: string}
  - path: [{name: get}, {name: ai}, {name: status}]
`

func TestPersistentFlagInvocation(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		path string
		want []string
		err  string
	}{
		{
			"before",
			[]string{"ai", "--context", "example", "status"},
			"kongctl ai status",
			[]string{"--context", "example"},
			"",
		},
		{"after", []string{"ai", "status", "--context=example"}, "kongctl ai status", []string{"--context=example"}, ""},
		{
			"repeated",
			[]string{"ai", "--context=first", "nested", "--verbose", "st", "--context", "last", "--verbose=false"},
			"kongctl ai nested status",
			[]string{"--context=first", "--verbose", "--context", "last", "--verbose=false"},
			"",
		},
		{
			"alias",
			[]string{"assistant", "--context", "status", "status"},
			"kongctl ai status",
			[]string{"--context", "status"},
			"",
		},
		{"value is command", []string{"ai", "--context", "status"}, "kongctl ai", []string{"--context", "status"}, ""},
		{"empty", []string{"ai", "--context=", "status"}, "kongctl ai status", []string{"--context="}, ""},
		{"bool", []string{"ai", "--verbose", "status"}, "kongctl ai status", []string{"--verbose"}, ""},
		{
			"bool separated value is positional",
			[]string{"ai", "--verbose", "false", "status"},
			"kongctl ai",
			[]string{"--verbose", "false", "status"},
			"",
		},
		{"missing", []string{"ai", "status", "--context"}, "", nil, "requires a value"},
		{"missing before separator", []string{"ai", "--context", "--", "status"}, "", nil, "requires a value"},
		{"invalid bool", []string{"ai", "--verbose=invalid", "status"}, "", nil, "invalid syntax"},
		{
			"separator",
			[]string{"ai", "--", "literal", "status", "--help"},
			"kongctl ai",
			[]string{"literal", "status", "--help"},
			"",
		},
		{"separator before child", []string{"ai", "--", "status"}, "kongctl ai", []string{"status"}, ""},
		{
			"host-like value",
			[]string{"ai", "--context", "--help", "status"},
			"kongctl ai status",
			[]string{"--context", "--help"},
			"",
		},
		{
			"host flags",
			[]string{"--profile", "dev", "ai", "--output", "json", "--context=x", "status", "--profile", "prod"},
			"kongctl ai status",
			[]string{"--context=x"},
			"",
		},
		{
			"shared root",
			[]string{"get", "ai", "--context", "example", "status"},
			"kongctl get ai status",
			[]string{"--context", "example"},
			"",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := testRootCommand()
			root.TraverseChildren = true
			root.PersistentFlags().String("profile", "default", "")
			root.PersistentFlags().String("output", "text", "")
			root.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
				if tc.name == "host flags" {
					require.Equal(t, "prod", root.PersistentFlags().Lookup("profile").Value.String())
					require.Equal(t, "json", root.PersistentFlags().Lookup("output").Value.String())
				}
				return nil
			}
			ext := mustExtension(t, persistentManifest)
			require.NoError(t, RegisterCommands(root, NewStore(t.TempDir()), []Extension{ext}))
			called := false
			for _, contribution := range ext.CommandPaths {
				command, _, err := root.Find(CommandPathNames(contribution))
				require.NoError(t, err)
				command.RunE = func(command *cobra.Command, _ []string) error {
					called = true
					invocation := command.Context().Value(invocationKey{}).(*extensionInvocation)
					require.Equal(t, tc.args, invocation.original)
					require.Equal(t, tc.path, command.CommandPath())
					cfg := newTestHook()
					split, err := SplitExtensionArgs(command, invocation.args, cfg)
					require.NoError(t, err)
					require.Equal(t, tc.want, split.Remaining)
					if tc.name == "host flags" {
						require.Equal(t, "prod", split.ProfileOverride)
						require.Equal(t, "json", cfg.GetString(cmdcommon.OutputConfigPath))
					}
					return nil
				}
			}
			_, err := ExecuteContextC(t.Context(), root, tc.args)
			if tc.err != "" {
				require.ErrorContains(t, err, tc.err)
				require.False(t, called)
			} else {
				require.NoError(t, err)
				require.True(t, called)
			}
		})
	}
}

func TestPersistentFlagValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*Manifest)
		want string
	}{
		{"type", func(m *Manifest) { m.CommandPaths[0].PersistentFlags[0].Type = "int" }, "unsupported type"},
		{"required type", func(m *Manifest) { m.CommandPaths[0].PersistentFlags[0].Type = "" }, "unsupported type"},
		{"name", func(m *Manifest) { m.CommandPaths[0].PersistentFlags[0].Name = "--bad" }, "must match"},
		{"duplicate", func(m *Manifest) {
			m.CommandPaths[0].PersistentFlags[1] = m.CommandPaths[0].PersistentFlags[0]
		}, "duplicate"},
		{"inheritance", func(m *Manifest) {
			m.CommandPaths[1].PersistentFlags = []Flag{{Name: "context", Type: "bool"}}
		}, "inherited"},
		{"metadata", func(m *Manifest) { m.CommandPaths[1].Flags = []Flag{{Name: "context"}} }, "help-only"},
		{"host", func(m *Manifest) { m.CommandPaths[0].PersistentFlags[0].Name = "profile" }, "host flag"},
		{"help", func(m *Manifest) { m.CommandPaths[0].PersistentFlags[0].Name = "help" }, "host flag"},
		{"shared root", func(m *Manifest) { m.CommandPaths[0].Path = []PathSegment{{Name: "get"}} }, "shared built-in root"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := mustExtension(t, persistentManifest).Manifest
			tc.edit(&m)
			require.ErrorContains(t, NormalizeAndValidateManifest(&m), tc.want)
		})
	}
	t.Run("actual host flags", func(t *testing.T) {
		root := testRootCommand()
		root.PersistentFlags().String("context", "", "host flag")
		err := RegisterCommands(root, NewStore(t.TempDir()), []Extension{mustExtension(t, persistentManifest)})
		require.ErrorContains(t, err, "host flag")
		require.Nil(t, findChildByName(root, "ai"))
	})
}

func TestPersistentFlagsHelpAndIsolation(t *testing.T) {
	for _, args := range [][]string{
		{"ai", "--help"},
		{"ai", "status", "--help"},
		{"ai", "nested", "--help"},
		{"ai", "nested", "status", "--help"},
		{"ai", "--help", "status"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			root := testRootCommand()
			root.TraverseChildren = true
			require.NoError(t, RegisterCommands(root, NewStore(t.TempDir()), []Extension{mustExtension(t, persistentManifest)}))
			var out bytes.Buffer
			root.SetOut(&out)
			ctx := context.WithValue(t.Context(), config.ConfigKey, newTestHook())
			ctx = context.WithValue(ctx, build.InfoKey, &build.Info{Version: "dev"})
			_, err := ExecuteContextC(ctx, root, args)
			require.NoError(t, err)
			require.Contains(t, out.String(), "--context")
			require.Contains(t, out.String(), "--verbose")
		})
	}
	root := testRootCommand()
	root.TraverseChildren = true
	require.NoError(t, RegisterCommands(root, NewStore(t.TempDir()), []Extension{mustExtension(t, persistentManifest)}))
	for _, args := range [][]string{{"get", "apis"}, {"list"}, {"get", "--context=x", "apis"}} {
		routing, invocation, err := prepareExtensionInvocation(root, args)
		require.NoError(t, err)
		require.Nil(t, invocation)
		require.Equal(t, args, routing)
	}
	require.Nil(t, findChildByName(root, "get").PersistentFlags().Lookup("context"))
}

func TestPersistentFlagsDispatchContextAndArgv(t *testing.T) {
	source := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(source, ManifestFileName), []byte(persistentManifest), 0o600))
	// Copy the context and print one argv token per line before Dispatch removes
	// its temporary context file.
	script := "#!/bin/sh\ncat \"$KONGCTL_EXTENSION_CONTEXT\" > \"$CAPTURE_CONTEXT\"\nprintf '%s\\n' \"$@\"\n"
	runtimePath := filepath.Join(source, "run")
	require.NoError(t, os.WriteFile(runtimePath, []byte(script), 0o600))
	require.NoError(t, os.Chmod(runtimePath, 0o700))
	capture := filepath.Join(t.TempDir(), "context.json")
	t.Setenv("CAPTURE_CONTEXT", capture)
	store := NewStore(t.TempDir())
	ext, err := store.LinkLocal(source, "dev", time.Now())
	require.NoError(t, err)
	root := testRootCommand()
	root.TraverseChildren = true
	require.NoError(t, RegisterCommands(root, store, []Extension{ext}))
	streams := iostreams.NewTestIOStreamsOnly()
	var out bytes.Buffer
	streams.Out = &out
	ctx := context.WithValue(t.Context(), config.ConfigKey, newTestHook())
	ctx = context.WithValue(ctx, build.InfoKey, &build.Info{Version: "dev"})
	ctx = context.WithValue(ctx, iostreams.StreamsKey, streams)
	args := []string{"assistant", "--context=one", "nested", "--verbose", "st", "--context", "two"}
	_, err = ExecuteContextC(ctx, root, args)
	require.NoError(t, err)
	data, err := os.ReadFile(capture)
	require.NoError(t, err)
	var runtimeCtx RuntimeContext
	require.NoError(t, json.Unmarshal(data, &runtimeCtx))
	require.Equal(t, args, runtimeCtx.Invocation.OriginalArgs)
	require.Equal(t, []string{"--context=one", "--verbose", "--context", "two"}, runtimeCtx.Invocation.RemainingArgs)
	require.Equal(t, "--context=one\n--verbose\n--context\ntwo\n", out.String())
}

func TestPersistentFlagsLeaveOtherCommandsUnchanged(t *testing.T) {
	root := testRootCommand()
	root.TraverseChildren = true
	store := NewStore(t.TempDir())
	require.NoError(t, RegisterCommands(root, store, []Extension{mustExtension(t, persistentManifest)}))
	legacy := mustExtension(t, `schema_version: 1
publisher: kong
name: legacy
runtime: {command: run}
command_paths:
  - path: [{name: legacy}]
    flags: [{name: context, type: anything}]
  - path: [{name: legacy}, {name: status, aliases: [st]}]
  - path: [{name: list}, {name: legacy}]
`)
	require.NoError(t, RegisterCommands(root, store, []Extension{legacy}))
	for _, args := range [][]string{
		{"legacy", "--context", "example", "st"},
		{"legacy", "st", "--context", "example"},
		{"legacy", "--help"},
		{"list", "legacy"},
	} {
		routing, invocation, err := prepareExtensionInvocation(root, args)
		require.NoError(t, err)
		require.Nil(t, invocation)
		require.Equal(t, args, routing)
	}
	for _, args := range [][]string{{"get", "apis"}, {"get", "api-alias"}} {
		builtIn := findChildByName(findChildByName(root, "get"), "apis")
		builtIn.Aliases = []string{"api-alias"}
		called := false
		builtIn.RunE = func(_ *cobra.Command, _ []string) error { called = true; return nil }
		_, err := ExecuteContextC(t.Context(), root, args)
		require.NoError(t, err)
		require.True(t, called)
	}
}

func TestPersistentFlagsStaySeparateAcrossExtensions(t *testing.T) {
	root := testRootCommand()
	root.TraverseChildren = true
	other := mustExtension(t, `schema_version: 1
publisher: kong
name: tools
runtime: {command: run}
command_paths:
  - path: [{name: tools}]
    persistent_flags:
      - {name: context, type: bool}
      - {name: dry-run, type: bool}
  - path: [{name: tools}, {name: status}]
`)
	extensions := []Extension{mustExtension(t, persistentManifest), other}
	require.NoError(t, RegisterCommands(root, NewStore(t.TempDir()), extensions))
	ai := findChildByName(root, "ai")
	tools := findChildByName(root, "tools")
	aiContext := ai.PersistentFlags().Lookup("context")
	toolsContext := tools.PersistentFlags().Lookup("context")
	require.NotSame(t, aiContext, toolsContext)
	require.Equal(t, "string", aiContext.Value.Type())
	require.Equal(t, "bool", toolsContext.Value.Type())
	require.NoError(t, aiContext.Value.Set("ai-only"))
	require.Equal(t, "false", toolsContext.Value.String())
	require.NoError(t, toolsContext.Value.Set("true"))
	require.Equal(t, "ai-only", aiContext.Value.String())
	for _, tc := range []struct {
		parent *cobra.Command
		args   []string
		want   []string
		absent string
	}{
		{
			ai,
			[]string{"ai", "--context", "first", "status", "--context=last"},
			[]string{"--context", "first", "--context=last"},
			"dry-run",
		},
		{
			tools,
			[]string{"tools", "--context", "status", "--context=false"},
			[]string{"--context", "--context=false"},
			"verbose",
		},
	} {
		t.Run(tc.parent.Name(), func(t *testing.T) {
			child := findChildByName(tc.parent, "status")
			require.Same(t, tc.parent.PersistentFlags().Lookup("context"), child.InheritedFlags().Lookup("context"))
			require.Nil(t, child.InheritedFlags().Lookup(tc.absent))
			called := false
			child.RunE = func(command *cobra.Command, _ []string) error {
				called = true
				invocation := command.Context().Value(invocationKey{}).(*extensionInvocation)
				require.Same(t, child, invocation.command)
				split, err := SplitExtensionArgs(command, invocation.args, newTestHook())
				require.NoError(t, err)
				require.Equal(t, tc.want, split.Remaining)
				return nil
			}
			_, err := ExecuteContextC(t.Context(), root, tc.args)
			require.NoError(t, err)
			require.True(t, called)
		})
	}
}

func TestPersistentFlagExampleManifests(t *testing.T) {
	for _, example := range []string{"go", "script"} {
		path := filepath.Join("..", "..", "docs", "examples", "extensions", example, ManifestFileName)
		manifest, _, err := LoadManifestFile(path)
		require.NoError(t, err)
		require.NoError(t, ValidateExtensionCommands(testRootCommand(), Extension{CommandPaths: manifest.CommandPaths}))
	}
}
