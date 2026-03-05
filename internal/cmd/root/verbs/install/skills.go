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
	defaultSkillsInstallPath = ".agent/skills"
	skillsManifestFileName   = ".kongctl-skills-manifest.json"
)

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
	TargetDir    string
	CLIVersion   string
	SkillNames   []string
	WrittenFiles []string
	ManifestPath string
	DryRun       bool
}

func newInstallSkillsCmd() *cobra.Command {
	opts := &installSkillsOptions{
		path: defaultSkillsInstallPath,
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
		defaultSkillsInstallPath,
		"Destination directory for installed skills.",
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

	targetDir, err := resolveInstallTargetDir(cwd, opts.path)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}

	result, err := installBundledSkills(targetDir, version, opts.dryRun, time.Now().UTC())
	if err != nil {
		return cmdpkg.PrepareExecutionErrorWithHelper(helper, "failed to install skills", err)
	}

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

func installBundledSkills(
	targetDir string,
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
		TargetDir:  targetDir,
		CLIVersion: cliVersion,
		SkillNames: skillNames,
		DryRun:     dryRun,
	}

	if !dryRun {
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return skillsInstallResult{}, fmt.Errorf("create target directory: %w", err)
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

		destPath := filepath.Join(targetDir, filepath.FromSlash(asset.RelPath))
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

	manifestPath := filepath.Join(targetDir, skillsManifestFileName)
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
	sepIdx := strings.Index(rest, separator)
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

	if _, err := fmt.Fprintf(
		out,
		"%s %d kongctl skill(s) to %s\n",
		action,
		len(result.SkillNames),
		result.TargetDir,
	); err != nil {
		return err
	}

	for _, skillName := range result.SkillNames {
		if _, err := fmt.Fprintf(out, "- %s\n", skillName); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(out, "Skill metadata.version: %s\n", result.CLIVersion); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Install manifest: %s\n", result.ManifestPath); err != nil {
		return err
	}

	if result.DryRun {
		if _, err := fmt.Fprintln(out, "\nDry run only. No files were written."); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(out, "\nClaude Code setup (optional):"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "  mkdir -p .claude/skills"); err != nil {
		return err
	}
	for _, skillName := range result.SkillNames {
		relativeTarget := claudeSymlinkTarget(cwd, result.TargetDir, skillName)
		if _, err := fmt.Fprintf(
			out,
			"  ln -s %s .claude/skills/%s\n",
			relativeTarget,
			skillName,
		); err != nil {
			return err
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

func claudeSymlinkTarget(cwd, targetDir, skillName string) string {
	fromDir := filepath.Join(cwd, ".claude", "skills")
	target := filepath.Join(targetDir, skillName)

	relative, err := filepath.Rel(fromDir, target)
	if err != nil {
		return filepath.ToSlash(target)
	}

	return filepath.ToSlash(relative)
}
