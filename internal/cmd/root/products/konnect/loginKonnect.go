package konnect

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/konnect/auth"
	"github.com/kong/kongctl/internal/meta"
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

	baseURL := cfg.GetString(common.BaseURLConfigPath)
	authPath := cfg.GetString(common.AuthPathConfigPath)
	authURL := baseURL + authPath

	pollPath := cfg.GetString(common.TokenURLPathConfigPath)
	pollURL := baseURL + pollPath

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
