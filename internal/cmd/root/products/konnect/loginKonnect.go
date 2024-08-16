package konnect

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/kong/kong-cli/internal/cmd"
	"github.com/kong/kong-cli/internal/cmd/root/products/konnect/common"
	"github.com/kong/kong-cli/internal/konnect/auth"
	"github.com/kong/kong-cli/internal/meta"
	"github.com/kong/kong-cli/internal/util/i18n"
	"github.com/kong/kong-cli/internal/util/normalizers"
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
	userResp := fmt.Sprintf("Authenticating with Konnect in the browser...\n\n\n"+
		" Either copy this one-time code: %s\n\n"+
		" (Expires in %d seconds) \n\n And go to %s\n"+
		" Or Click or go to %s \n\n\n waiting for user to Authenticate......",
		resp.UserCode, resp.ExpiresIn, resp.VerificationURI, resp.VerificationURIComplete)

	fmt.Println(userResp)
}

func (c *loginKonnectCmd) run(helper cmd.Helper) error {
	httpClient = &http.Client{Timeout: time.Second * 15}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	baseURL := cfg.GetString(common.BaseURLConfigPath)
	authPath := cfg.GetString(common.AuthPathConfigPath)
	authURL := baseURL + authPath

	pollPath := cfg.GetString(common.TokenURLPathConfigPath)
	pollURL := baseURL + pollPath

	clientID := cfg.GetString(common.MachineClientIDConfigPath)

	resp, err := auth.RequestDeviceCode(httpClient, authURL, clientID)
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
		time.Sleep(time.Duration(resp.Interval) * time.Second)
		pollResp, err := auth.PollForToken(httpClient, pollURL, clientID, resp.DeviceCode)
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
			err := auth.SaveAccessTokenToDisk(
				auth.BuildDefaultCredentialFilePath(cfg.GetProfile()),
				pollResp)
			if err != nil {
				return fmt.Errorf("%s: %w", "failed to save tokens", err)
			}
			break
		}
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

func newLoginKonnectCmd(baseCmd *cobra.Command) *loginKonnectCmd {
	rv := loginKonnectCmd{
		Command: baseCmd,
	}

	baseCmd.Short = loginKonnectShort
	baseCmd.Long = loginKonnectLong
	baseCmd.Example = loginKonnectExample
	baseCmd.RunE = rv.runE

	return &rv
}
