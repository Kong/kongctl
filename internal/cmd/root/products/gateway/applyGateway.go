package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kong/go-database-reconciler/pkg/diff"
	"github.com/kong/go-database-reconciler/pkg/dump"
	"github.com/kong/go-database-reconciler/pkg/file"
	"github.com/kong/go-database-reconciler/pkg/state"
	"github.com/kong/go-database-reconciler/pkg/utils"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	gatewayCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/gateway/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/err"
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

func generateRandomFileName(prefix string, extension string) string {
	randBytes := make([]byte, 16)
	_, err := rand.Read(randBytes)
	if err != nil {
		panic(err)
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s.%s", prefix, hex.EncodeToString(randBytes), extension))
}

func (c *applyGatewayCmd) run(helper cmd.Helper) error {
	outType, e := helper.GetOutputFormat()
	if e != nil {
		return e
	}

	printer, e := cli.Format(outType.String(), helper.GetStreams().Out)
	if e != nil {
		return e
	}

	httpClient := utils.HTTPClient()

	kongClient, e := utils.GetKongClient(utils.KongClientConfig{
		Address:    "http://localhost:8001",
		HTTPClient: httpClient,
	})
	if e != nil {
		return e
	}

	dumpConfig := dump.Config{}
	currentRawState, e := dump.Get(helper.GetContext(), kongClient, dumpConfig)
	if e != nil {
		return e
	}

	currentKongState, e := state.Get(currentRawState)
	if e != nil {
		return e
	}

	inputFileContent, e := file.GetContentFromFiles(helper.GetArgs(), false)
	if e != nil {
		return e
	}

	kongVersion, e := utils.ParseKongVersion(getKonnectKongVersion())
	if e != nil {
		return e
	}

	targetRawState, e := file.Get(
		helper.GetContext(),
		inputFileContent,
		file.RenderConfig{
			CurrentState: currentKongState,
			KongVersion:  kongVersion,
		},
		dump.Config{},
		kongClient)
	if e != nil {
		return e
	}

	targetKongState, e := state.Get(targetRawState)
	if e != nil {
		return e
	}

	syncer, e := diff.NewSyncer(diff.SyncerOpts{
		EnableEntityActions: false,
		CurrentState:        currentKongState,
		TargetState:         targetKongState,
		KongClient:          kongClient,
		StageDelaySec:       0,
		NoMaskValues:        true,
		IsKonnect:           false,
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

func newApplyGatewayCmd(_ verbs.VerbValue,
	baseCmd *cobra.Command,
) *applyGatewayCmd {
	rv := &applyGatewayCmd{
		Command: baseCmd,
	}

	rv.Args = cobra.MinimumNArgs(1) // configuration files
	rv.Short = applyGatewayShort
	rv.Long = applyGatewayLong
	rv.Example = applyGatewayExample

	gatewayCommon.AddControlPlaneFlags(baseCmd)

	rv.RunE = rv.runE

	return rv
}
