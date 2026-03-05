package install

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	skillassets "github.com/kong/kongctl/skills"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"
)

const (
	defaultCanonicalSkillsPath = ".kongctl/skills"
	skillsManifestFileName     = ".kongctl-skills-manifest.json"
)

// toolIntegration describes a tool-specific directory where per-skill
// symlinks are created so that the tool can discover installed skills.
type toolIntegration struct {
	relPath string
	tools   string
}

// toolIntegrations lists directories that receive symlinks to the canonical
// skill install location. Each entry corresponds to a set of agent tools
// that read skills from that path.
var toolIntegrations = []toolIntegration{
	{relPath: ".agent/skills", tools: "codex, cursor, opencode"},
	{relPath: ".claude/skills", tools: "claude code"},
}

type installSkillsOptions struct {
	path   string
	dryRun bool
}

type bundledSkillAsset struct {
	SkillName string
	RelPath   string
	EmbedPath string
}

type skillsInstallManifest struct {
	CLIVersion  string   `json:"cli_version"`
	InstalledAt string   `json:"installed_at"`
	Skills      []string `json:"skills"`
}

type skillsInstallResult struct {
	CanonicalDir string
	CLIVersion   string
	SkillNames   []string
	WrittenFiles []string
	ManifestPath string
	Symlinks     []plannedSymlink
	DryRun       bool
}

type plannedSymlink struct {
	LinkPath   string
	TargetPath string
	RelTarget  string
	SkillName  string
	Tools      string
}

type symlinkConflict struct {
	linkPath string
	detail   string
}

func newInstallSkillsCmd() *cobra.Command {
	opts := &installSkillsOptions{
		path: defaultCanonicalSkillsPath,
	}

	cmd := &cobra.Command{
		Use:   "skills",
		Short: i18n.T("root.verbs.install.skills.short", "Install kongctl agent skills"),
		Long: i18n.T("root.verbs.install.skills.long",
			"Install versioned kongctl skills into a local agent skills directory."),
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			return runInstallSkills(command, args, *opts)
		},
	}

	cmd.Flags().StringVar(
		&opts.path,
		"path",
		defaultCanonicalSkillsPath,
		"Canonical directory for installed skill files.",
	)
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Show planned writes without creating files.")

	return cmd
}

func runInstallSkills(command *cobra.Command, args []string, opts installSkillsOptions) error {
	helper := cmdpkg.BuildHelper(command, args)
	streams := helper.GetStreams()

	buildInfo, err := helper.GetBuildInfo()
	if err != nil {
		return err
	}

	version := strings.TrimSpace(buildInfo.Version)
	if version == "" {
		version = meta.DefaultCLIVersion
	}

	cwd, err := os.Getwd()
	if err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to determine current directory", err)
	}

	canonicalDir, err := resolveInstallTargetDir(cwd, opts.path)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}

	_, skillNames, err := listBundledSkillAssets()
	if err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to list bundled skills", err)
	}

	planned := planSymlinks(cwd, canonicalDir, skillNames)

	if !opts.dryRun {
		if conflicts := checkSymlinkConflicts(planned); len(conflicts) > 0 {
			return &cmdpkg.ConfigurationError{Err: formatConflictError(conflicts)}
		}
	}

	result, err := installBundledSkills(canonicalDir, version, opts.dryRun, time.Now().UTC())
	if err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to install skills", err)
	}

	if err := createToolSymlinks(planned, opts.dryRun); err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to create tool symlinks", err)
	}
	result.Symlinks = planned

	if err := printSkillsInstallSummary(streams.Out, cwd, result); err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to write install output", err)
	}

	return nil
}

func resolveInstallTargetDir(cwd, path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("--path cannot be empty")
	}

	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed), nil
	}

	return filepath.Clean(filepath.Join(cwd, trimmed)), nil
}

// planSymlinks computes the symlinks to create for each skill in each tool
// integration directory. If a tool directory resolves to the same path as
// the canonical directory, it is skipped to avoid circular symlinks.
func planSymlinks(cwd, canonicalDir string, skillNames []string) []plannedSymlink {
	var links []plannedSymlink
	for _, ti := range toolIntegrations {
		toolDir := filepath.Clean(filepath.Join(cwd, ti.relPath))
		if toolDir == filepath.Clean(canonicalDir) {
			continue
		}
		for _, name := range skillNames {
			linkPath := filepath.Join(toolDir, name)
			targetPath := filepath.Join(canonicalDir, name)
			relTarget, err := filepath.Rel(filepath.Dir(linkPath), targetPath)
			if err != nil {
				relTarget = targetPath
			}
			links = append(links, plannedSymlink{
				LinkPath:   linkPath,
				TargetPath: targetPath,
				RelTarget:  relTarget,
				SkillName:  name,
				Tools:      ti.tools,
			})
		}
	}
	return links
}

// checkSymlinkConflicts inspects every planned symlink path. A path that
// does not exist or that is already a symlink pointing to the expected
// canonical target (re-install case) is not a conflict. Anything else is.
func checkSymlinkConflicts(links []plannedSymlink) []symlinkConflict {
	var conflicts []symlinkConflict
	for _, link := range links {
		info, err := os.Lstat(link.LinkPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			conflicts = append(conflicts, symlinkConflict{link.LinkPath, err.Error()})
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			existing, readErr := os.Readlink(link.LinkPath)
			if readErr == nil && isMatchingSymlink(link.LinkPath, existing, link.TargetPath) {
				continue
			}
		}
		conflicts = append(conflicts, symlinkConflict{
			linkPath: link.LinkPath,
			detail:   "path already exists and is not a kongctl-managed symlink",
		})
	}
	return conflicts
}

// isMatchingSymlink returns true when existingTarget (the value read from a
// symlink at linkPath) resolves to the same location as expectedTarget.
func isMatchingSymlink(linkPath, existingTarget, expectedTarget string) bool {
	if existingTarget == expectedTarget {
		return true
	}
	absExisting := existingTarget
	if !filepath.IsAbs(existingTarget) {
		absExisting = filepath.Join(filepath.Dir(linkPath), existingTarget)
	}
	return filepath.Clean(absExisting) == filepath.Clean(expectedTarget)
}

func createToolSymlinks(links []plannedSymlink, dryRun bool) error {
	if dryRun {
		return nil
	}
	for _, link := range links {
		if err := os.MkdirAll(filepath.Dir(link.LinkPath), 0o755); err != nil {
			return fmt.Errorf("create directory for symlink %q: %w", link.LinkPath, err)
		}
		// Remove existing kongctl-managed symlink on re-install.
		// Safe because the conflict check already passed.
		_ = os.Remove(link.LinkPath)
		if err := os.Symlink(link.RelTarget, link.LinkPath); err != nil {
			return fmt.Errorf("create symlink %q: %w", link.LinkPath, err)
		}
	}
	return nil
}

func formatConflictError(conflicts []symlinkConflict) error {
	var b strings.Builder
	fmt.Fprintf(&b, "cannot install: %d path conflict(s) detected\n", len(conflicts))
	for _, c := range conflicts {
		fmt.Fprintf(&b, "  %s: %s\n", c.linkPath, c.detail)
	}
	fmt.Fprint(&b, "Remove or rename the conflicting paths and re-run the install command.")
	return fmt.Errorf("%s", b.String())
}

func installBundledSkills(
	canonicalDir string,
	cliVersion string,
	dryRun bool,
	now time.Time,
) (skillsInstallResult, error) {
	assets, skillNames, err := listBundledSkillAssets()
	if err != nil {
		return skillsInstallResult{}, err
	}
	if len(assets) == 0 {
		return skillsInstallResult{}, fmt.Errorf("no bundled skill assets found")
	}

	result := skillsInstallResult{
		CanonicalDir: canonicalDir,
		CLIVersion:   cliVersion,
		SkillNames:   skillNames,
		DryRun:       dryRun,
	}

	if !dryRun {
		if err := os.MkdirAll(canonicalDir, 0o755); err != nil {
			return skillsInstallResult{}, fmt.Errorf("create canonical directory: %w", err)
		}
	}

	for _, asset := range assets {
		content, err := skillassets.BundledFS.ReadFile(asset.EmbedPath)
		if err != nil {
			return skillsInstallResult{}, fmt.Errorf("read bundled skill asset %q: %w", asset.EmbedPath, err)
		}

		if filepath.Base(asset.RelPath) == "SKILL.md" {
			content, err = injectSkillMetadataVersion(content, cliVersion)
			if err != nil {
				return skillsInstallResult{}, fmt.Errorf("update version for %q: %w", asset.RelPath, err)
			}
		}

		destPath := filepath.Join(canonicalDir, filepath.FromSlash(asset.RelPath))
		result.WrittenFiles = append(result.WrittenFiles, destPath)

		if dryRun {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return skillsInstallResult{}, fmt.Errorf("create destination directory for %q: %w", destPath, err)
		}

		if err := os.WriteFile(destPath, content, 0o600); err != nil {
			return skillsInstallResult{}, fmt.Errorf("write %q: %w", destPath, err)
		}
	}

	manifestPath := filepath.Join(canonicalDir, skillsManifestFileName)
	result.ManifestPath = manifestPath
	result.WrittenFiles = append(result.WrittenFiles, manifestPath)

	if !dryRun {
		manifest := skillsInstallManifest{
			CLIVersion:  cliVersion,
			InstalledAt: now.Format(time.RFC3339),
			Skills:      skillNames,
		}

		manifestData, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return skillsInstallResult{}, fmt.Errorf("marshal install manifest: %w", err)
		}
		manifestData = append(manifestData, '\n')

		if err := os.WriteFile(manifestPath, manifestData, 0o600); err != nil {
			return skillsInstallResult{}, fmt.Errorf("write manifest %q: %w", manifestPath, err)
		}
	}

	return result, nil
}

func listBundledSkillAssets() ([]bundledSkillAsset, []string, error) {
	assets := make([]bundledSkillAsset, 0)
	skillSet := map[string]struct{}{}

	err := fs.WalkDir(skillassets.BundledFS, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == "." || d.IsDir() {
			return nil
		}

		relPath := strings.TrimPrefix(path, "./")
		if relPath == "" {
			return nil
		}

		parts := strings.Split(relPath, "/")
		if len(parts) < 2 {
			return fmt.Errorf("invalid bundled skill file path: %s", path)
		}

		skillName := parts[0]
		skillSet[skillName] = struct{}{}

		assets = append(assets, bundledSkillAsset{
			SkillName: skillName,
			RelPath:   relPath,
			EmbedPath: path,
		})

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	sort.Slice(assets, func(i, j int) bool {
		return assets[i].RelPath < assets[j].RelPath
	})

	skillNames := make([]string, 0, len(skillSet))
	for skillName := range skillSet {
		skillNames = append(skillNames, skillName)
	}
	sort.Strings(skillNames)

	return assets, skillNames, nil
}

func injectSkillMetadataVersion(content []byte, version string) ([]byte, error) {
	frontmatter, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, err
	}

	frontmatterMap := map[string]any{}
	if err := yaml.Unmarshal(frontmatter, &frontmatterMap); err != nil {
		return nil, fmt.Errorf("unmarshal frontmatter: %w", err)
	}

	metadata := map[string]any{}
	if raw, ok := frontmatterMap["metadata"]; ok && raw != nil {
		if existing, ok := raw.(map[string]any); ok {
			metadata = existing
		}
	}
	metadata["version"] = strings.TrimSpace(version)
	frontmatterMap["metadata"] = metadata

	updatedFrontmatter, err := yaml.Marshal(frontmatterMap)
	if err != nil {
		return nil, fmt.Errorf("marshal frontmatter: %w", err)
	}

	final := append([]byte("---\n"), updatedFrontmatter...)
	final = append(final, []byte("---\n")...)
	final = append(final, body...)

	return final, nil
}

func splitFrontmatter(content []byte) ([]byte, []byte, error) {
	const opening = "---\n"
	const separator = "\n---\n"

	text := string(content)
	if !strings.HasPrefix(text, opening) {
		return nil, nil, fmt.Errorf("skill file missing YAML frontmatter opening delimiter")
	}

	rest := text[len(opening):]
	idx, found := strings.CutPrefix(rest, "")
	if !found {
		idx = rest
	}
	sepIdx := strings.Index(idx, separator)
	if sepIdx < 0 {
		return nil, nil, fmt.Errorf("skill file missing YAML frontmatter closing delimiter")
	}

	frontmatter := []byte(rest[:sepIdx])
	body := []byte(rest[sepIdx+len(separator):])

	return frontmatter, body, nil
}

func printSkillsInstallSummary(out io.Writer, cwd string, result skillsInstallResult) error {
	if out == nil {
		return nil
	}

	action := "Installed"
	if result.DryRun {
		action = "Planned"
	}

	relCanonical, err := filepath.Rel(cwd, result.CanonicalDir)
	if err != nil {
		relCanonical = result.CanonicalDir
	}

	if _, err := fmt.Fprintf(out, "%s %d kongctl skill(s) to %s\n",
		action, len(result.SkillNames), relCanonical); err != nil {
		return err
	}

	for _, name := range result.SkillNames {
		if _, err := fmt.Fprintf(out, "- %s\n", name); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(out, "Skill metadata.version: %s\n", result.CLIVersion); err != nil {
		return err
	}

	relManifest, err := filepath.Rel(cwd, result.ManifestPath)
	if err != nil {
		relManifest = result.ManifestPath
	}
	if _, err := fmt.Fprintf(out, "Install manifest: %s\n", relManifest); err != nil {
		return err
	}

	if result.DryRun {
		if _, err := fmt.Fprintln(out, "\nDry run only. No files were written."); err != nil {
			return err
		}
	}

	if len(result.Symlinks) > 0 {
		if _, err := fmt.Fprintln(out, "\nTool integrations:"); err != nil {
			return err
		}
		type toolGroup struct {
			tools  string
			relDir string
		}
		var groups []toolGroup
		seen := map[string]bool{}
		for _, sl := range result.Symlinks {
			if seen[sl.Tools] {
				continue
			}
			seen[sl.Tools] = true
			dir := filepath.Dir(sl.LinkPath)
			relDir, relErr := filepath.Rel(cwd, dir)
			if relErr != nil {
				relDir = dir
			}
			groups = append(groups, toolGroup{tools: sl.Tools, relDir: relDir})
		}
		for _, g := range groups {
			if _, err := fmt.Fprintf(out, "  %s/ (%s)\n", g.relDir, g.tools); err != nil {
				return err
			}
		}
	}

	if _, err := fmt.Fprintln(out, "\nExample agent prompts:"); err != nil {
		return err
	}
	prompts := []string{
		`"Show me my Kong Konnect control planes"`,
		`"Generate declarative config for a portal with two APIs from my OpenAPI specs"`,
	}
	for _, p := range prompts {
		if _, err := fmt.Fprintf(out, "  %s\n", p); err != nil {
			return err
		}
	}

	return nil
}
