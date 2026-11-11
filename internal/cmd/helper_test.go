package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestTryConvertErrorToAttrs(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		want map[string]any
	}{
		{
			name: "SDK error message retains structured fields",
			err:  &sdkerrors.KonnectCPLegacyBadRequestError{Code: new(int64(3)), Message: new("invalid field")},
			want: map[string]any{"code": float64(3), "message": "invalid field"},
		},
		{
			name: "wrapped SDK error retains structured fields",
			err: fmt.Errorf("request failed: %w", &sdkerrors.KonnectCPLegacyBadRequestError{
				Code: new(int64(3)), Message: new("invalid field"),
			}),
			want: map[string]any{"code": float64(3), "message": "invalid field"},
		},
		{
			name: "JSON error text",
			err:  errors.New(`{"message":"invalid field","code":3}`),
			want: map[string]any{"code": float64(3), "message": "invalid field"},
		},
		{name: "plain error text", err: errors.New("invalid field"), want: map[string]any{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			attrs := TryConvertErrorToAttrs(tt.err)
			got := make(map[string]any)
			for i := 0; i < len(attrs); i += 2 {
				got[attrs[i].(string)] = attrs[i+1]
			}
			require.Equal(t, tt.want, got)
		})
	}
}

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
