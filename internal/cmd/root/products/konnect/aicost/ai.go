package aicost

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/kong/kongctl/internal/declarative/state"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

type readResource struct {
	name        string
	description string
	kind        resources.ResourceType
	detail      bool
	parent      bool
}

func NewDirectCmd(verb verbs.VerbValue) *cobra.Command {
	return NewCmd(verb, nil, func(cmd *cobra.Command, _ []string) error {
		ctx := context.WithValue(cmd.Context(), products.Product, products.ProductKonnect)
		ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, common.GetSDKFactoryForVerb(verb))
		cmd.SetContext(ctx)
		return nil
	})
}

func NewCmd(verb verbs.VerbValue, flags func(verbs.VerbValue, *cobra.Command),
	preRun func(*cobra.Command, []string) error,
) *cobra.Command {
	ai := &cobra.Command{Use: "ai", Short: "Inspect Konnect AI resources"}
	cost := &cobra.Command{Use: "cost-management", Short: "Inspect AI usage and cost management"}
	cmdpkg.ConfigureRequiresSubcommand(ai)
	cmdpkg.ConfigureRequiresSubcommand(cost)
	ai.AddCommand(cost)
	for _, resource := range []readResource{
		{name: "budgets", description: "List or get AI cost budgets", kind: resources.ResourceTypeAICostBudget, detail: true},
		{
			name: "notification-channels", description: "List or get budget alert destinations",
			kind: resources.ResourceTypeAICostChannel, detail: true,
		},
		{
			name: "export-schedules", description: "List or get recurring cost exports",
			kind: resources.ResourceTypeAICostSchedule, detail: true,
		},
		{
			name: "notification-rules", description: "Get the notification rule for a limit budget",
			kind: resources.ResourceTypeAICostRule, detail: true, parent: true,
		},
		{name: "budget-balances", description: "List budget balances or get a budget balance", detail: true},
		{name: "notification-events", description: "List or get AI cost notification events", detail: true},
		{name: "notification-firings", description: "List threshold firings for a limit budget", parent: true},
		{name: "exports", description: "List or get usage export jobs", detail: true},
		{name: "usage-events", description: "List AI usage events"},
		{name: "forecast", description: "Get the monthly AI cost forecast"},
	} {
		if resource.parent && resource.detail && verb == verbs.List {
			continue
		}
		command := &cobra.Command{Use: resource.name, Short: resource.description, Args: cobra.NoArgs, PreRunE: preRun}
		if resource.parent {
			command.Use += " <budget-id>"
			command.Args = cobra.ExactArgs(1)
		} else if resource.detail && verb == verbs.Get {
			command.Use += " [id]"
			command.Args = cobra.MaximumNArgs(1)
		}
		if flags != nil {
			flags(verb, command)
		}
		command.RunE = func(c *cobra.Command, args []string) error { return run(c, args, resource) }
		cost.AddCommand(command)
	}
	return ai
}

func run(command *cobra.Command, args []string, resource readResource) error {
	id := ""
	if len(args) > 0 {
		id = args[0]
		if !util.IsValidUUID(id) {
			return fmt.Errorf("%s requires a UUID", resource.name)
		}
	}
	helper := cmdpkg.BuildHelper(command, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}
	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}
	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}
	format, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}
	api := state.NewAICostAPI(sdk)
	size := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if size == 0 {
		size = 20
	}
	data, err := read(helper.GetContext(), api, resource, id, size)
	if err != nil {
		return err
	}
	out := helper.GetStreams().Out
	if values, ok := data.([]state.AICostObject); ok && format == cmdcommon.TEXT && resource.kind != "" {
		table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(table, "ID\tNAME")
		for _, value := range values {
			fmt.Fprintf(table, "%s\t%s\n", value.GetID(), value.GetName())
		}
		return table.Flush()
	}
	if format == cmdcommon.TEXT {
		format = cmdcommon.YAML
	}
	printer, err := cli.Format(format.String(), out)
	if err != nil {
		return err
	}
	defer printer.Flush()
	printer.Print(data)
	return nil
}

func read(ctx context.Context, api *state.AICostAPI, resource readResource, id string, size int64) (any, error) {
	if resource.kind == "" {
		return api.ReadReport(ctx, resource.name, id, size)
	}
	if id == "" {
		return api.List(ctx, resource.kind, size)
	}
	value, err := api.Get(ctx, resource.kind, id)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("%s %s not found", resource.name, id)
	}
	return value, nil
}
