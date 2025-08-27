package common

import (
	"fmt"
	"log/slog"

	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/auth"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
)

const (
	BaseURLDefault  = "https://global.api.konghq.com"
	BaseURLFlagName = "base-url"

	AuthPathDefault  = "/v3/internal/oauth/device/authorize"
	AuthPathFlagName = "auth-path"

	TokenPathDefault  = "/v3/internal/oauth/device/token" // #nosec G101
	TokenPathFlagName = "token-path"                      // #nosec G101

	RefreshPathDefault  = "/kauth/api/v1/refresh"
	RefreshPathFlagName = "refresh-path"

	MachineClientIDDefault  = "344f59db-f401-4ce7-9407-00a0823fbacf"
	MachineClientIDFlagName = "machine-client-id"

	PATFlagName = "pat"

	RequestPageSizeFlagName = "page-size"
	DefaultRequestPageSize  = 10
)

var (
	PATConfigPath          = "konnect." + PATFlagName
	AuthTokenConfigPath    = "konnect.auth-token"    // #nosec G101
	RefreshTokenConfigPath = "konnect.refresh-token" // #nosec G101

	BaseURLConfigPath             = "konnect." + BaseURLFlagName
	AuthPathConfigPath            = "konnect." + AuthPathFlagName
	AuthMachineClientIDConfigPath = "konnect." + MachineClientIDFlagName
	TokenURLPathConfigPath        = "konnect." + TokenPathFlagName
	RefreshPathConfigPath         = "konnect." + RefreshPathFlagName

	MachineClientIDConfigPath = "konnect." + MachineClientIDFlagName
	RequestPageSizeConfigPath = "konnect." + RequestPageSizeFlagName
)

func GetAccessToken(cfg config.Hook, logger *slog.Logger) (string, error) {
	pat := cfg.GetString(PATConfigPath)
	if pat != "" {
		return pat, nil
	}

	refreshURL := cfg.GetString(BaseURLConfigPath) + cfg.GetString(RefreshPathConfigPath)
	tok, err := auth.LoadAccessToken(cfg, refreshURL, logger)
	if err != nil {
		return "", err
	}
	return tok.Token.AuthToken, nil
}

// This is the real implementation of the SDKAPIFactory,
// which creates a real Konnect SDK instance
func KonnectSDKFactory(cfg config.Hook, logger *slog.Logger) (helpers.SDKAPI, error) {
	token, e := GetAccessToken(cfg, logger)
	if e != nil {
		return nil, fmt.Errorf(
			`no access token available. Use "%s login konnect" to authenticate or provide a Konnect PAT using the --pat flag`,
			meta.CLIName,
		)
	}

	baseURL := cfg.GetString(BaseURLConfigPath)

	sdk, err := auth.GetAuthenticatedClient(baseURL, token, logger)
	if err != nil {
		return nil, err
	}

	return &helpers.KonnectSDK{
		SDK: sdk,
	}, nil
}

// GetSDKFactory returns the SDK factory to use, checking for test overrides
func GetSDKFactory() helpers.SDKAPIFactory {
	if helpers.DefaultSDKFactory != nil {
		return helpers.DefaultSDKFactory
	}
	return KonnectSDKFactory
}
