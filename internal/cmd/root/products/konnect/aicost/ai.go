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
	alias       string
	description string
	report      string
	kind        resources.ResourceType
	detail      bool
	budgetScope budgetScope
}

type budgetScope int

const (
	unscoped budgetScope = iota
	optionalBudget
	requiredBudget
)

const budgetIDFlag = "budget-id"

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
	newCommand := func(resource readResource) *cobra.Command {
		return newReadCommand(verb, flags, preRun, resource)
	}
	budget := newCommand(readResource{
		name: "budget", alias: "budgets", description: "List or get AI cost budgets",
		kind: resources.ResourceTypeAICostBudget, detail: true,
	})
	balance := readResource{
		name: "balance", alias: "balances", description: "List budget balances", report: "budget-balances",
	}
	rule := &cobra.Command{Use: "notification-rule", Short: "Inspect a limit budget's notification rule"}
	if verb == verbs.Get {
		balance.description = "List budget balances or get a balance for one budget"
		balance.budgetScope = optionalBudget
		rule = newCommand(readResource{
			name: "notification-rule", description: "Get a limit budget's notification rule",
			kind: resources.ResourceTypeAICostRule, budgetScope: requiredBudget,
		})
	} else {
		cmdpkg.ConfigureRequiresSubcommand(rule)
	}
	rule.AddCommand(newCommand(readResource{
		name: "firing", alias: "firings", description: "List threshold firings for a limit budget",
		report: "notification-firings", budgetScope: requiredBudget,
	}))
	budget.AddCommand(newCommand(balance), rule)
	cost.AddCommand(budget)

	notifications := &cobra.Command{Use: "notifications", Short: "Inspect budget alert destinations and history"}
	cmdpkg.ConfigureRequiresSubcommand(notifications)
	notifications.AddCommand(
		newCommand(readResource{
			name: "channels", description: "List or get budget alert destinations",
			kind: resources.ResourceTypeAICostChannel, detail: true,
		}),
		newCommand(readResource{
			name: "events", description: "List or get AI cost notification events",
			report: "notification-events", detail: true,
		}),
	)
	cost.AddCommand(notifications)

	for _, resource := range []readResource{
		{
			name: "export-schedules", description: "List or get recurring cost exports",
			kind: resources.ResourceTypeAICostSchedule, detail: true,
		},
		{name: "exports", description: "List or get usage export jobs", detail: true},
		{name: "usage-events", description: "List AI usage events"},
		{name: "forecast", description: "Get the monthly AI cost forecast"},
	} {
		cost.AddCommand(newCommand(resource))
	}
	return ai
}

func newReadCommand(verb verbs.VerbValue, flags func(verbs.VerbValue, *cobra.Command),
	preRun func(*cobra.Command, []string) error, resource readResource,
) *cobra.Command {
	command := &cobra.Command{Use: resource.name, Short: resource.description, Args: cobra.NoArgs, PreRunE: preRun}
	if resource.alias != "" {
		command.Aliases = []string{resource.alias}
	}
	if resource.detail && verb == verbs.Get {
		command.Use += " [id]"
		command.Args = cobra.MaximumNArgs(1)
	}
	if flags != nil {
		flags(verb, command)
	}
	if resource.budgetScope != unscoped {
		command.Flags().String(budgetIDFlag, "", "Budget UUID")
		if resource.budgetScope == requiredBudget {
			_ = command.MarkFlagRequired(budgetIDFlag)
		}
	}
	command.RunE = func(c *cobra.Command, args []string) error {
		var id string
		if resource.budgetScope != unscoped {
			var err error
			id, err = c.Flags().GetString(budgetIDFlag)
			if err != nil {
				return err
			}
			if (resource.budgetScope == requiredBudget || c.Flags().Changed(budgetIDFlag)) && !util.IsValidUUID(id) {
				return fmt.Errorf("--%s requires a UUID", budgetIDFlag)
			}
		} else if len(args) > 0 {
			id = args[0]
			if !util.IsValidUUID(id) {
				return fmt.Errorf("%s requires a UUID", resource.name)
			}
		}
		return run(c, args, resource, id)
	}
	return command
}

func run(command *cobra.Command, args []string, resource readResource, id string) error {
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
		report := resource.report
		if report == "" {
			report = resource.name
		}
		return api.ReadReport(ctx, report, id, size)
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
