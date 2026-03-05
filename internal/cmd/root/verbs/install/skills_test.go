package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
)

func TestInjectSkillMetadataVersion(t *testing.T) {
	input := []byte(`---
name: sample-skill
description: test
---

# body
`)

	out, err := injectSkillMetadataVersion(input, "v1.2.3")
	require.NoError(t, err)

	frontmatter := parseFrontmatterMap(t, out)
	metadata, ok := frontmatter["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "v1.2.3", metadata["version"])
}

func TestInstallBundledSkillsWritesVersionedSkillsAndManifest(t *testing.T) {
	canonicalDir := filepath.Join(t.TempDir(), ".kongctl", "skills")
	now := time.Date(2026, 3, 4, 21, 30, 0, 0, time.UTC)

	result, err := installBundledSkills(canonicalDir, "v9.9.9", false, now)
	require.NoError(t, err)

	assert.Equal(t, canonicalDir, result.CanonicalDir)
	assert.Equal(t, "v9.9.9", result.CLIVersion)
	assert.ElementsMatch(t, []string{"kongctl-declarative", "kongctl-query"}, result.SkillNames)

	for _, skillName := range result.SkillNames {
		skillPath := filepath.Join(canonicalDir, skillName, "SKILL.md")
		content, err := os.ReadFile(skillPath)
		require.NoError(t, err, "read %s", skillPath)

		frontmatter := parseFrontmatterMap(t, content)
		metadata, ok := frontmatter["metadata"].(map[string]any)
		require.True(t, ok, "metadata should exist in %s", skillPath)
		assert.Equal(t, "v9.9.9", metadata["version"])
	}

	manifestData, err := os.ReadFile(result.ManifestPath)
	require.NoError(t, err)

	var manifest skillsInstallManifest
	require.NoError(t, json.Unmarshal(manifestData, &manifest))
	assert.Equal(t, "v9.9.9", manifest.CLIVersion)
	assert.Equal(t, now.Format(time.RFC3339), manifest.InstalledAt)
	assert.ElementsMatch(t, []string{"kongctl-declarative", "kongctl-query"}, manifest.Skills)
}

func TestInstallBundledSkillsDryRunDoesNotWriteFiles(t *testing.T) {
	canonicalDir := filepath.Join(t.TempDir(), ".kongctl", "skills")
	result, err := installBundledSkills(canonicalDir, "v0.0.1", true, time.Now().UTC())
	require.NoError(t, err)

	assert.True(t, result.DryRun)
	assert.Greater(t, len(result.WrittenFiles), 0)

	_, err = os.Stat(canonicalDir)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestResolveCanonicalDirDefault(t *testing.T) {
	cwd := t.TempDir()
	dir, err := resolveCanonicalDir(cwd, installSkillsOptions{})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(cwd, localCanonicalSkillsPath), dir)
}

func TestResolveCanonicalDirCustomPath(t *testing.T) {
	cwd := t.TempDir()
	dir, err := resolveCanonicalDir(cwd, installSkillsOptions{path: "custom/loc"})
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(cwd, "custom", "loc"), dir)
}

func TestResolveCanonicalDirCustomAbsPath(t *testing.T) {
	cwd := t.TempDir()
	absPath := filepath.Join(t.TempDir(), "abs-skills")
	dir, err := resolveCanonicalDir(cwd, installSkillsOptions{path: absPath})
	require.NoError(t, err)
	assert.Equal(t, absPath, dir)
}

func TestPlanSymlinksSkipsCanonicalDir(t *testing.T) {
	cwd := t.TempDir()
	canonicalDir := filepath.Join(cwd, ".agent", "skills")
	skills := []string{"kongctl-query", "kongctl-declarative"}

	planned := planSymlinks(cwd, canonicalDir, skills)

	for _, link := range planned {
		assert.NotContains(t, link.Tools, "codex",
			"should skip .agent/skills/ when it matches canonical dir")
	}
	var claudeLinks []plannedSymlink
	for _, link := range planned {
		if link.Tools == "claude code" {
			claudeLinks = append(claudeLinks, link)
		}
	}
	assert.Len(t, claudeLinks, len(skills))
}

func TestPlanSymlinksCreatesLinksForAllToolDirs(t *testing.T) {
	cwd := t.TempDir()
	canonicalDir := filepath.Join(cwd, ".kongctl", "skills")
	skills := []string{"kongctl-query"}

	planned := planSymlinks(cwd, canonicalDir, skills)

	assert.Len(t, planned, len(toolIntegrations))
	for _, link := range planned {
		assert.Equal(t, "kongctl-query", link.SkillName)
		assert.Equal(t, filepath.Join(canonicalDir, "kongctl-query"), link.TargetPath)
	}
}

func TestCheckSymlinkConflictsNoConflictWhenEmpty(t *testing.T) {
	cwd := t.TempDir()
	canonicalDir := filepath.Join(cwd, ".kongctl", "skills")
	planned := planSymlinks(cwd, canonicalDir, []string{"kongctl-query"})

	conflicts := checkSymlinkConflicts(planned)
	assert.Empty(t, conflicts)
}

func TestCheckSymlinkConflictsDetectsExistingDirectory(t *testing.T) {
	cwd := t.TempDir()
	canonicalDir := filepath.Join(cwd, ".kongctl", "skills")

	conflictDir := filepath.Join(cwd, ".agent", "skills", "kongctl-query")
	require.NoError(t, os.MkdirAll(conflictDir, 0o755))

	planned := planSymlinks(cwd, canonicalDir, []string{"kongctl-query"})
	conflicts := checkSymlinkConflicts(planned)

	assert.NotEmpty(t, conflicts)
	found := false
	for _, c := range conflicts {
		if c.linkPath == conflictDir {
			found = true
		}
	}
	assert.True(t, found, "expected conflict for %s", conflictDir)
}

func TestCheckSymlinkConflictsAllowsReinstall(t *testing.T) {
	cwd := t.TempDir()
	canonicalDir := filepath.Join(cwd, ".kongctl", "skills")

	require.NoError(t, os.MkdirAll(filepath.Join(canonicalDir, "kongctl-query"), 0o755))

	planned := planSymlinks(cwd, canonicalDir, []string{"kongctl-query"})

	for _, link := range planned {
		require.NoError(t, os.MkdirAll(filepath.Dir(link.LinkPath), 0o755))
		require.NoError(t, os.Symlink(link.RelTarget, link.LinkPath))
	}

	conflicts := checkSymlinkConflicts(planned)
	assert.Empty(t, conflicts, "re-install should not produce conflicts")
}

func TestCreateToolSymlinksCreatesValidLinks(t *testing.T) {
	cwd := t.TempDir()
	canonicalDir := filepath.Join(cwd, ".kongctl", "skills")

	skillDir := filepath.Join(canonicalDir, "kongctl-query")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillDir, "SKILL.md"), []byte("test"), 0o600))

	planned := planSymlinks(cwd, canonicalDir, []string{"kongctl-query"})
	require.NoError(t, createToolSymlinks(planned, false))

	for _, link := range planned {
		info, err := os.Lstat(link.LinkPath)
		require.NoError(t, err)
		assert.NotZero(t, info.Mode()&os.ModeSymlink, "expected symlink at %s", link.LinkPath)

		content, err := os.ReadFile(filepath.Join(link.LinkPath, "SKILL.md"))
		require.NoError(t, err)
		assert.Equal(t, "test", string(content))
	}
}

func TestCreateToolSymlinksDryRunNoOp(t *testing.T) {
	cwd := t.TempDir()
	canonicalDir := filepath.Join(cwd, ".kongctl", "skills")
	planned := planSymlinks(cwd, canonicalDir, []string{"kongctl-query"})

	require.NoError(t, createToolSymlinks(planned, true))

	for _, link := range planned {
		_, err := os.Lstat(link.LinkPath)
		assert.ErrorIs(t, err, os.ErrNotExist)
	}
}

func TestIsMatchingSymlinkRelativeAndAbsolute(t *testing.T) {
	linkPath := "/project/.agent/skills/my-skill"
	target := "/project/.kongctl/skills/my-skill"

	assert.True(t, isMatchingSymlink(linkPath, target, target),
		"exact absolute match")
	assert.True(t, isMatchingSymlink(linkPath, "../../.kongctl/skills/my-skill", target),
		"relative match")
	assert.False(t, isMatchingSymlink(linkPath, "/other/path/my-skill", target),
		"different absolute path")
}

func TestRelativizeOrKeep(t *testing.T) {
	assert.Equal(t, ".kongctl/skills",
		relativizeOrKeep("/project", "/project/.kongctl/skills"))
	assert.Equal(t, "/home/user/.config/kongctl/skills",
		relativizeOrKeep("/project", "/home/user/.config/kongctl/skills"))
}

func parseFrontmatterMap(t *testing.T, content []byte) map[string]any {
	t.Helper()

	frontmatter, _, err := splitFrontmatter(content)
	require.NoError(t, err)

	result := map[string]any{}
	require.NoError(t, yaml.Unmarshal(frontmatter, &result))

	return result
}
