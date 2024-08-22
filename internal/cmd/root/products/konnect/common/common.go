package common

import (
	"log/slog"

	"github.com/kong/kong-cli/internal/config"
	"github.com/kong/kong-cli/internal/konnect/auth"
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

func GetAccessToken(cfg config.Hook, logger *slog.Logger) (*auth.AccessToken, error) {
	refreshURL := cfg.GetString(BaseURLConfigPath) + cfg.GetString(RefreshPathConfigPath)
	return auth.LoadAccessToken(cfg.GetProfile(), refreshURL, logger)
}
