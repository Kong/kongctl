package install

import (
	"bytes"
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
	localCanonicalSkillsPath = ".kongctl/skills"
	skillsManifestFileName   = ".kongctl-skills-manifest.json"
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
	{relPath: ".agents/skills", tools: "codex, cursor, opencode"},
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
	opts := &installSkillsOptions{}

	cmd := &cobra.Command{
		Use:   "skills",
		Short: i18n.T("root.verbs.install.skills.short", "Install kongctl agent skills"),
		Long: i18n.T("root.verbs.install.skills.long",
			"Install bundled kongctl skills and create symlinks for agent tool integration."),
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, args []string) error {
			return runInstallSkills(command, args, *opts)
		},
	}

	cmd.Flags().StringVar(&opts.path, "path", "",
		"Custom directory for installed skill files (default: .kongctl/skills/ in current directory).")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false,
		"Show planned writes without creating files.")

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

	canonicalDir, err := resolveCanonicalDir(cwd, opts)
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

// resolveCanonicalDir determines the canonical directory for skill files.
// When --path is set it is used (resolved relative to cwd if not absolute).
// Otherwise the default is .kongctl/skills/ under cwd.
func resolveCanonicalDir(cwd string, opts installSkillsOptions) (string, error) {
	if opts.path != "" {
		trimmed := strings.TrimSpace(opts.path)
		if trimmed == "" {
			return "", fmt.Errorf("--path cannot be empty")
		}
		if filepath.IsAbs(trimmed) {
			return filepath.Clean(trimmed), nil
		}
		return filepath.Clean(filepath.Join(cwd, trimmed)), nil
	}
	return filepath.Clean(filepath.Join(cwd, localCanonicalSkillsPath)), nil
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

		destPath, err := resolveDestinationPath(canonicalDir, asset.RelPath)
		if err != nil {
			return skillsInstallResult{}, fmt.Errorf("resolve destination path for %q: %w", asset.RelPath, err)
		}
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

func resolveDestinationPath(canonicalDir, relPath string) (string, error) {
	cleanCanonical := filepath.Clean(canonicalDir)
	cleanRel := filepath.Clean(filepath.FromSlash(relPath))
	if cleanRel == "." || cleanRel == "" {
		return "", fmt.Errorf("relative path cannot be empty")
	}
	if filepath.IsAbs(cleanRel) {
		return "", fmt.Errorf("relative path must not be absolute")
	}

	destPath := filepath.Clean(filepath.Join(cleanCanonical, cleanRel))
	relativeToCanonical, err := filepath.Rel(cleanCanonical, destPath)
	if err != nil {
		return "", fmt.Errorf("resolve relative path: %w", err)
	}
	if relativeToCanonical == ".." || strings.HasPrefix(relativeToCanonical, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes canonical directory")
	}

	return destPath, nil
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

	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	text := string(normalized)
	if !strings.HasPrefix(text, opening) {
		return nil, nil, fmt.Errorf("skill file missing YAML frontmatter opening delimiter")
	}

	rest := text[len(opening):]
	before, after, ok := strings.Cut(rest, separator)
	if !ok {
		return nil, nil, fmt.Errorf("skill file missing YAML frontmatter closing delimiter")
	}

	frontmatter := []byte(before)
	body := []byte(after)

	return frontmatter, body, nil
}

func printSkillsInstallSummary(out io.Writer, cwd string, result skillsInstallResult) error {
	if out == nil {
		return nil
	}

	if result.DryRun {
		if _, err := fmt.Fprintln(out, "Dry run, no files will be written"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
	}

	if result.DryRun {
		if _, err := fmt.Fprintf(out, "Would install %d kongctl skills and %d coding agent symlinks:\n",
			len(result.SkillNames), len(result.Symlinks)); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(out, "Installed %d kongctl skills and %d coding agent symlinks:\n",
			len(result.SkillNames), len(result.Symlinks)); err != nil {
			return err
		}
	}

	for _, name := range result.SkillNames {
		skillDir := filepath.Join(result.CanonicalDir, name)
		if _, err := fmt.Fprintf(out, "  %s\n", relativizeOrKeep(cwd, skillDir)); err != nil {
			return err
		}
	}
	for _, sl := range result.Symlinks {
		if _, err := fmt.Fprintf(out, "  %s -> %s\n",
			relativizeOrKeep(cwd, sl.LinkPath), relativizeOrKeep(cwd, sl.TargetPath)); err != nil {
			return err
		}
	}

	if !result.DryRun {
		if _, err := fmt.Fprintln(out, "\nExample agent prompts:"); err != nil {
			return err
		}
		prompts := []string{
			`"Show me my Kong Konnect control planes"`,
			`"Help me setup declarative configuration for Kong Konnect with kongctl using my openapi spec"`,
		}
		for _, p := range prompts {
			if _, err := fmt.Fprintf(out, "  %s\n", p); err != nil {
				return err
			}
		}
	}

	return nil
}

// relativizeOrKeep returns a relative path from cwd to target when the
// result is a clean descent (no leading ".."). Otherwise it returns the
// absolute target path for clarity.
func relativizeOrKeep(cwd, target string) string {
	rel, err := filepath.Rel(cwd, target)
	if err != nil || strings.HasPrefix(rel, "..") {
		return target
	}
	return rel
}
