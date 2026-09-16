package mesh

import (
	"fmt"
	"net/http"

	"charm.land/bubbles/v2/table"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	meshcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh/common"
	"github.com/segmentio/cli"
)

// deleteRow is the text table projection of a deleted resource.
type deleteRow struct {
	Type   string `json:"type" table:"TYPE"`
	Name   string `json:"name" table:"NAME"`
	Mesh   string `json:"mesh" table:"MESH"`
	Result string `json:"result" table:"RESULT"`
}

// runDeleteResources serves `delete mesh <type> <name>`.
//
// The type is resolved from /_resources like every other mesh command, so no
// resource type is named in code here either.
func runDeleteResources(helper cmd.Helper, args []string) error {
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	descriptors, err := Discover(helper)
	if err != nil {
		return cmd.PrepareExecutionError("failed to retrieve mesh resource types", err, helper.GetCmd())
	}

	descriptor, err := ResolveType(descriptors, args[0])
	if err != nil {
		return &cmd.ConfigurationError{Err: err}
	}

	// The control plane also refuses this with a 405, but naming the type is
	// more use than reporting a status.
	if descriptor.ReadOnly {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"%s is read only on this control plane and cannot be deleted", descriptor.Singular()),
		}
	}

	name := args[1]
	mesh := ""
	if descriptor.IsMeshScoped() {
		mesh = meshcommon.ResolveMesh(cfg)
	}

	if _, err := sendForStatus(helper, http.MethodDelete, descriptor.ItemPath(mesh, name), nil); err != nil {
		return cmd.PrepareExecutionError(
			fmt.Sprintf("failed to delete %s %s", descriptor.Singular(), name), err, helper.GetCmd())
	}

	return reportDeleted(helper, descriptor, mesh, name)
}

func reportDeleted(helper cmd.Helper, descriptor ResourceDescriptor, mesh, name string) error {
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	rows := []deleteRow{{
		Type:   descriptor.Name,
		Name:   name,
		Mesh:   mesh,
		Result: "deleted",
	}}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		rows,
		rows,
		"Deleted Mesh Resource",
		tableview.WithExactCustomTable(
			[]string{colType, colName, colMesh, colResult},
			[]table.Row{{descriptor.Name, name, mesh, "deleted"}},
		),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}
