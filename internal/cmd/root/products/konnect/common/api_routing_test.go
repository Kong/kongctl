package common

import (
	"strings"
	"testing"

	"github.com/kong/kongctl/internal/config"
	utilviper "github.com/kong/kongctl/internal/util/viper"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestResolveAPIBaseURLWithProfiledConfig(t *testing.T) {
	for _, tc := range []struct {
		name         string
		fileConfig   string
		explicitFlag bool
		want         string
	}{
		{name: "default", want: GlobalBaseURL},
		{name: "explicit flag", explicitFlag: true, want: BaseURLDefault},
		{
			name: "explicit file", fileConfig: "default:\n  konnect:\n    base-url: https://eu.api.konghq.com\n",
			want: "https://eu.api.konghq.com",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mainv := utilviper.NewViper("nonexistent.yaml")
			if tc.fileConfig != "" {
				mainv.SetConfigType("yaml")
				require.NoError(t, mainv.ReadConfig(strings.NewReader(tc.fileConfig)))
			}
			cfg := config.BuildProfiledConfig("default", "nonexistent.yaml", mainv)
			c := &cobra.Command{Use: "api"}
			c.Flags().String(BaseURLFlagName, "", "")
			require.NoError(t, cfg.BindFlag(BaseURLConfigPath, c.Flags().Lookup(BaseURLFlagName)))
			if tc.explicitFlag {
				require.NoError(t, c.Flags().Set(BaseURLFlagName, BaseURLDefault))
			}
			got, err := ResolveAPIBaseURL(c, cfg, "/v3/organizations/org/personal-access-tokens")
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
			got, err = ResolveAPIBaseURL(c, cfg, "/v3/organizations/org/personal-access-tokens")
			require.NoError(t, err)
			require.Equal(t, tc.want, got, "cached base URL must retain routing behavior")
		})
	}
}

func TestResolveAPIBaseURL(t *testing.T) {
	for _, tc := range []struct {
		name, endpoint, base, region, want string
		explicit                           bool
	}{
		{name: "organization PATs", endpoint: "/v3/organizations/org/personal-access-tokens", want: GlobalBaseURL},
		{name: "PAT settings", endpoint: "/v3/organizations/org/personal-access-token-settings", want: GlobalBaseURL},
		{name: "user PATs with query", endpoint: "v3/users/user/access-tokens?page[size]=10", want: GlobalBaseURL},
		{name: "regional selection", endpoint: "/v3/teams", region: "eu", want: GlobalBaseURL},
		{name: "staging", endpoint: "/v3/system-accounts", base: TechBaseURLDefault, want: TechGlobalBaseURL},
		{name: "audit logs", endpoint: "/v3/audit-logs", want: GlobalBaseURL},
		{name: "regions", endpoint: "/v3/available-regions", want: GlobalBaseURL},
		{name: "roles", endpoint: "/v3/roles", want: GlobalBaseURL},
		{name: "regional APIs", endpoint: "/v3/apis", want: BaseURLDefault},
		{name: "portal users", endpoint: "/v3/portals/p/users", want: BaseURLDefault},
		{name: "segment boundary", endpoint: "/v3/users-extra", want: BaseURLDefault},
		{name: "custom host", endpoint: "/v3/users", base: "https://proxy.example.com", want: "https://proxy.example.com"},
		{name: "custom base path", endpoint: "/v3/users", base: BaseURLDefault + "/proxy", want: BaseURLDefault + "/proxy"},
		{name: "explicit regional host", endpoint: "/v3/users", base: BaseURLDefault, explicit: true, want: BaseURLDefault},
		{name: "absolute endpoint", endpoint: "https://eu.api.konghq.com/v3/users", want: BaseURLDefault},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _ := newTestConfig(map[string]string{BaseURLConfigPath: tc.base, RegionConfigPath: tc.region})
			c := &cobra.Command{Use: "api"}
			c.Flags().String(BaseURLFlagName, "", "")
			if tc.explicit {
				require.NoError(t, c.Flags().Set(BaseURLFlagName, tc.base))
			}
			got, err := ResolveAPIBaseURL(c, cfg, tc.endpoint)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestResolveAPIBaseURLPreservesEnvironmentOverride(t *testing.T) {
	t.Setenv("KONGCTL_DEFAULT_KONNECT_BASE_URL", BaseURLDefault)
	cfg, _ := newTestConfig(map[string]string{BaseURLConfigPath: BaseURLDefault})
	got, err := ResolveAPIBaseURL(&cobra.Command{}, cfg, "/v3/users/me")
	require.NoError(t, err)
	require.Equal(t, BaseURLDefault, got)
}
