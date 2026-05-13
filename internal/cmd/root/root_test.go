package root

import (
	"strings"
	"testing"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/common"
	"github.com/spf13/cobra"
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
