package common

import (
	"fmt"
	"net/url"
	"strings"

	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/spf13/cobra"
)

// ResolveAPIBaseURL routes global API families to their global host while
// preserving explicit endpoint overrides and absolute request URLs.
func ResolveAPIBaseURL(command *cobra.Command, cfg config.Hook, endpoint string) (string, error) {
	baseURL, err := ResolveBaseURL(cfg)
	if err != nil {
		return "", err
	}
	if cmdcommon.CommandTreeFlagChanged(command, BaseURLFlagName) ||
		cfg.InConfig(BaseURLConfigPath) || profileConfigEnvSet(cfg.GetProfile(), BaseURLConfigPath) {
		return baseURL, nil
	}
	requestURL, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return "", fmt.Errorf("parse Konnect endpoint: %w", err)
	}
	if requestURL.IsAbs() || requestURL.Host != "" || !isGlobalAPIPath(requestURL.Path) {
		return baseURL, nil
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse Konnect base URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Port() != "" ||
		(parsed.Path != "" && parsed.Path != "/") {
		return baseURL, nil
	}
	for _, domain := range []string{konnectProductionDomain, konnectTechDomain} {
		host := strings.ToLower(parsed.Hostname())
		region, ok := strings.CutSuffix(host, ".api."+domain)
		if ok && regionPattern.MatchString(region) {
			return "https://global.api." + domain, nil
		}
	}
	return baseURL, nil
}

// These families are served globally by the Identity, Organizations, Audit Logs,
// and Regions APIs. Match path segments so similarly named regional resources
// (including portal users and teams) retain their regional host.
func isGlobalAPIPath(path string) bool {
	path = "/" + strings.Trim(path, "/")
	for _, prefix := range []string{
		"/v3/organizations", "/v3/users", "/v3/teams", "/v3/system-accounts",
		"/v3/roles", "/v3/identity-providers", "/v3/authenticate",
		"/v3/audit-logs", "/v3/available-regions",
	} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
