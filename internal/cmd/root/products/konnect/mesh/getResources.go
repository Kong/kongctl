package mesh

import (
	"encoding/json"
	"fmt"
	"time"

	"charm.land/bubbles/v2/table"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	meshcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/segmentio/cli"
)

// listEnvelope is the envelope Kuma wraps resource lists in. A single resource
// is returned bare, without one.
type listEnvelope struct {
	Total int              `json:"total"`
	Items []map[string]any `json:"items"`
	Next  string           `json:"next"`
}

// runGetResources serves `get mesh <type> [name]` for every resource type the
// control plane advertises.
//
// There is deliberately no per-type code and no compiled-in type table: the
// type is resolved from /_resources, the URL is composed from the descriptor's
// path and scope, and the columns come from the three printers in printers.go.
// A resource type added by a newer Kong Mesh release therefore works without a
// kongctl release.
func runGetResources(helper cmd.Helper, args []string) error {
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	cfg, err := helper.GetConfig()
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

	descriptor, err := ResolveType(descriptors, args[0])
	if err != nil {
		return &cmd.ConfigurationError{Err: err}
	}

	var name string
	if len(args) > 1 {
		name = args[1]
	}

	path, err := requestPath(cfg, descriptor, name)
	if err != nil {
		return &cmd.ConfigurationError{Err: err}
	}

	body, err := fetch(helper, path)
	if err != nil {
		return cmd.PrepareExecutionError(
			fmt.Sprintf("failed to retrieve mesh %s", descriptor.Plural()), err, helper.GetCmd())
	}

	// JSON and YAML print the control plane payload as it arrived, so scripts
	// written against kumactl continue to parse it (NFR-1).
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("failed to decode mesh %s response: %w", descriptor.Plural(), err)
	}

	items, err := itemsFrom(body, name)
	if err != nil {
		return err
	}

	rows := buildRows(items, time.Now())
	headers := headersFor(descriptor)

	tableRows := make([]table.Row, 0, len(rows))
	for _, row := range rows {
		tableRows = append(tableRows, table.Row(cellsFor(descriptor, row)))
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		rows,
		raw,
		descriptor.Plural(),
		tableview.WithExactCustomTable(headers, tableRows),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

// requestPath composes the control plane path for the resolved type, applying
// the mesh only where the discovered scope calls for one.
func requestPath(cfg config.Hook, descriptor ResourceDescriptor, name string) (string, error) {
	if !descriptor.IsMeshScoped() {
		if name != "" {
			return descriptor.ItemPath("", name), nil
		}
		return descriptor.CollectionPath(""), nil
	}

	// Mesh scoped types are registered at both /meshes/{mesh}/{path} and
	// /{path}, the latter listing across every mesh.
	if cfg.GetBool(meshcommon.AllMeshesConfigPath) {
		if name != "" {
			return "", fmt.Errorf(
				"--%s lists across meshes and cannot address a single resource; drop it and pass --%s",
				meshcommon.AllMeshesFlagName, meshcommon.MeshFlagName)
		}
		return "/" + descriptor.Path, nil
	}

	mesh := meshcommon.ResolveMesh(cfg)
	if name != "" {
		return descriptor.ItemPath(mesh, name), nil
	}
	return descriptor.CollectionPath(mesh), nil
}

// itemsFrom extracts the rows to render. A list arrives wrapped in an
// envelope; a single named resource arrives bare.
func itemsFrom(body []byte, name string) ([]map[string]any, error) {
	if name != "" {
		var item map[string]any
		if err := json.Unmarshal(body, &item); err != nil {
			return nil, fmt.Errorf("failed to decode mesh resource response: %w", err)
		}
		return []map[string]any{item}, nil
	}

	var envelope listEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("failed to decode mesh resource list response: %w", err)
	}
	return envelope.Items, nil
}
