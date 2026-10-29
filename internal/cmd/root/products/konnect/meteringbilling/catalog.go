package meteringbilling

import (
	"context"
	"fmt"
	"maps"
	"strconv"
	"strings"

	c "github.com/Kong/sdk-konnect-go/models/components"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	adoptcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/state"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/maturity"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

func addCatalogCommands(family *cobra.Command, verb verbs.VerbValue, configure func(*cobra.Command)) error {
	for _, entry := range []struct {
		kind        helpers.BillingKind
		name, alias string
	}{
		{helpers.BillingTaxCode, "tax-codes", "tax-code"},
		{helpers.BillingCurrency, "currencies", "currency"},
		{helpers.BillingApp, "apps", "app"},
		{helpers.BillingProfile, "profiles", "profile"},
		{helpers.BillingPlan, "plans", "plan"},
	} {
		if entry.kind == helpers.BillingCurrency &&
			(verb == verbs.Update || verb == verbs.Delete || verb == verbs.Adopt) {
			continue
		}
		leaf := &cobra.Command{
			Use:     entry.name + " [identifier]",
			Aliases: []string{entry.alias},
			Short:   "Manage billing " + entry.name,
		}
		configure(leaf)
		// Billing command/action subset.
		//exhaustive:ignore
		switch verb {
		case verbs.Create:
			leaf.Args = cobra.NoArgs
		case verbs.Get, verbs.List:
			leaf.Args = cobra.MaximumNArgs(1)
		default:
			leaf.Args = cobra.ExactArgs(1)
		}
		if verb == verbs.Create || verb == verbs.Update {
			leaf.Flags().String("file", "", "JSON or YAML API request file")
		}
		if verb == verbs.Adopt {
			leaf.Flags().String("namespace", "", "Namespace to adopt into (required)")
			leaf.Flags().Bool("overwrite-namespace", false, "Move an already managed resource into this namespace")
		}
		if verb == verbs.Delete {
			leaf.Flags().Bool("auto-approve", false, "Skip deletion confirmation")
		}
		if verb == verbs.Get || verb == verbs.List {
			if entry.kind == helpers.BillingTaxCode {
				leaf.Flags().Bool("include-deleted", false, "Include soft-deleted tax codes in lists")
			}
			if entry.kind == helpers.BillingCurrency {
				leaf.Flags().String("type", "", "Filter currencies: fiat or custom")
				leaf.Flags().Bool("expand-cost-basis", false, "Include active and scheduled currency cost bases")
			}
		}
		kind := entry.kind
		leaf.RunE = func(cmd *cobra.Command, args []string) error { return runCatalog(cmd, args, verb, kind, "") }
		if kind == helpers.BillingPlan && verb == verbs.Update {
			for _, action := range []string{"publish", "archive"} {
				leaf.Flags().Bool(action, false, "Explicitly "+action+" this plan version (requires ULID; no --file)")
			}
			leaf.MarkFlagsMutuallyExclusive("publish", "archive", "file")
		}
		if kind == helpers.BillingApp && (verb == verbs.Get || verb == verbs.List) {
			catalog := &cobra.Command{
				Use:   "catalog [type]",
				Short: "Inspect available billing app integrations",
				Args:  cobra.MaximumNArgs(1),
			}
			configure(catalog)
			catalog.RunE = func(cmd *cobra.Command, args []string) error { return runCatalog(cmd, args, verb, kind, "catalog") }
			leaf.AddCommand(catalog)
			if err := maturity.AnnotateCommand(catalog, maturity.Metadata{Level: maturity.LevelBeta}); err != nil {
				return err
			}
		}
		if kind == helpers.BillingCurrency && (verb == verbs.Get || verb == verbs.List || verb == verbs.Create) {
			costs := &cobra.Command{
				Use:   "cost-bases <custom-currency-ULID>",
				Short: "Inspect or explicitly schedule custom currency cost bases",
				Args:  cobra.ExactArgs(1),
			}
			configure(costs)
			if verb == verbs.Create {
				costs.Flags().String("file", "", "Cost basis request file (required)")
			}
			costs.RunE = func(cmd *cobra.Command, args []string) error { return runCatalog(cmd, args, verb, kind, "cost-bases") }
			leaf.AddCommand(costs)
		}
		if kind == helpers.BillingCurrency ||
			(kind == helpers.BillingApp &&
				(verb == verbs.Create || verb == verbs.Update || verb == verbs.Delete || verb == verbs.Adopt)) {
			if err := maturity.AnnotateCommand(leaf, maturity.Metadata{Level: maturity.LevelBeta}); err != nil {
				return err
			}
		}
		family.AddCommand(leaf)
	}
	return nil
}

// ResolveBilling accepts ULIDs or unambiguous API keys/codes/names. Plan keys
// require an explicit @version whenever more than one version exists.
func ResolveBilling(
	ctx context.Context,
	api helpers.BillingCatalogAPI,
	kind helpers.BillingKind,
	identifier string,
	size int64,
) (helpers.BillingObject, error) {
	if helpers.IsMeterULID(identifier) {
		return api.Get(ctx, kind, identifier)
	}
	values, err := api.List(ctx, kind, size)
	if err != nil {
		return nil, err
	}
	key := identifier
	var version int64
	if kind == helpers.BillingPlan {
		if k, v, ok := strings.Cut(identifier, "@"); ok {
			version, err = strconv.ParseInt(v, 10, 64)
			if err != nil || version < 1 {
				return nil, fmt.Errorf("plan version must be a positive integer")
			}
			key = k
		}
	}
	var match helpers.BillingObject
	for _, value := range values {
		field := "name"
		// Scoped billing lifecycle classification; other kinds follow the default path.
		//exhaustive:ignore
		switch kind {
		case helpers.BillingTaxCode, helpers.BillingPlan:
			field = "key"
		case helpers.BillingCurrency:
			field = "code"
		}
		if value.String(field) != key {
			continue
		}
		if version != 0 && value["version"] != float64(version) {
			continue
		}
		if match != nil {
			return nil, fmt.Errorf("ambiguous %s %q; use a ULID or plan key@version", kind, identifier)
		}
		match = value
	}
	if match == nil {
		return nil, fmt.Errorf("%s %q not found", kind, identifier)
	}
	return match, nil
}

func runCatalog(
	cmd *cobra.Command,
	args []string,
	verb verbs.VerbValue,
	kind helpers.BillingKind,
	operation string,
) error {
	helper := cmdpkg.BuildHelper(cmd, args)
	format, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}
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
	api := sdk.GetBillingCatalogAPI()
	if api == nil {
		return fmt.Errorf("billing catalog API is not configured")
	}
	size := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if size < 1 {
		size = common.DefaultRequestPageSize
	}
	var result any
	switch {
	case operation == "catalog":
		typ := ""
		if len(args) > 0 {
			typ = args[0]
		}
		result, err = api.Catalog(cmd.Context(), typ, size)
	case operation == "cost-bases":
		if !helpers.IsMeterULID(args[0]) {
			return fmt.Errorf("cost bases require a custom currency ULID")
		}
		if verb == verbs.Create {
			type request c.CreateCostBasisRequest
			var req request
			if err := readRequest(cmd, &req); err != nil {
				return err
			}
			result, err = api.CreateCostBasis(cmd.Context(), args[0], c.CreateCostBasisRequest(req))
		} else {
			result, err = api.CostBases(cmd.Context(), args[0], size)
		}
	case verb == verbs.Get || verb == verbs.List:
		if len(args) == 0 {
			options := helpers.BillingListOptions{}
			if cmd.Flags().Lookup("include-deleted") != nil {
				options.IncludeDeleted, _ = cmd.Flags().GetBool("include-deleted")
			}
			if cmd.Flags().Lookup("type") != nil {
				options.CurrencyType, _ = cmd.Flags().GetString("type")
				options.ExpandCostBasis, _ = cmd.Flags().GetBool("expand-cost-basis")
			}
			result, err = api.List(cmd.Context(), kind, size, options)
		} else {
			result, err = ResolveBilling(cmd.Context(), api, kind, args[0], size)
		}
	case verb == verbs.Create:
		fields, readErr := readCatalogRequest(cmd, kind)
		if readErr != nil {
			return readErr
		}
		if err := validateImperativeMeterLabels(helpers.BillingObject(fields).Labels()); err != nil {
			return err
		}
		if err := helpers.ValidateBillingFields(kind, fields, false); err != nil {
			return err
		}
		result, err = api.Create(cmd.Context(), kind, fields)
	default:
		// Mutations use ULIDs for non-unique names and versioned plans.
		if !helpers.IsMeterULID(args[0]) &&
			(kind == helpers.BillingPlan || kind == helpers.BillingProfile || kind == helpers.BillingApp) {
			return fmt.Errorf("%s mutations require an explicit ULID", kind)
		}
		current, getErr := ResolveBilling(cmd.Context(), api, kind, args[0], size)
		if getErr != nil {
			return getErr
		}
		if labels.IsProtectedResource(current.Labels()) {
			return fmt.Errorf("%s %s is protected", kind, current.ID())
		}
		// Scoped billing lifecycle classification; other kinds follow the default path.
		//exhaustive:ignore
		switch verb {
		case verbs.Delete:
			if kind == helpers.BillingProfile && current["default"] == true {
				return fmt.Errorf("default billing profile cannot be deleted")
			}
			if kind == helpers.BillingPlan && current.String("status") != "draft" {
				return fmt.Errorf("only draft plans can be deleted; archive published versions explicitly")
			}
			if err := cmdpkg.ConfirmDelete(helper, fmt.Sprintf("%s %s", kind, current.ID())); err != nil {
				return err
			}
			client := state.NewClient(state.ClientConfig{BillingCatalogAPI: api})
			resourceType := resources.BillingResourceType(kind)
			if err := client.ValidateBillingDeletion(cmd.Context(), resourceType, current.ID()); err != nil {
				return err
			}
			err = api.Delete(cmd.Context(), kind, current.ID())
			result = map[string]string{"id": current.ID(), "status": "deleted"}
		case verbs.Update:
			action := ""
			for _, name := range []string{"publish", "archive"} {
				if flag := cmd.Flags().Lookup(name); flag != nil && flag.Value.String() == "true" {
					action = name
				}
			}
			if action != "" {
				result, err = api.PlanAction(cmd.Context(), current.ID(), action)
				break
			}
			if kind == helpers.BillingPlan && current.String("status") != "draft" {
				return fmt.Errorf("plan is not a draft; explicitly create a new draft version using create mb plans")
			}
			fields, readErr := readCatalogRequest(cmd, kind)
			if readErr != nil {
				return readErr
			}
			if err := validateImperativeMeterLabels(helpers.BillingObject(fields).Labels()); err != nil {
				return err
			}
			if kind == helpers.BillingApp && fields["type"] != current["type"] {
				return fmt.Errorf("app type is immutable")
			}
			if kind == helpers.BillingTaxCode {
				if key, ok := fields["key"]; ok {
					if key != current["key"] {
						return fmt.Errorf("tax code key is immutable")
					}
					delete(fields, "key")
				}
			}
			owned := helpers.BillingObject(fields).Labels()
			for key, value := range current.Labels() {
				if labels.IsKongctlLabel(key) {
					owned[key] = value
				}
			}
			fields["labels"] = owned
			if err := helpers.ValidateBillingFields(kind, fields, true); err != nil {
				return err
			}
			result, err = api.Update(cmd.Context(), kind, current.ID(), fields)
		case verbs.Adopt:
			flags, readErr := adoptcommon.ReadAdoptFlags(cmd)
			if readErr != nil {
				return readErr
			}
			if labels.IsManagedResource(current.Labels()) && !flags.OverwriteNamespace {
				return fmt.Errorf(
					"resource already belongs to namespace %q; use --overwrite-namespace to move it",
					current.Labels()[labels.NamespaceKey],
				)
			}
			if kind == helpers.BillingPlan && current.String("status") != "draft" {
				return fmt.Errorf("only draft plans can be adopted")
			}
			fields, normalizeErr := helpers.BillingConfiguration(kind, current)
			if normalizeErr != nil {
				return normalizeErr
			}
			// Scoped billing lifecycle classification; other kinds follow the default path.
			//exhaustive:ignore
			switch kind {
			case helpers.BillingTaxCode:
				delete(fields, "key")
			case helpers.BillingProfile:
				delete(fields, "apps")
			case helpers.BillingPlan:
				for _, key := range []string{"key", "currency", "billing_cadence"} {
					delete(fields, key)
				}
			}
			owned := maps.Clone(current.Labels())
			if owned == nil {
				owned = map[string]string{}
			}
			owned[labels.NamespaceKey] = flags.Namespace
			fields["labels"] = owned
			result, err = api.Update(cmd.Context(), kind, current.ID(), fields)
		default:
			return fmt.Errorf("unsupported billing operation %s", verb)
		}
	}
	if err != nil {
		return fmt.Errorf("%s %s: %w", verb, kind, err)
	}
	printer, err := cli.Format(format.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	printer.Print(result)
	printer.Flush()
	return nil
}

func readCatalogRequest(cmd *cobra.Command, kind helpers.BillingKind) (map[string]any, error) {
	var fields map[string]any
	if err := readRequest(cmd, &fields); err != nil {
		if kind == helpers.BillingApp {
			return nil, fmt.Errorf("cannot read billing app request; check --file and its JSON/YAML syntax")
		}
		return nil, err
	}
	return fields, nil
}
