package mesh

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/table"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

var (
	getControlPlanesShort = i18n.T("root.products.konnect.mesh.getControlPlanesShort",
		"List the Kong Mesh control planes available to you")

	getControlPlanesLong = normalizers.LongDesc(i18n.T("root.products.konnect.mesh.getControlPlanesLong",
		`List the Konnect hosted Kong Mesh control planes the authenticated identity
can see, with the identifier every other mesh command needs.

The API LINE column is the Konnect API line a control plane is reached on,
not the version it runs. A control plane on the v3 line runs Kong Mesh 3;
one on the v0 line runs the 2.14 series, which this command surface does
not support. To read the version a control plane actually runs, address it
and read its index endpoint.

Identifiers are abbreviated in text output, as they are elsewhere in
kongctl. Pass --text-id-format full to print them whole, or -o json. In
most cases the identifier is not needed at all: other mesh commands accept
--control-plane-name.`))

	getControlPlanesExample = normalizers.Examples(i18n.T("root.products.konnect.mesh.getControlPlanesExample",
		fmt.Sprintf(`
	# List the control planes available
	%[1]s get mesh control-planes

	# Use one by name, without looking up its identifier
	%[1]s get mesh dataplanes --control-plane-name my-mesh

	# Print identifiers in full, to copy one
	%[1]s get mesh control-planes --text-id-format full
	`, meta.CLIName)))
)

// controlPlaneRow is the text table projection of a control plane.
type controlPlaneRow struct {
	Name    string `json:"name" table:"NAME"`
	ID      string `json:"id" table:"ID"`
	APILine string `json:"api_line" table:"API LINE"`
}

type getControlPlanesCmd struct {
	*cobra.Command
}

func newGetControlPlanesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &getControlPlanesCmd{}
	cmdObj := &cobra.Command{
		Use:     "control-planes",
		Aliases: []string{"control-plane", "cps", "cp"},
		Short:   getControlPlanesShort,
		Long:    getControlPlanesLong,
		Example: getControlPlanesExample,
		Args:    cobra.NoArgs,
		RunE:    c.runE,
	}

	c.Command = cmdObj
	if parentPreRun != nil {
		c.PreRunE = parentPreRun
	}
	if addParentFlags != nil {
		addParentFlags(verb, c.Command)
	}
	return c.Command
}

func (c *getControlPlanesCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)

	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	controlPlanes, err := ListControlPlanes(helper)
	if err != nil {
		return cmd.PrepareExecutionError("failed to list mesh control planes", err, helper.GetCmd())
	}

	rows := make([]controlPlaneRow, 0, len(controlPlanes))
	for _, controlPlane := range controlPlanes {
		rows = append(rows, controlPlaneRow{
			Name:    controlPlane.Name,
			ID:      controlPlane.ID,
			APILine: controlPlane.Version,
		})
	}
	slices.SortFunc(rows, func(a, b controlPlaneRow) int {
		return strings.Compare(a.Name, b.Name)
	})

	tableRows := make([]table.Row, 0, len(rows))
	for _, row := range rows {
		tableRows = append(tableRows, table.Row{row.Name, row.ID, row.APILine})
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		rows,
		controlPlanes,
		"Mesh Control Planes",
		tableview.WithExactCustomTable([]string{"NAME", "ID", "API LINE"}, tableRows),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}
