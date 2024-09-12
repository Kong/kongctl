package gateway

import (
	"fmt"
	"net/url"

	"github.com/kong/go-database-reconciler/pkg/diff"
	"github.com/kong/go-database-reconciler/pkg/dump"
	"github.com/kong/go-database-reconciler/pkg/file"
	"github.com/kong/go-database-reconciler/pkg/state"
	"github.com/kong/go-database-reconciler/pkg/utils"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	gatewayCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway/controlplane"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/err"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

type applyGatewayCmd struct {
	*cobra.Command
}

var (
	applyGatewayShort = i18n.T("root.products.konnect.gateway.applyGatewayShort",
		"Apply configuration to a Konnect Kong Gateway")
	applyGatewayLong = i18n.T("root.products.konnect.gateway.applyGatewayLong",
		`Apply a configuration to a Konnect Kong Gateway resource. Entities will only be
created if they do not alrady exist. Pass files or directories containing the configuration as arguments,
and use '-' to read from stdin.`)
	applyGatewayExample = normalizers.Examples(
		i18n.T("root.products.konnect.gateway.applyGatewayExamples",
			fmt.Sprintf(`
	# Apply a configuration to a Konnect Kong Gateway by control plane ID 
	%[1]s apply konnect gateway --control-plane <cp-name> <path-to-config.yaml>`, meta.CLIName)))
)

func (c *applyGatewayCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) < 1 {
		return fmt.Errorf("apply requires one or more configuration file paths as arguments")
	}
	return nil
}

func getKonnectKongVersion() string {
	// Below copied from decK
	//
	// Returning an hardcoded version for now. reconciler only needs the version
	// to determine the format_version expected in the state file. Since
	// Konnect is always on the latest version, we can safely return the
	// latest version here and avoid making an extra and unnecessary request.
	return "3.5.0.0"
}

func (c *applyGatewayCmd) run(helper cmd.Helper) error {
	cfg, e := helper.GetConfig()
	if e != nil {
		return e
	}
	logger, e := helper.GetLogger()
	if e != nil {
		return e
	}

	outType, e := helper.GetOutputFormat()
	if e != nil {
		return e
	}

	printer, e := cli.Format(outType.String(), helper.GetStreams().Out)
	if e != nil {
		return e
	}

	targetContent, e := file.GetContentFromFiles(helper.GetArgs(), false)
	if e != nil {
		return e
	}
	// TODO: Validate the file, for example:
	// - Can't have a _workspace key
	// - Some features are not supported in Konnect(?)

	kkClient, e := helper.GetKonnectSDK(cfg, logger)
	if e != nil {
		return e
	}

	cpID, e := helpers.GetControlPlaneIDByNameIfNecessary(helper.GetContext(), kkClient.GetControlPlaneAPI(),
		cfg.GetString(gatewayCommon.ControlPlaneIDConfigPath),
		cfg.GetString(gatewayCommon.ControlPlaneNameConfigPath))
	if e != nil {
		attrs := err.TryConvertErrorToAttrs(e)
		return cmd.PrepareExecutionError("Failed to get Control Plane ID", e, helper.GetCmd(), attrs...)
	}

	// TODO: Support the creation of a new control plane with
	//  a flag, for example: --new-control-plane <name>

	baseURL := cfg.GetString(common.BaseURLConfigPath)
	coreEntityURL, e := url.JoinPath(baseURL, controlplane.ControlPlaneURL, cpID, "core-entities")
	if e != nil {
		return e
	}

	token, e := common.GetAccessToken(cfg, logger)
	if e != nil {
		return e
	}
	authHeader := "Authorization:Bearer " + token

	httpClient := utils.HTTPClient()

	kongClient, e := utils.GetKongClient(utils.KongClientConfig{
		Address:    coreEntityURL,
		HTTPClient: httpClient,
		Headers:    []string{authHeader},
	})
	if e != nil {
		return e
	}

	dumpConfig := dump.Config{}
	currentRawState, e := dump.Get(helper.GetContext(), kongClient, dumpConfig)
	if e != nil {
		return e
	}
	currentState, e := state.Get(currentRawState)
	if e != nil {
		return e
	}

	kongVersion, e := utils.ParseKongVersion(getKonnectKongVersion())
	if e != nil {
		return e
	}

	rawState, e := file.Get(
		helper.GetContext(),
		targetContent,
		file.RenderConfig{
			CurrentState: currentState,
			KongVersion:  kongVersion,
		},
		dump.Config{},
		kongClient)
	if e != nil {
		return e
	}

	targetState, e := state.Get(rawState)
	if e != nil {
		return e
	}

	syncer, e := diff.NewSyncer(diff.SyncerOpts{
		EnableEntityActions: false,
		CurrentState:        currentState,
		TargetState:         targetState,
		KongClient:          kongClient,
		StageDelaySec:       0,
		NoMaskValues:        true,
		IsKonnect:           true,
		NoDeletes:           true,
	})
	if e != nil {
		return e
	}

	dryRun := false
	isJSONOutput := true
	parallelism := 1

	errsChan := make(chan error)
	changesChan := make(chan diff.EntityChanges)

	// The solver runs in a separate goroutine so that we can receive the results
	// on the resultChan and apply them in the main goroutine here.
	go func() {
		_, errs, changes := syncer.Solve(
			helper.GetContext(),
			parallelism,
			dryRun,
			isJSONOutput)

		for _, err := range errs {
			errsChan <- err
		}
		close(errsChan)

		changesChan <- changes
		close(changesChan)
	}()

	errors := []error{}
	for err := range errsChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return &err.ErrorsBucket{
			Msg:    "Errors during apply",
			Errors: errors,
		}
	}

	changes := <-changesChan

	if outType == cmdCommon.TEXT {
		// TODO: Add display records for deck entity changes
		//var displayRecords []textDisplayRecord
		//for _, change := range changes {
		//	displayRecords = append(displayRecords, entityChangeToDisplayRecord(change))
		//}
		printer.Print(changes)
	} else {
		printer.Print(changes)
	}
	return nil
}

func (c *applyGatewayCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if e := c.validate(helper); e != nil {
		return e
	}

	return c.run(helper)
}

func newApplyGatewayCmd(verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *applyGatewayCmd {
	rv := &applyGatewayCmd{
		Command: baseCmd,
	}

	rv.Args = cobra.MinimumNArgs(1) // configuration files
	rv.Short = applyGatewayShort
	rv.Long = applyGatewayLong
	rv.Example = applyGatewayExample

	gatewayCommon.AddControlPlaneFlags(baseCmd)
	if addParentFlags != nil {
		addParentFlags(verb, rv.Command)
	}

	if parentPreRun != nil {
		rv.PreRunE = parentPreRun
		//rv.PreRunE = func(cmd *cobra.Command, args []string) error {
		//	return parentPreRun(cmd, args)
		//}
	}

	rv.RunE = rv.runE

	return rv
}
