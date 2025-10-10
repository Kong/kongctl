package konnect

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/auth"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

var (
	loginKonnectShort = i18n.T("root.products.konnect.loginKonnectShort", "Login to Konnect")
	loginKonnectLong  = i18n.T("root.products.konnect.loginKonnectLong",
		"Initiate a login to Konnect using the browser based machine code authorization flow.")
	loginKonnectExample = normalizers.Examples(
		i18n.T("root.products.konnect.loginKonnectExample",
			fmt.Sprintf(`
# Login to Konnect
%[1]s login konnect`, meta.CLIName)))

	httpClient *http.Client
)

type loginKonnectCmd struct {
	*cobra.Command
}

func (c *loginKonnectCmd) validate(_ cmd.Helper) error {
	return nil
}

func displayUserInstructions(resp auth.DeviceCodeResponse) {
	userResp := fmt.Sprintf("Logging your CLI into Kong Konnect with the browser...\n\n"+
		" To login, go to the following URL in your browser:\n\n"+
		"   %s\n\n"+
		" Or copy this one-time code: %s\n\n"+
		" And open your browser to %s\n\n"+
		" (Code expires in %d seconds)\n\n"+
		" Waiting for user to Login...",
		resp.VerificationURIComplete, resp.UserCode, resp.VerificationURI, resp.ExpiresIn)

	fmt.Println(userResp)
}

func (c *loginKonnectCmd) run(helper cmd.Helper) error {
	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	httpClient = &http.Client{Timeout: time.Second * 15}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	authBaseURL := cfg.GetString(common.AuthBaseURLConfigPath)
	if authBaseURL == "" {
		authBaseURL = common.AuthBaseURLDefault
	}
	authPath := cfg.GetString(common.AuthPathConfigPath)
	// Device authorization endpoints default to the global Konnect API host but can be overridden.
	authURL := authBaseURL + authPath

	pollPath := cfg.GetString(common.TokenURLPathConfigPath)
	pollURL := authBaseURL + pollPath

	clientID := cfg.GetString(common.MachineClientIDConfigPath)

	resp, err := auth.RequestDeviceCode(httpClient, authURL, clientID, logger)
	if err != nil {
		return err
	}

	if resp.UserCode == "" || resp.VerificationURI == "" || resp.VerificationURIComplete == "" ||
		resp.Interval == 0 || resp.ExpiresIn == 0 {
		return fmt.Errorf("invalid device code request response from Konnect: %v", resp)
	}

	displayUserInstructions(resp)

	expiresAt := time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
	// poll for token while the user completes authorizing the request
	for {
		var err error
		var pollResp *auth.AccessToken
		select {
		case <-helper.GetContext().Done():
			c.SilenceUsage = true
			c.SilenceErrors = true
			return helper.GetContext().Err()
		case <-time.After(time.Duration(resp.Interval) * time.Second):
			pollResp, err = auth.PollForToken(
				helper.GetContext(), httpClient, pollURL, clientID, resp.DeviceCode, logger)
		}
		var dagError *auth.DAGError
		if errors.As(err, &dagError) && dagError.ErrorCode == auth.AuthorizationPendingErrorCode {
			continue
		}
		if err != nil {
			return err
		}

		if time.Now().After(expiresAt) {
			return fmt.Errorf("%s: %w", "device authorization request has expired", err)
		}

		if pollResp != nil && pollResp.Token.AuthToken != "" {
			fmt.Println("\nUser successfully authorized")
			err := auth.SaveAccessToken(cfg, pollResp)
			if err != nil {
				return fmt.Errorf("%s: %w", "failed to save tokens", err)
			}
			break
		}
	}

	return nil
}

func (c *loginKonnectCmd) preRunE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	f := c.Flags().Lookup(common.AuthPathFlagName)
	err = cfg.BindFlag(common.AuthPathConfigPath, f)
	if err != nil {
		return err
	}

	f = c.Flags().Lookup(common.AuthBaseURLFlagName)
	err = cfg.BindFlag(common.AuthBaseURLConfigPath, f)
	if err != nil {
		return err
	}

	f = c.Flags().Lookup(common.RefreshPathFlagName)
	err = cfg.BindFlag(common.RefreshPathConfigPath, f)
	if err != nil {
		return err
	}

	f = c.Flags().Lookup(common.TokenPathFlagName)
	err = cfg.BindFlag(common.TokenURLPathConfigPath, f)
	if err != nil {
		return err
	}

	f = c.Flags().Lookup(common.MachineClientIDFlagName)
	err = cfg.BindFlag(common.MachineClientIDConfigPath, f)
	if err != nil {
		return err
	}

	return nil
}

func (c *loginKonnectCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if e := c.validate(helper); e != nil {
		return e
	}

	return c.run(helper)
}

func newLoginKonnectCmd(verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *loginKonnectCmd {
	rv := loginKonnectCmd{
		Command: baseCmd,
	}

	rv.Short = loginKonnectShort
	rv.Long = loginKonnectLong
	rv.Example = loginKonnectExample

	addParentFlags(verb, rv.Command)

	rv.Flags().String(common.AuthPathFlagName, common.AuthPathDefault,
		fmt.Sprintf(`URL path used to initiate Konnect Authorization.
- Config path: [ %s ]
-`, // (default ...)
			common.AuthPathConfigPath))

	rv.Flags().String(common.AuthBaseURLFlagName, common.AuthBaseURLDefault,
		fmt.Sprintf(`Base URL used for Konnect Authorization requests.
- Config path: [ %s ]
-`, // (default ...)
			common.AuthBaseURLConfigPath))

	rv.Flags().String(common.RefreshPathFlagName, common.RefreshPathDefault,
		fmt.Sprintf(`URL path used to refresh the Konnect auth token.
- Config path: [ %s ]
-`, // (default ...)
			common.RefreshPathConfigPath))

	rv.Flags().String(common.MachineClientIDFlagName, common.MachineClientIDDefault,
		fmt.Sprintf(`Machine Client ID used to identify the application for Konnect Authorization.
- Config path: [ %s ]
-`, // (default ...)
			common.MachineClientIDConfigPath))
	util.CheckError(rv.Flags().MarkHidden(common.MachineClientIDFlagName))

	rv.Flags().String(common.TokenPathFlagName, common.TokenPathDefault,
		fmt.Sprintf(`URL path used to poll for the Konnect Authorization response token.
- Config path: [ %s ]
-`, // (default ...)
			common.TokenURLPathConfigPath))

	rv.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		e := parentPreRun(c, args)
		if e != nil {
			return e
		}
		return rv.preRunE(c, args)
	}
	rv.RunE = rv.runE

	return &rv
}
