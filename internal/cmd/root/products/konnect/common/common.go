package common

import (
	"fmt"
	"log/slog"
	"regexp"
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

	RegionFlagName = "region"

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

	RegionConfigPath = "konnect." + RegionFlagName
)

var regionPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

// BuildBaseURLFromRegion converts a region identifier into the corresponding Konnect API host.
func BuildBaseURLFromRegion(region string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(region))
	if trimmed == "" {
		return "", fmt.Errorf("konnect region cannot be empty")
	}

	if trimmed == "global" {
		return GlobalBaseURL, nil
	}

	if !regionPattern.MatchString(trimmed) {
		return "", fmt.Errorf("invalid konnect region %q (expected lowercase letters, numbers, or hyphens)", region)
	}

	return fmt.Sprintf("https://%s.api.konghq.com", trimmed), nil
}

// ResolveBaseURL determines the effective Konnect base URL, honoring the precedence rules:
// 1) Explicit base-url flag/config
// 2) Region flag/config (converted to a URL)
// 3) Default US region
func ResolveBaseURL(cfg config.Hook) (string, error) {
	baseURL := strings.TrimSpace(cfg.GetString(BaseURLConfigPath))
	if baseURL != "" {
		return baseURL, nil
	}

	region := strings.TrimSpace(cfg.GetString(RegionConfigPath))
	var resolved string
	if region != "" {
		r, err := BuildBaseURLFromRegion(region)
		if err != nil {
			return "", err
		}
		resolved = r
	} else {
		resolved = BaseURLDefault
	}

	cfg.SetString(BaseURLConfigPath, resolved)
	return resolved, nil
}

func GetAccessToken(cfg config.Hook, logger *slog.Logger) (string, error) {
	pat := cfg.GetString(PATConfigPath)
	if pat != "" {
		return pat, nil
	}

	baseURL, err := ResolveBaseURL(cfg)
	if err != nil {
		return "", err
	}

	refreshURL := baseURL + cfg.GetString(RefreshPathConfigPath)
	tok, err := auth.LoadAccessToken(cfg, refreshURL, logger)
	if err != nil {
		// Provide helpful guidance on authentication options instead of exposing
		// internal implementation details like file paths
		profile := cfg.GetProfile()
		envVar := fmt.Sprintf("KONGCTL_%s_KONNECT_PAT", strings.ToUpper(profile))

		return "", fmt.Errorf(
			"authentication token not available. Use one of the following to authorize %s:\n"+
				"  - '%s login' to authenticate via the web\n"+
				"  - provide a token via the --%s flag\n"+
				"  - set the %s environment variable\n"+
				"  - configure a token value in the '%s.%s' path of your configuration file",
			meta.CLIName,
			meta.CLIName,
			PATFlagName,
			envVar,
			profile,
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

	baseURL, err := ResolveBaseURL(cfg)
	if err != nil {
		return nil, err
	}

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
