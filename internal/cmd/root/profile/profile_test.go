package profile

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/kong/kongctl/internal/build"
	"github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/konnect/helpers"
	profilepkg "github.com/kong/kongctl/internal/profile"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

type profileTestManager struct {
	profiles []string
	data     map[string]map[string]any
}

func (m profileTestManager) GetProfiles() []string {
	return append([]string{}, m.profiles...)
}

func (m profileTestManager) GetProfile(name string) (map[string]any, error) {
	return m.data[name], nil
}

func (m profileTestManager) CreateProfile(_ string) error {
	return nil
}

func (m profileTestManager) DeleteProfile(_ string) error {
	return nil
}

type profileTestHelper struct {
	cmd     *cobra.Command
	args    []string
	verb    verbs.VerbValue
	streams *iostreams.IOStreams
	cfg     config.Hook
}

func (h profileTestHelper) GetCmd() *cobra.Command {
	return h.cmd
}

func (h profileTestHelper) GetArgs() []string {
	return h.args
}

func (h profileTestHelper) GetVerb() (verbs.VerbValue, error) {
	return h.verb, nil
}

func (h profileTestHelper) GetProduct() (products.ProductValue, error) {
	return "", nil
}

func (h profileTestHelper) GetStreams() *iostreams.IOStreams {
	return h.streams
}

func (h profileTestHelper) GetConfig() (config.Hook, error) {
	return h.cfg, nil
}

func (h profileTestHelper) GetOutputFormat() (common.OutputFormat, error) {
	return common.JSON, nil
}

func (h profileTestHelper) GetLogger() (*slog.Logger, error) {
	return slog.Default(), nil
}

func (h profileTestHelper) GetBuildInfo() (*build.Info, error) {
	return nil, nil
}

func (h profileTestHelper) GetContext() context.Context {
	return context.Background()
}

func (h profileTestHelper) GetKonnectSDK(config.Hook, *slog.Logger) (helpers.SDKAPI, error) {
	return nil, nil
}

func TestNewProfileCmdDescribesKongctlProfiles(t *testing.T) {
	cmd := NewProfileCmd()

	require.Equal(t, "profile [profile-name]", cmd.Use)
	require.Equal(t, "Manage kongctl profiles", cmd.Short)
}

func TestRunGetListsProfiles(t *testing.T) {
	output := runGetForTest(t, nil, verbs.Get, profileTestManager{
		profiles: []string{"team-b", "default", "team-a"},
	})

	var got []string
	require.NoError(t, json.Unmarshal([]byte(output), &got))
	require.Equal(t, []string{"default", "team-a", "team-b"}, got)
}

func TestRunGetShowsNamedProfileConfiguration(t *testing.T) {
	output := runGetForTest(t, []string{"team-a"}, verbs.Get, profileTestManager{
		profiles: []string{"default", "team-a"},
		data: map[string]map[string]any{
			"team-a": {
				"output": "json",
				"konnect": map[string]any{
					"base_url": "https://us.api.konghq.com",
				},
			},
		},
	})

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &got))
	require.Equal(t, "json", got["output"])
	require.Equal(t, "https://us.api.konghq.com", got["konnect"].(map[string]any)["base_url"])
}

func TestRunListProfilesMatchesGetProfiles(t *testing.T) {
	output := runForTest(t, nil, verbs.List, profileTestManager{
		profiles: []string{"team-b", "default", "team-a"},
	})

	var got []string
	require.NoError(t, json.Unmarshal([]byte(output), &got))
	require.Equal(t, []string{"default", "team-a", "team-b"}, got)
}

func TestRunGetReturnsErrorForUnknownProfile(t *testing.T) {
	var out bytes.Buffer
	helper := newProfileTestHelper([]string{"missing"}, verbs.Get, &out)
	oldProfileManager := profileManager
	profileManager = profileTestManager{profiles: []string{"default"}}
	t.Cleanup(func() {
		profileManager = oldProfileManager
	})

	err := runGet(helper)

	require.ErrorContains(t, err, `profile "missing" not found`)
}

func runGetForTest(t *testing.T, args []string, verb verbs.VerbValue, manager profilepkg.Manager) string {
	t.Helper()

	var out bytes.Buffer
	helper := newProfileTestHelper(args, verb, &out)
	oldProfileManager := profileManager
	profileManager = manager
	t.Cleanup(func() {
		profileManager = oldProfileManager
	})

	require.NoError(t, runGet(helper))
	return out.String()
}

func runForTest(t *testing.T, args []string, verb verbs.VerbValue, manager profilepkg.Manager) string {
	t.Helper()

	var out bytes.Buffer
	helper := newProfileTestHelper(args, verb, &out)
	oldProfileManager := profileManager
	profileManager = manager
	t.Cleanup(func() {
		profileManager = oldProfileManager
	})

	require.NoError(t, run(helper))
	return out.String()
}

func newProfileTestHelper(args []string, verb verbs.VerbValue, out *bytes.Buffer) profileTestHelper {
	return profileTestHelper{
		cmd:  &cobra.Command{Use: "profile"},
		args: args,
		verb: verb,
		streams: &iostreams.IOStreams{
			In:     &bytes.Buffer{},
			Out:    out,
			ErrOut: &bytes.Buffer{},
		},
		cfg: config.BuildProfiledConfig(profilepkg.DefaultProfile, "", viper.New()),
	}
}
