package konnect

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/konnect/auth"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/telemetry"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/mattn/go-isatty"
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

	loginInputIsTerminal = isTerminalReader
)

type loginKonnectCmd struct {
	*cobra.Command
}

// resolveAuthURLs returns the fully constructed auth and token poll URLs from cfg.
func resolveAuthURLs(cfg config.Hook) (authURL, pollURL string) {
	authBaseURL := cfg.GetString(common.AuthBaseURLConfigPath)
	if authBaseURL == "" {
		authBaseURL = common.AuthBaseURLDefault
	}
	return authBaseURL + cfg.GetString(common.AuthPathConfigPath),
		authBaseURL + cfg.GetString(common.TokenURLPathConfigPath)
}

func (c *loginKonnectCmd) validate(helper cmd.Helper) error {
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	authURL, pollURL := resolveAuthURLs(cfg)
	if err := auth.ValidateKonnectURL(authURL); err != nil {
		return cmd.PrepareExecutionErrorWithHelper(helper, "invalid auth URL", err)
	}
	if err := auth.ValidateKonnectURL(pollURL); err != nil {
		return cmd.PrepareExecutionErrorWithHelper(helper, "invalid token poll URL", err)
	}

	return nil
}

func displayUserInstructions(w io.Writer, resp auth.DeviceCodeResponse) {
	userResp := fmt.Sprintf("Logging your CLI into Kong Konnect with the browser...\n\n"+
		" To login, go to the following URL in your browser:\n\n"+
		"   %s\n\n"+
		" Or copy this one-time code: %s\n\n"+
		" And open your browser to %s\n\n"+
		" (Code expires in %d seconds)\n\n"+
		" Waiting for user to Login...",
		resp.VerificationURIComplete, resp.UserCode, resp.VerificationURI, resp.ExpiresIn)

	fmt.Fprintln(w, userResp)
}

func handleTelemetryPreference(
	ctx context.Context,
	streams *iostreams.IOStreams,
	cfg config.Hook,
	rec *telemetry.Recorder,
) error {
	if streams == nil || cfg == nil || rec == nil || !rec.Enabled() || telemetry.PreferenceFileExists(cfg) {
		return nil
	}

	if !loginInputIsTerminal(streams.In) {
		writeTelemetryDisclosure(streams.ErrOut, false)
		return nil
	}

	writeTelemetryDisclosure(streams.Out, true)
	enabled, ok, err := promptTelemetryPreference(ctx, streams.In, streams.Out)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if !enabled {
		_ = rec.Disable(context.Background())
	}
	if err := telemetry.WritePreference(cfg, enabled); err != nil {
		fmt.Fprintf(streams.ErrOut, "warning: failed to save telemetry preference: %v\n", err)
	}
	return nil
}

func writeTelemetryDisclosure(w io.Writer, prompt bool) {
	fmt.Fprint(w, `kongctl collects limited usage data to help Kong understand CLI usage.

Collected:
  - kongctl version
  - operating system and architecture
  - command path, such as "login" or "get apis"

Not collected:
  - command arguments or flag values
  - resource names or IDs
  - auth tokens, request bodies, or response bodies
  - config file contents, file paths, hostnames, usernames, or email addresses

Telemetry can be disabled at any time with:
  kongctl --no-telemetry <command>
  KONGCTL_NO_TELEMETRY=true kongctl <command>
  DO_NOT_TRACK=1 kongctl <command>

`)
	if prompt {
		fmt.Fprint(w, "Allow kongctl to collect usage data on this device? [Y/n]: ")
	}
}

func promptTelemetryPreference(ctx context.Context, in io.Reader, out io.Writer) (bool, bool, error) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	return readTelemetryPreferenceAnswer(ctx, in, out, sigCh)
}

func readTelemetryPreferenceAnswer(
	ctx context.Context,
	in io.Reader,
	out io.Writer,
	interrupt <-chan os.Signal,
) (bool, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	reader := bufio.NewReader(in)
	for attempt := range 2 {
		lineCh := make(chan string, 1)
		errCh := make(chan error, 1)
		// A terminal line read cannot be cancelled directly. In the login
		// prompt we accept this fire-and-forget goroutine so Ctrl-C can abort
		// immediately; the process exits shortly after this path returns.
		go func() {
			line, err := reader.ReadString('\n')
			if err != nil {
				errCh <- err
				return
			}
			lineCh <- line
		}()

		var line string
		select {
		case <-ctx.Done():
			return false, false, ctx.Err()
		case <-interrupt:
			fmt.Fprintln(out)
			return false, false, context.Canceled
		case <-errCh:
			return false, false, nil
		case line = <-lineCh:
		}

		switch strings.ToLower(strings.TrimSpace(line)) {
		case "", "y", "yes":
			return true, true, nil
		case "n", "no":
			return false, true, nil
		default:
			if attempt == 0 {
				fmt.Fprint(out, "Please answer y or n: ")
			}
		}
	}
	return false, false, nil
}

func isTerminalReader(in io.Reader) bool {
	file, ok := in.(*os.File)
	if !ok {
		return false
	}
	fd := file.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

func (c *loginKonnectCmd) run(helper cmd.Helper) error {
	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	httpClient = httpclient.NewHTTPClient(15 * time.Second)

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if err := handleTelemetryPreference(
		helper.GetContext(),
		helper.GetStreams(),
		cfg,
		telemetry.FromContext(helper.GetContext()),
	); err != nil {
		c.SilenceUsage = true
		c.SilenceErrors = true
		return err
	}

	// Device authorization endpoints default to the global Konnect API host but can be overridden.
	authURL, pollURL := resolveAuthURLs(cfg)

	clientID := cfg.GetString(common.MachineClientIDConfigPath)

	resp, err := auth.RequestDeviceCode(httpClient, authURL, clientID, logger)
	if err != nil {
		return cmd.PrepareExecutionErrorWithHelper(helper, "failed to request device code", err)
	}

	if resp.UserCode == "" || resp.VerificationURI == "" || resp.VerificationURIComplete == "" ||
		resp.Interval == 0 || resp.ExpiresIn == 0 {
		return cmd.PrepareExecutionErrorMsg(helper,
			fmt.Sprintf("invalid device code request response from Konnect: %v", resp))
	}

	displayUserInstructions(helper.GetStreams().Out, resp)

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
			return cmd.PrepareExecutionErrorWithHelper(helper, "failed to poll for token", err)
		}

		if time.Now().After(expiresAt) {
			return cmd.PrepareExecutionErrorMsg(helper, "device authorization request has expired")
		}

		if pollResp != nil && pollResp.Token.AuthToken != "" {
			fmt.Fprintln(helper.GetStreams().Out, "\nUser successfully authorized")
			if err := auth.SaveAccessToken(cfg, pollResp); err != nil {
				return cmd.PrepareExecutionErrorWithHelper(helper, "failed to save tokens", err)
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
