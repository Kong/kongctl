package inventory

import (
	"slices"
	"testing"

	"github.com/kong/kongctl/internal/maturity"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectReturnsCanonicalSortedRecords(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	child := &cobra.Command{Use: "child <mode>", Aliases: []string{"alias"}}
	child.Flags().String("format", "", "format")
	root.AddCommand(child)
	require.NoError(t, maturity.AnnotateCommand(child, maturity.Metadata{Level: maturity.LevelBeta}))
	require.NoError(t, maturity.AnnotateFlagValue(
		child, "format", "preview", maturity.Metadata{Level: maturity.LevelTechPreview},
	))
	require.NoError(t, maturity.AnnotateArgument(child, "mode", maturity.Metadata{Level: maturity.LevelTechPreview}))

	records, err := Collect(root)
	require.NoError(t, err)
	require.NotEmpty(t, records)
	assert.True(t, slices.IsSortedFunc(records, func(a, b Record) int { return compareRecords(a, b) }))

	commandRecord := findRecord(records, maturity.KindCommand, "root child", "", "")
	require.NotNil(t, commandRecord)
	assert.Equal(t, maturity.LevelBeta, commandRecord.Effective.Level)
	assert.Equal(t, "root child", commandRecord.Source.Path)
	assert.Nil(t, findRecord(records, maturity.KindCommand, "root alias", "", ""))

	valueRecord := findRecord(records, maturity.KindFlagValue, "root child", "format", "preview")
	require.NotNil(t, valueRecord)
	assert.Equal(t, maturity.LevelTechPreview, valueRecord.Effective.Level)

	resourceRecord := findRecord(records, maturity.KindResource, "portal", "", "")
	require.NotNil(t, resourceRecord)
	assert.Equal(t, maturity.LevelGA, resourceRecord.Effective.Level)
	operationRecord := findRecord(records, maturity.KindOperation, "portal", "delete", "")
	require.NotNil(t, operationRecord)
	assert.Equal(t, maturity.LevelGA, operationRecord.Effective.Level)
}

func findRecord(records []Record, kind maturity.Kind, path, name, value string) *Record {
	for i := range records {
		record := &records[i]
		if record.Kind == kind && record.Path == path && record.Name == name && record.Value == value {
			return record
		}
	}
	return nil
}
