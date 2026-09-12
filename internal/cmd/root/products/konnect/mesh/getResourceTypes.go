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
	getResourceTypesShort = i18n.T("root.products.konnect.mesh.getResourceTypesShort",
		"List the resource types a Kong Mesh control plane serves")

	getResourceTypesLong = normalizers.LongDesc(i18n.T("root.products.konnect.mesh.getResourceTypesLong",
		`List the resource types the selected Kong Mesh control plane serves,
along with the name each is addressed by, its scope, and whether it can be
modified.

Use this to discover what a control plane supports, including policies and
enterprise resource types that vary between Kong Mesh releases.`))

	getResourceTypesExample = normalizers.Examples(i18n.T("root.products.konnect.mesh.getResourceTypesExample",
		fmt.Sprintf(`
	# List every resource type a control plane serves
	%[1]s get mesh resource-types --control-plane-id <id>

	# Show the full descriptors reported by the control plane
	%[1]s get mesh resource-types --control-plane-id <id> -o json
	`, meta.CLIName)))
)

// resourceTypeRow is the text table projection of a resource descriptor.
type resourceTypeRow struct {
	Name     string `table:"NAME"`
	Alias    string `table:"ALIAS"`
	Scope    string `table:"SCOPE"`
	Kind     string `table:"KIND"`
	Writable string `table:"WRITABLE"`
}

type getResourceTypesCmd struct {
	*cobra.Command
}

func newGetResourceTypesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	c := &getResourceTypesCmd{}
	cmdObj := &cobra.Command{
		Use:     "resource-types",
		Aliases: []string{"resource-type", "types"},
		Short:   getResourceTypesShort,
		Long:    getResourceTypesLong,
		Example: getResourceTypesExample,
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

func (c *getResourceTypesCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if len(helper.GetArgs()) > 0 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("the resource-types command does not accept arguments"),
		}
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

	descriptors, err := Discover(helper)
	if err != nil {
		return cmd.PrepareExecutionError("failed to retrieve mesh resource types", err, helper.GetCmd())
	}

	rows := buildResourceTypeRows(descriptors)

	// The default text table curates itself down to a few columns chosen by
	// heuristic. Every column here carries information an operator needs to
	// act, so the table is declared exactly.
	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		rows,
		descriptors,
		"Mesh Resource Types",
		tableview.WithExactCustomTable(resourceTypeHeaders, toResourceTypeTableRows(rows)),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

// resourceTypeHeaders is the column set for the resource type listing.
var resourceTypeHeaders = []string{colName, "ALIAS", "SCOPE", "KIND", "WRITABLE"}

func toResourceTypeTableRows(rows []resourceTypeRow) []table.Row {
	tableRows := make([]table.Row, 0, len(rows))
	for _, row := range rows {
		tableRows = append(tableRows, table.Row{row.Name, row.Alias, row.Scope, row.Kind, row.Writable})
	}
	return tableRows
}

// buildResourceTypeRows projects descriptors into table rows, sorted by the
// name used to address each type so the listing reads predictably.
func buildResourceTypeRows(descriptors []ResourceDescriptor) []resourceTypeRow {
	rows := make([]resourceTypeRow, 0, len(descriptors))
	for _, descriptor := range descriptors {
		rows = append(rows, resourceTypeRow{
			Name:     descriptor.Path,
			Alias:    descriptor.Alias(),
			Scope:    descriptor.Scope,
			Kind:     describeKind(descriptor),
			Writable: writableLabel(descriptor),
		})
	}

	slices.SortFunc(rows, func(a, b resourceTypeRow) int {
		return strings.Compare(a.Name, b.Name)
	})
	return rows
}

func describeKind(descriptor ResourceDescriptor) string {
	if descriptor.IsPolicy() {
		return "policy"
	}
	return "resource"
}

func writableLabel(descriptor ResourceDescriptor) string {
	if descriptor.ReadOnly {
		return "no"
	}
	return "yes"
}
