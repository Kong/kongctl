package common

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/auth"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
)

const (
	GlobalBaseURL   = "https://global.api.konghq.com"
	BaseURLDefault  = "https://us.api.konghq.com"
	BaseURLFlagName = "base-url"

	AuthBaseURLDefault  = GlobalBaseURL
	AuthBaseURLFlagName = "base-auth-url"

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
	AuthBaseURLConfigPath         = "konnect." + AuthBaseURLFlagName
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
		// Provide helpful guidance on authentication options instead of exposing
		// internal implementation details like file paths
		profile := cfg.GetProfile()
		envVar := fmt.Sprintf("KONGCTL_%s_KONNECT_PAT", strings.ToUpper(profile))

		return "", fmt.Errorf(
			"authentication token not available. Use '%s login' to authenticate, "+
				"provide a token via the --%s flag, set the %s environment variable, "+
				"or configure '%s' in your config file",
			meta.CLIName,
			PATFlagName,
			envVar,
			PATConfigPath,
		)
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
