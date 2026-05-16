//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kong/kongctl/test/e2e/harness"
)

const (
	scriptExtensionID = "kong/e2e-script"
	goExtensionID     = "kong/e2e-go"
)

type extensionRecord struct {
	ID           string                 `json:"id"`
	InstallType  string                 `json:"install_type"`
	CommandPaths []extensionCommandPath `json:"command_paths"`
	Install      *extensionInstallState `json:"install,omitempty"`
}

type extensionCommandPath struct {
	ID   string                 `json:"id"`
	Path []extensionPathSegment `json:"path"`
}

type extensionPathSegment struct {
	Name string `json:"name"`
}

type extensionInstallState struct {
	Source extensionSourceState `json:"source"`
}

type extensionSourceState struct {
	Type       string `json:"type"`
	Repository string `json:"repository"`
	Ref        string `json:"ref"`
	ReleaseTag string `json:"release_tag"`
}

type extensionInstallResult struct {
	Extension extensionRecord `json:"extension"`
}

type extensionUpgradeAllResult struct {
	Upgraded []string `json:"upgraded"`
	UpToDate []string `json:"up_to_date"`
}

type extensionRuntimeContext struct {
	MatchedCommandPath struct {
		ID          string   `json:"id"`
		ExtensionID string   `json:"extension_id"`
		Path        []string `json:"path"`
	} `json:"matched_command_path"`
	Invocation struct {
		OriginalArgs  []string `json:"original_args"`
		RemainingArgs []string `json:"remaining_args"`
	} `json:"invocation"`
	Resolved struct {
		Profile          string `json:"profile"`
		Output           string `json:"output"`
		ExtensionDataDir string `json:"extension_data_dir"`
	} `json:"resolved"`
	Output struct {
		Format string `json:"format"`
		JQ     struct {
			Expression string `json:"expression"`
			RawOutput  bool   `json:"raw_output"`
		} `json:"jq"`
	} `json:"output"`
	Session struct {
		Depth    int `json:"depth"`
		MaxDepth int `json:"max_depth"`
	} `json:"session"`
	Host struct {
		KongctlPath string `json:"kongctl_path"`
	} `json:"host"`
}

type goFixtureOutput struct {
	Kind         string   `json:"kind"`
	Args         []string `json:"args"`
	Profile      string   `json:"profile"`
	OutputFormat string   `json:"output_format"`
	DataDirSet   bool     `json:"data_dir_set"`
}

func TestE2E_ExtensionsScriptLifecycle(t *testing.T) {
	cli := newExtensionCLI(t)
	fixtureDir := prepareScriptExtensionFixture(t, cli)

	var linked extensionRecord
	runKongctlJSON(t, cli, &linked, "link", "extension", fixtureDir)
	requireEqual(t, scriptExtensionID, linked.ID, "linked extension id")
	requireEqual(t, "linked", linked.InstallType, "linked extension install type")

	var listed []extensionRecord
	runKongctlJSON(t, cli, &listed, "list", "extensions")
	requireExtensionListed(t, listed, scriptExtensionID, "linked")

	var detail extensionRecord
	runKongctlJSON(t, cli, &detail, "get", "extension", scriptExtensionID)
	requireEqual(t, scriptExtensionID, detail.ID, "linked extension detail id")
	requireEqual(t, "linked", detail.InstallType, "linked extension detail type")

	var runtimeCtx extensionRuntimeContext
	runKongctlJSON(t, cli, &runtimeCtx, "get", "e2e-script", "alpha", "--script-flag", "beta")
	requireEqual(t, scriptExtensionID, runtimeCtx.MatchedCommandPath.ExtensionID, "script runtime extension id")
	requireStringSliceEqual(t, []string{"get", "e2e-script"}, runtimeCtx.MatchedCommandPath.Path, "script command path")
	requireStringSliceEqual(t, []string{"alpha", "--script-flag", "beta"}, runtimeCtx.Invocation.RemainingArgs,
		"script remaining args")
	requireEqual(t, "json", runtimeCtx.Output.Format, "script output format")
	if runtimeCtx.Resolved.ExtensionDataDir == "" {
		t.Fatal("script runtime context did not include extension data directory")
	}
	if runtimeCtx.Host.KongctlPath == "" {
		t.Fatal("script runtime context did not include kongctl path")
	}
	if runtimeCtx.Session.Depth != 1 || runtimeCtx.Session.MaxDepth < 1 {
		t.Fatalf("unexpected script session depth: %+v", runtimeCtx.Session)
	}

	escapeLog := filepath.Join(cli.TestDir, "escape.log")
	runKongctlJSON(
		t,
		cli,
		&runtimeCtx,
		"get",
		"e2e-script",
		"--profile",
		cli.Profile,
		"--output",
		"json",
		"--log-level",
		"info",
		"--log-file",
		escapeLog,
		"--",
		"--output",
		"literal",
		"--profile",
		"literal",
	)
	requireStringSliceEqual(t, []string{"--output", "literal", "--profile", "literal"}, runtimeCtx.Invocation.RemainingArgs,
		"script escape-hatch args")

	var uninstall map[string]any
	runKongctlJSON(t, cli, &uninstall, "uninstall", "extension", scriptExtensionID)

	var installed extensionInstallResult
	runKongctlJSON(t, cli, &installed, "install", "extension", fixtureDir)
	requireEqual(t, scriptExtensionID, installed.Extension.ID, "installed extension id")
	requireEqual(t, "installed", installed.Extension.InstallType, "installed extension install type")

	runKongctlJSON(t, cli, &runtimeCtx, "get", "e2e-script")
	requireEqual(t, scriptExtensionID, runtimeCtx.MatchedCommandPath.ExtensionID, "installed script runtime extension id")

	runKongctlJSON(t, cli, &uninstall, "uninstall", "extension", scriptExtensionID, "--remove-data")
	if _, err := os.Stat(filepath.Join(cli.ConfigDir, "kongctl", "extensions", "data", "kong", "e2e-script")); err == nil {
		t.Fatal("extension data directory still exists after uninstall --remove-data")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat extension data directory: %v", err)
	}
}

func TestE2E_ExtensionsGoSDKOutput(t *testing.T) {
	cli := newExtensionCLI(t)
	fixtureDir := prepareGoExtensionFixture(t, cli)

	var linked extensionRecord
	runKongctlJSON(t, cli, &linked, "link", "extension", fixtureDir)
	requireEqual(t, goExtensionID, linked.ID, "linked Go extension id")
	requireEqual(t, "linked", linked.InstallType, "linked Go extension install type")

	var output goFixtureOutput
	runKongctlJSON(t, cli, &output, "get", "e2e-go", "one", "--fixture-flag", "two")
	requireEqual(t, "e2e-go", output.Kind, "Go fixture kind")
	requireStringSliceEqual(t, []string{"one", "--fixture-flag", "two"}, output.Args, "Go fixture args")
	requireEqual(t, cli.Profile, output.Profile, "Go fixture profile")
	requireEqual(t, "json", output.OutputFormat, "Go fixture output format")
	if !output.DataDirSet {
		t.Fatal("Go fixture did not receive an extension data directory")
	}

	res, err := cli.Run(context.Background(), "get", "e2e-go", "--output", "yaml")
	requireNoCommandError(t, res, err, "run Go fixture with yaml output")
	if !strings.Contains(res.Stdout, "kind: e2e-go") {
		t.Fatalf("yaml output did not include fixture kind:\n%s", res.Stdout)
	}

	res, err = cli.Run(context.Background(), "get", "e2e-go", "--output", "json", "--jq", ".kind", "--jq-raw-output")
	requireNoCommandError(t, res, err, "run Go fixture with jq")
	requireEqual(t, "e2e-go", strings.TrimSpace(res.Stdout), "Go fixture jq raw output")

	var uninstall map[string]any
	runKongctlJSON(t, cli, &uninstall, "uninstall", "extension", goExtensionID)

	var installed extensionInstallResult
	runKongctlJSON(t, cli, &installed, "install", "extension", fixtureDir)
	requireEqual(t, goExtensionID, installed.Extension.ID, "installed Go extension id")
	requireEqual(t, "installed", installed.Extension.InstallType, "installed Go extension type")

	runKongctlJSON(t, cli, &output, "get", "e2e-go")
	requireEqual(t, "e2e-go", output.Kind, "installed Go fixture kind")

	runKongctlJSON(t, cli, &uninstall, "uninstall", "extension", goExtensionID, "--remove-data")
}

func TestE2E_ExtensionsRemoteReleaseLifecycle(t *testing.T) {
	repo := strings.TrimSpace(os.Getenv("KONGCTL_E2E_EXTENSION_REMOTE_REPO"))
	oldTag := strings.TrimSpace(os.Getenv("KONGCTL_E2E_EXTENSION_REMOTE_OLD_TAG"))
	newTag := strings.TrimSpace(os.Getenv("KONGCTL_E2E_EXTENSION_REMOTE_NEW_TAG"))
	if repo == "" || oldTag == "" || newTag == "" {
		t.Skip("set KONGCTL_E2E_EXTENSION_REMOTE_REPO, _OLD_TAG, and _NEW_TAG to run remote extension E2E")
	}

	cli := newExtensionCLI(t)
	sourceAtOld := repo + "@" + oldTag
	sourceAtNew := repo + "@" + newTag

	var installedOld extensionInstallResult
	runKongctlJSON(t, cli, &installedOld, "install", "extension", sourceAtOld, "--yes")
	ext := installedOld.Extension
	installSource := requireInstallSource(t, ext)
	requireEqual(t, "github_release_asset", installSource.Type, "remote install source type")
	requireEqual(t, oldTag, installSource.ReleaseTag, "remote old release tag")

	runRemoteExtensionCommand(t, cli, ext)

	var upgraded extensionInstallResult
	runKongctlJSON(t, cli, &upgraded, "upgrade", "extension", sourceAtNew, "--yes")
	requireEqual(t, ext.ID, upgraded.Extension.ID, "upgraded remote extension id")
	requireEqual(t, newTag, requireInstallSource(t, upgraded.Extension).ReleaseTag, "remote explicit upgrade release tag")

	var uninstall map[string]any
	runKongctlJSON(t, cli, &uninstall, "uninstall", "extension", ext.ID, "--remove-data")

	runKongctlJSON(t, cli, &installedOld, "install", "extension", sourceAtOld, "--yes")

	var upgradedAll extensionUpgradeAllResult
	runKongctlJSON(t, cli, &upgradedAll, "upgrade", "extensions", "--yes")
	if !slices.Contains(upgradedAll.Upgraded, ext.ID) && !slices.Contains(upgradedAll.UpToDate, ext.ID) {
		t.Fatalf("upgrade all result did not include %s: %+v", ext.ID, upgradedAll)
	}

	var detail extensionRecord
	runKongctlJSON(t, cli, &detail, "get", "extension", ext.ID)
	requireEqual(t, newTag, requireInstallSource(t, detail).ReleaseTag, "remote upgrade-all release tag")

	runRemoteExtensionCommand(t, cli, detail)
	runKongctlJSON(t, cli, &uninstall, "uninstall", "extension", ext.ID, "--remove-data")
}

func newExtensionCLI(t *testing.T) *harness.CLI {
	t.Helper()
	cli, err := harness.NewCLIT(t)
	if err != nil {
		t.Fatalf("create CLI harness: %v", err)
	}
	return cli
}

func prepareScriptExtensionFixture(t *testing.T, cli *harness.CLI) string {
	t.Helper()
	target := filepath.Join(cli.TestDir, "fixtures", "script-context")
	copyTree(t, repoPath(t, "test/e2e/testdata/extensions/script-context"), target)
	requireNoError(t, os.Chmod(filepath.Join(target, "bin", "kongctl-ext-e2e-script"), 0o755),
		"chmod script fixture")
	return target
}

func prepareGoExtensionFixture(t *testing.T, cli *harness.CLI) string {
	t.Helper()
	target := filepath.Join(cli.TestDir, "fixtures", "go-output")
	source := repoPath(t, "test/e2e/testdata/extensions/go-output")
	copyTree(t, source, target)

	binPath := filepath.Join(target, "bin", "kongctl-ext-e2e-go")
	requireNoError(t, os.MkdirAll(filepath.Dir(binPath), 0o755), "create Go fixture bin dir")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", binPath, filepath.Join(source, "main.go"))
	cmd.Dir = repoPath(t, "")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build Go extension fixture: %v\n%s", err, strings.TrimSpace(string(output)))
	}
	requireNoError(t, os.Chmod(binPath, 0o755), "chmod Go fixture binary")
	return target
}

func runRemoteExtensionCommand(t *testing.T, cli *harness.CLI, ext extensionRecord) {
	t.Helper()
	args := firstCommandPathArgs(t, ext)
	res, err := cli.Run(context.Background(), args...)
	requireNoCommandError(t, res, err, "run remote extension command "+strings.Join(args, " "))
}

func firstCommandPathArgs(t *testing.T, ext extensionRecord) []string {
	t.Helper()
	if len(ext.CommandPaths) == 0 || len(ext.CommandPaths[0].Path) == 0 {
		t.Fatalf("extension %s did not include command paths", ext.ID)
	}
	args := make([]string, 0, len(ext.CommandPaths[0].Path))
	for _, segment := range ext.CommandPaths[0].Path {
		if strings.TrimSpace(segment.Name) == "" {
			t.Fatalf("extension %s included an empty command path segment", ext.ID)
		}
		args = append(args, segment.Name)
	}
	return args
}

func runKongctlJSON(t *testing.T, cli *harness.CLI, out any, args ...string) harness.Result {
	t.Helper()
	finalArgs := append([]string(nil), args...)
	if !hasOutputArg(finalArgs) {
		finalArgs = append(finalArgs, "--output", "json")
	}
	res, err := cli.Run(context.Background(), finalArgs...)
	requireNoCommandError(t, res, err, "run kongctl "+strings.Join(finalArgs, " "))

	decoder := json.NewDecoder(strings.NewReader(res.Stdout))
	if err := decoder.Decode(out); err != nil {
		t.Fatalf("decode JSON output for %q: %v\nstdout:\n%s\nstderr:\n%s",
			strings.Join(finalArgs, " "),
			err,
			res.Stdout,
			res.Stderr,
		)
	}
	return res
}

func hasOutputArg(args []string) bool {
	for i := range args {
		if args[i] == "--" {
			return false
		}
		if args[i] == "-o" || args[i] == "--output" || strings.HasPrefix(args[i], "--output=") {
			return true
		}
	}
	return false
}

func requireExtensionListed(t *testing.T, extensions []extensionRecord, id string, installType string) {
	t.Helper()
	for _, ext := range extensions {
		if ext.ID == id {
			requireEqual(t, installType, ext.InstallType, "listed extension install type")
			return
		}
	}
	t.Fatalf("extension %s not listed in %+v", id, extensions)
}

func requireInstallSource(t *testing.T, ext extensionRecord) extensionSourceState {
	t.Helper()
	if ext.Install == nil {
		t.Fatalf("extension %s did not include install metadata", ext.ID)
	}
	return ext.Install.Source
}

func copyTree(t *testing.T, source, target string) {
	t.Helper()
	requireNoError(t, os.RemoveAll(target), "remove existing fixture copy")
	requireNoError(t, filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		targetPath := filepath.Join(target, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(targetPath, info.Mode().Perm())
		}
		return copyFile(path, targetPath, info.Mode().Perm())
	}), "copy fixture tree")
}

func copyFile(source, target string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(target, mode)
}

func repoPath(t *testing.T, rel string) string {
	t.Helper()
	root := repoRoot(t)
	if strings.TrimSpace(rel) == "" {
		return root
	}
	return filepath.Join(root, filepath.FromSlash(rel))
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repository root")
		}
		dir = parent
	}
}

func requireNoCommandError(t *testing.T, res harness.Result, err error, action string) {
	t.Helper()
	if err == nil {
		return
	}
	t.Fatalf("%s failed: %v\nexit: %d\nstdout:\n%s\nstderr:\n%s", action, err, res.ExitCode, res.Stdout, res.Stderr)
}

func requireNoError(t *testing.T, err error, action string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", action, err)
	}
}

func requireEqual[T comparable](t *testing.T, want, got T, label string) {
	t.Helper()
	if want != got {
		t.Fatalf("%s: got %v, want %v", label, got, want)
	}
}

func requireStringSliceEqual(t *testing.T, want, got []string, label string) {
	t.Helper()
	if !slices.Equal(want, got) {
		t.Fatalf("%s: got %v, want %v", label, got, want)
	}
}
