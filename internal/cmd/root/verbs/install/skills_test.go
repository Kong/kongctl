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
	targetDir := filepath.Join(t.TempDir(), ".agent", "skills")
	now := time.Date(2026, 3, 4, 21, 30, 0, 0, time.UTC)

	result, err := installBundledSkills(targetDir, "v9.9.9", false, now)
	require.NoError(t, err)

	assert.Equal(t, targetDir, result.TargetDir)
	assert.Equal(t, "v9.9.9", result.CLIVersion)
	assert.ElementsMatch(t, []string{"kongctl-declarative", "kongctl-query"}, result.SkillNames)

	for _, skillName := range result.SkillNames {
		skillPath := filepath.Join(targetDir, skillName, "SKILL.md")
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
	targetDir := filepath.Join(t.TempDir(), ".agent", "skills")
	result, err := installBundledSkills(targetDir, "v0.0.1", true, time.Now().UTC())
	require.NoError(t, err)

	assert.True(t, result.DryRun)
	assert.Greater(t, len(result.WrittenFiles), 0)

	_, err = os.Stat(targetDir)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func parseFrontmatterMap(t *testing.T, content []byte) map[string]any {
	t.Helper()

	frontmatter, _, err := splitFrontmatter(content)
	require.NoError(t, err)

	result := map[string]any{}
	require.NoError(t, yaml.Unmarshal(frontmatter, &result))

	return result
}
