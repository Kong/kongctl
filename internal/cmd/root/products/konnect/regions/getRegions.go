package regions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const availableRegionsPath = "/v3/available-regions"

var (
	getRegionsShort = i18n.T("root.products.konnect.regions.getRegionsShort",
		"List available Konnect regions")
	getRegionsLong = i18n.T("root.products.konnect.regions.getRegionsLong",
		`Use the get verb with the regions command to retrieve the regions supported by Konnect.`)
	getRegionsExample = normalizers.Examples(i18n.T("root.products.konnect.regions.getRegionsExample",
		fmt.Sprintf(`
	# List Konnect regions
	%[1]s get regions
	`, meta.CLIName)))
)

type availableRegionsResponse struct {
	Regions availableRegionGroups `json:"regions"`
}

type availableRegionGroups struct {
	Stable      []string `json:"stable"`
	StableOptIn []string `json:"stable_opt_in"`
	Beta        []string `json:"beta"`
}

type regionRow struct {
	Category string `table:"Category"`
	Regions  string `table:"Regions"`
}

type getRegionsCmd struct {
	*cobra.Command
}

var fetchRegionsFn = fetchAvailableRegions

func fetchAvailableRegions(ctx context.Context) (*availableRegionsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		common.GlobalBaseURL+availableRegionsPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve Konnect regions: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return nil, fmt.Errorf("konnect regions request failed: status %d: %s",
			res.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload availableRegionsResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode Konnect regions response: %w", err)
	}
	return &payload, nil
}

func (c *getRegionsCmd) validate(helper cmd.Helper) error {
	if len(helper.GetArgs()) > 0 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("the regions command does not accept arguments"),
		}
	}
	return nil
}

func (c *getRegionsCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if err := c.validate(helper); err != nil {
		return err
	}

	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	ctx := helper.GetContext()
	if ctx == nil {
		ctx = context.Background()
	}

	payload, err := fetchRegionsFn(ctx)
	if err != nil {
		return cmd.PrepareExecutionError("failed to retrieve Konnect regions", err, helper.GetCmd())
	}

	rows := []regionRow{
		{Category: "Stable", Regions: strings.Join(payload.Regions.Stable, ", ")},
		{Category: "Stable Opt-In", Regions: strings.Join(payload.Regions.StableOptIn, ", ")},
		{Category: "Beta", Regions: strings.Join(payload.Regions.Beta, ", ")},
	}

	return tableview.RenderForFormat(helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		rows,
		payload,
		"Konnect Regions",
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func newGetRegionsCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *getRegionsCmd {
	rv := &getRegionsCmd{Command: baseCmd}
	rv.Short = getRegionsShort
	rv.Long = getRegionsLong
	rv.Example = getRegionsExample
	rv.RunE = rv.runE
	if parentPreRun != nil {
		rv.PreRunE = parentPreRun
	}

	if addParentFlags != nil {
		addParentFlags(verb, rv.Command)
	}

	return rv
}
