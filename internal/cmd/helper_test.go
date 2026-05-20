package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestCommandHelperGetOutputFormatUsesCommandSpecificValidation(t *testing.T) {
	cfg := config.BuildProfiledConfig("default", "", viper.New())
	cfg.SetString(common.OutputConfigPath, common.HELM.String())

	cmd := &cobra.Command{Use: "leaf"}
	cmd.SetContext(context.WithValue(context.Background(), config.ConfigKey, cfg))

	helper := CommandHelper{Cmd: cmd}
	_, err := helper.GetOutputFormat()
	if err == nil {
		t.Fatal("expected helm to be rejected without command opt-in")
	}
	if !strings.Contains(err.Error(), "must be one of [json yaml text]") {
		t.Fatalf("expected command-specific allowed formats, got: %v", err)
	}
	if strings.Contains(err.Error(), "[json yaml text helm]") {
		t.Fatalf("expected error not to advertise helm without command opt-in, got: %v", err)
	}

	common.AllowExtraOutputFormats(cmd, common.HELM.String())
	outType, err := helper.GetOutputFormat()
	if err != nil {
		t.Fatalf("expected helm to be allowed with command opt-in: %v", err)
	}
	if outType != common.HELM {
		t.Fatalf("expected HELM output format, got %s", outType.String())
	}
}
