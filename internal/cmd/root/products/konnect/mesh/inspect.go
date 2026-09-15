package mesh

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/table"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	meshcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

// TypeFlagName selects which inspection a dataplane inspect performs.
const TypeFlagName = "type"

// Inspection types for `inspect dataplane`. The first reads a control plane
// computation; the rest proxy the proxy's own Envoy admin interface, which
// only answers while the zone is connected.
const (
	InspectPolicies = "policies"
	InspectXDS      = "xds"
	InspectStats    = "stats"
	InspectClusters = "clusters"
	InspectConfig   = "config"
)

var inspectDataplaneTypes = []string{
	InspectPolicies, InspectXDS, InspectStats, InspectClusters, InspectConfig,
}

var (
	inspectShort = i18n.T("root.products.konnect.mesh.inspectShort",
		"Inspect what a control plane computed for a resource")

	inspectLong = normalizers.LongDesc(i18n.T("root.products.konnect.mesh.inspectLong",
		`Inspect what a Kong Mesh control plane computed, rather than what was
applied to it.

'get mesh' reads resources back as they were written. 'inspect' answers the
questions that only the control plane can: which policies apply to a proxy
once matching and merging have run, and which proxies a policy matches.`))

	inspectExample = normalizers.Examples(i18n.T("root.products.konnect.mesh.inspectExample",
		fmt.Sprintf(`
	# Policies that apply to a proxy, after matching and merging
	%[1]s get mesh inspect dataplane backend-01

	# Proxies a policy matches
	%[1]s get mesh inspect meshtimeout slow

	# A proxy's Envoy configuration, while its zone is connected
	%[1]s get mesh inspect dataplane backend-01 --type stats

	# Overviews
	%[1]s get mesh inspect dataplanes
	%[1]s get mesh inspect meshes
	%[1]s get mesh inspect zones
	`, meta.CLIName)))
)

// newInspectCmd builds `get mesh inspect`. It sits under the get verb rather
// than becoming a verb of its own, because every other kongctl product groups
// reads under get and an inspect verb unique to mesh would read as an
// exception to that.
func newInspectCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	baseCmd := &cobra.Command{
		Use:     "inspect",
		Short:   inspectShort,
		Long:    inspectLong,
		Example: inspectExample,
		// A policy type is not a fixed subcommand, so it falls through to here.
		RunE: func(cmdObj *cobra.Command, args []string) error {
			helper := cmd.BuildHelper(cmdObj, args)
			if len(args) == 0 {
				return cmd.RequireSubcommand(cmdObj, args)
			}
			// Suggesting a near miss keeps a mistyped subcommand from being
			// sent to the control plane as a resource type.
			if len(cmdObj.SuggestionsFor(args[0])) > 0 {
				return cmd.UnknownSubcommandError(cmdObj, args[0])
			}
			if len(args) != 2 {
				return &cmd.ConfigurationError{
					Err: fmt.Errorf(
						"expected a policy type and a name, for example 'get mesh inspect meshtimeout <name>'"),
				}
			}
			return runInspectPolicy(helper, args[0], args[1])
		},
	}

	if parentPreRun != nil {
		baseCmd.PreRunE = parentPreRun
	}
	if addParentFlags != nil {
		addParentFlags(verb, baseCmd)
	}

	baseCmd.AddCommand(newInspectDataplaneCmd(verb, addParentFlags, parentPreRun))
	baseCmd.AddCommand(newInspectOverviewCmd(
		"dataplanes", "Overview of every dataplane in the mesh", overviewDataplanes,
		verb, addParentFlags, parentPreRun))
	baseCmd.AddCommand(newInspectOverviewCmd(
		"meshes", "Overview of every mesh", overviewMeshes,
		verb, addParentFlags, parentPreRun))
	baseCmd.AddCommand(newInspectOverviewCmd(
		"zones", "Overview of every zone", overviewZones,
		verb, addParentFlags, parentPreRun))

	return baseCmd
}

func newInspectDataplaneCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmdObj := &cobra.Command{
		Use:     "dataplane NAME",
		Aliases: []string{"dp"},
		Short:   "Inspect what applies to one dataplane",
		Args:    cobra.ExactArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			helper := cmd.BuildHelper(cobraCmd, args)
			inspection, err := cobraCmd.Flags().GetString(TypeFlagName)
			if err != nil {
				return err
			}
			if !slices.Contains(inspectDataplaneTypes, inspection) {
				return &cmd.ConfigurationError{
					Err: fmt.Errorf("invalid --%s %q; expected one of %s",
						TypeFlagName, inspection, strings.Join(inspectDataplaneTypes, ", ")),
				}
			}
			return runInspectDataplane(helper, args[0], inspection)
		},
	}

	cmdObj.Flags().String(TypeFlagName, InspectPolicies,
		fmt.Sprintf("What to inspect. One of: %s.", strings.Join(inspectDataplaneTypes, ", ")))

	if parentPreRun != nil {
		cmdObj.PreRunE = parentPreRun
	}
	if addParentFlags != nil {
		addParentFlags(verb, cmdObj)
	}
	return cmdObj
}

type overviewKind int

const (
	overviewDataplanes overviewKind = iota
	overviewMeshes
	overviewZones
)

func newInspectOverviewCmd(
	use string,
	short string,
	kind overviewKind,
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmdObj := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			helper := cmd.BuildHelper(cobraCmd, args)
			return runInspectOverview(helper, kind)
		},
	}
	if parentPreRun != nil {
		cmdObj.PreRunE = parentPreRun
	}
	if addParentFlags != nil {
		addParentFlags(verb, cmdObj)
	}
	return cmdObj
}

// runInspectDataplane reads either the policies the control plane matched to a
// proxy, or the proxy's Envoy admin output relayed by the control plane.
func runInspectDataplane(helper cmd.Helper, name, inspection string) error {
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	mesh := meshcommon.ResolveMesh(cfg)

	if inspection != InspectPolicies {
		// Envoy admin output has no shape kongctl can usefully tabulate, so it
		// is relayed verbatim the way the proxy produced it.
		path := fmt.Sprintf("/meshes/%s/dataplanes/%s/%s", mesh, name, inspection)
		body, err := fetch(helper, path)
		if err != nil {
			return cmd.PrepareExecutionError(
				fmt.Sprintf("failed to inspect dataplane %s", name), err, helper.GetCmd())
		}
		_, err = helper.GetStreams().Out.Write(body)
		return err
	}

	// Policies are reported per inbound and per outbound, not for the proxy as
	// a whole: the control plane's own top-level _policies endpoint answers
	// with an empty list even for a proxy a mesh-wide policy demonstrably
	// matches. So read the layout first and ask about each port, which is also
	// the answer an operator wants -- a policy applying to one port of a proxy
	// and not another is exactly what is hard to see otherwise.
	layoutPath := fmt.Sprintf("/meshes/%s/dataplanes/%s/_layout", mesh, name)
	layoutBody, err := fetch(helper, layoutPath)
	if err != nil {
		return cmd.PrepareExecutionError(
			fmt.Sprintf("failed to inspect dataplane %s", name), err, helper.GetCmd())
	}

	var layout struct {
		Inbounds []struct {
			KRI string `json:"kri"`
		} `json:"inbounds"`
		Outbounds []struct {
			KRI string `json:"kri"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(layoutBody, &layout); err != nil {
		return fmt.Errorf("failed to decode dataplane layout response: %w", err)
	}

	type scoped struct {
		direction string
		kri       string
	}
	ports := make([]scoped, 0, len(layout.Inbounds)+len(layout.Outbounds))
	for _, inbound := range layout.Inbounds {
		ports = append(ports, scoped{"_inbounds", inbound.KRI})
	}
	for _, outbound := range layout.Outbounds {
		ports = append(ports, scoped{"_outbounds", outbound.KRI})
	}

	var rows []inspectPolicyRow
	collected := map[string]any{}
	for _, port := range ports {
		portPath := fmt.Sprintf("/meshes/%s/dataplanes/%s/%s/%s/_policies",
			mesh, name, port.direction, port.kri)
		portBody, err := fetch(helper, portPath)
		if err != nil {
			return cmd.PrepareExecutionError(
				fmt.Sprintf("failed to inspect dataplane %s", name), err, helper.GetCmd())
		}

		var envelope struct {
			Policies []map[string]any `json:"policies"`
		}
		if err := json.Unmarshal(portBody, &envelope); err != nil {
			return fmt.Errorf("failed to decode dataplane policies response: %w", err)
		}
		if len(envelope.Policies) > 0 {
			collected[port.kri] = envelope.Policies
		}
		for _, policy := range envelope.Policies {
			rows = append(rows, inspectPolicyRow{
				Port:    sectionOf(port.kri),
				Kind:    stringField(policy, "kind"),
				Origins: joinOrigins(policy["origins"]),
			})
		}
	}

	aggregated, err := json.Marshal(collected)
	if err != nil {
		return fmt.Errorf("failed to encode dataplane policies: %w", err)
	}

	return renderInspect(helper, aggregated, rows,
		[]string{"PORT", "KIND", "ORIGINS"},
		func(r inspectPolicyRow) table.Row { return table.Row{r.Port, r.Kind, r.Origins} },
		fmt.Sprintf("Policies for %s", name))
}

// runInspectPolicy reads the proxies a policy matches.
func runInspectPolicy(helper cmd.Helper, typeArg, name string) error {
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	descriptors, err := Discover(helper)
	if err != nil {
		return cmd.PrepareExecutionError("failed to retrieve mesh resource types", err, helper.GetCmd())
	}
	descriptor, err := ResolveType(descriptors, typeArg)
	if err != nil {
		return &cmd.ConfigurationError{Err: err}
	}
	if !descriptor.IsPolicy() {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"%s is not a policy, so nothing matches it; inspect applies to policy types",
				descriptor.Plural()),
		}
	}

	mesh := meshcommon.ResolveMesh(cfg)
	path := descriptor.ItemPath(mesh, name) + "/_resources/dataplanes"
	body, err := fetch(helper, path)
	if err != nil {
		return cmd.PrepareExecutionError(
			fmt.Sprintf("failed to inspect %s %s", descriptor.Singular(), name), err, helper.GetCmd())
	}

	var envelope listEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("failed to decode matched dataplanes response: %w", err)
	}

	rows := make([]inspectDataplaneRow, 0, len(envelope.Items))
	for _, item := range envelope.Items {
		rows = append(rows, inspectDataplaneRow{
			Mesh: stringField(item, "mesh"),
			Name: stringField(item, "name"),
		})
	}

	return renderInspect(helper, body, rows,
		[]string{colMesh, colName},
		func(r inspectDataplaneRow) table.Row { return table.Row{r.Mesh, r.Name} },
		fmt.Sprintf("Dataplanes matched by %s", name))
}

// runInspectOverview reads an overview collection.
func runInspectOverview(helper cmd.Helper, kind overviewKind) error {
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	var path, label string
	switch kind {
	case overviewDataplanes:
		if cfg.GetBool(meshcommon.AllMeshesConfigPath) {
			path, label = "/dataplanes/_overview", "Dataplanes"
		} else {
			mesh := meshcommon.ResolveMesh(cfg)
			path = fmt.Sprintf("/meshes/%s/dataplanes/_overview", mesh)
			label = "Dataplanes"
		}
	case overviewMeshes:
		path, label = "/meshes/_overview", "Meshes"
	case overviewZones:
		path, label = "/zones/_overview", "Zones"
	}

	body, err := fetch(helper, path)
	if err != nil {
		return cmd.PrepareExecutionError(
			fmt.Sprintf("failed to inspect %s", strings.ToLower(label)), err, helper.GetCmd())
	}

	var envelope listEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("failed to decode %s overview response: %w", strings.ToLower(label), err)
	}

	rows := make([]inspectOverviewRow, 0, len(envelope.Items))
	for _, item := range envelope.Items {
		status := overviewStatus(item)
		if kind == overviewMeshes {
			status = meshDataplaneCounts(item)
		}
		rows = append(rows, inspectOverviewRow{
			Mesh:   stringField(item, "mesh"),
			Name:   stringField(item, "name"),
			Status: status,
		})
	}

	// Each overview reports something different, so each gets the columns that
	// carry information for it. A mesh has no connection of its own; what an
	// operator wants from it is how many of its proxies are up.
	switch kind {
	case overviewDataplanes:
		return renderInspect(helper, body, rows,
			[]string{colMesh, colName, "STATUS"},
			func(r inspectOverviewRow) table.Row { return table.Row{r.Mesh, r.Name, r.Status} },
			label)
	case overviewMeshes:
		return renderInspect(helper, body, rows,
			[]string{colName, "DATAPLANES"},
			func(r inspectOverviewRow) table.Row { return table.Row{r.Name, r.Status} },
			label)
	case overviewZones:
		return renderInspect(helper, body, rows,
			[]string{colName, "STATUS"},
			func(r inspectOverviewRow) table.Row { return table.Row{r.Name, r.Status} },
			label)
	default:
		return fmt.Errorf("unknown overview kind %d", kind)
	}
}

type inspectPolicyRow struct {
	Port    string `table:"PORT"`
	Kind    string `table:"KIND"`
	Origins string `table:"ORIGINS"`
}

type inspectDataplaneRow struct {
	Mesh string `table:"MESH"`
	Name string `table:"NAME"`
}

type inspectOverviewRow struct {
	Mesh   string `table:"MESH"`
	Name   string `table:"NAME"`
	Status string `table:"STATUS"`
}

// renderInspect prints rows as a table, or the control plane payload verbatim
// for JSON and YAML, matching how the other mesh reads behave.
func renderInspect[T any](
	helper cmd.Helper,
	body []byte,
	rows []T,
	headers []string,
	toRow func(T) table.Row,
	label string,
) error {
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}
	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("failed to decode inspect response: %w", err)
	}

	tableRows := make([]table.Row, 0, len(rows))
	for _, row := range rows {
		tableRows = append(tableRows, toRow(row))
	}

	return tableview.RenderForFormat(
		helper, false, outType, printer, helper.GetStreams(),
		rows, raw, label,
		tableview.WithExactCustomTable(headers, tableRows),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

// joinOrigins renders the origins of a matched policy. The control plane
// reports each as an object, and the name is what identifies it to a reader.
func joinOrigins(value any) string {
	list, ok := value.([]any)
	if !ok {
		return ""
	}
	names := make([]string, 0, len(list))
	for _, entry := range list {
		origin, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if name := stringField(origin, "name"); name != "" {
			names = append(names, name)
			continue
		}
		if kri := stringField(origin, "kri"); kri != "" {
			names = append(names, kri)
		}
	}
	return strings.Join(names, ", ")
}

// sectionOf returns the section name at the end of a KRI, which is the port or
// inbound name a reader recognises. A KRI is underscore separated and ends with
// the section, so the last segment is it.
func sectionOf(kri string) string {
	if kri == "" {
		return ""
	}
	parts := strings.Split(strings.TrimSuffix(kri, "_"), "_")
	return parts[len(parts)-1]
}

// meshDataplaneCounts renders how many of a mesh's proxies are connected. A
// Mesh has no subscription of its own, so a connection status would always be
// blank; the counts are what the overview is actually for.
func meshDataplaneCounts(item map[string]any) string {
	insight, ok := item["meshInsight"].(map[string]any)
	if !ok {
		return ""
	}
	counts, ok := insight["dataplanes"].(map[string]any)
	if !ok {
		return ""
	}
	number := func(key string) int64 {
		if value, ok := counts[key].(float64); ok {
			return int64(value)
		}
		return 0
	}
	return fmt.Sprintf("%d/%d online", number("online"), number("total"))
}

// overviewStatus reports whether the subject is connected to the control
// plane. An overview carries no status field: the control plane reports the
// subscriptions instead, and Kuma reads one as online when it has a connect
// time and no disconnect time. Matching that here keeps the column meaning
// what it means in kumactl and the GUI.
func overviewStatus(item map[string]any) string {
	for _, key := range []string{"dataplaneInsight", "zoneInsight"} {
		insight, ok := item[key].(map[string]any)
		if !ok {
			continue
		}
		subscriptions, ok := insight["subscriptions"].([]any)
		if !ok {
			continue
		}
		for _, entry := range subscriptions {
			subscription, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			if subscription["connectTime"] != nil && subscription["disconnectTime"] == nil {
				return "Online"
			}
		}
		return "Offline"
	}
	return ""
}
