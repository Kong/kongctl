package meteringbilling

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products"
	adoptcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"

	"sigs.k8s.io/yaml"
)

const CommandName = "metering-billing"

// NewCommand establishes the shared M&B namespace for subsequent resource families.
func NewCommand(verb verbs.VerbValue) (*cobra.Command, error) {
	family := &cobra.Command{
		Use:     CommandName,
		Aliases: []string{"mb"},
		Short:   "Manage Konnect Metering & Billing resources",
	}
	leaf := &cobra.Command{Use: "meters [key|ULID]", Aliases: []string{"meter"}, Short: "Manage usage meters"}
	// Only M&B imperative verbs add commands.
	//exhaustive:ignore
	switch verb {
	case verbs.Get, verbs.List:
		leaf.Args = cobra.MaximumNArgs(1)
	case verbs.Create:
		leaf.Args = cobra.NoArgs
	case verbs.Update, verbs.Delete, verbs.Adopt:
		leaf.Args = cobra.ExactArgs(1)
	default:
		return nil, nil
	}
	configure := func(c *cobra.Command) {
		c.Flags().String(common.BaseURLFlagName, "", "Konnect API base URL")
		c.Flags().String(common.RegionFlagName, "", "Konnect region")
		c.Flags().String(common.PATFlagName, "", "Konnect personal access token")
		c.Flags().Int(common.RequestPageSizeFlagName, common.DefaultRequestPageSize, "Items per API page")
		c.PreRunE = func(c *cobra.Command, args []string) error {
			// Preserve explicitly supplied ancestor flags when this scoped command
			// also declares the flag for direct and explicit-Konnect use.
			for _, name := range []string{
				common.BaseURLFlagName, common.RegionFlagName, common.PATFlagName,
				common.RequestPageSizeFlagName, "namespace", "overwrite-namespace", "auto-approve",
			} {
				local := c.Flags().Lookup(name)
				if local == nil || local.Changed {
					continue
				}
				for parent := c.Parent(); parent != nil; parent = parent.Parent() {
					inherited := parent.PersistentFlags().Lookup(name)
					if inherited != nil && inherited.Changed {
						if err := c.Flags().Set(name, inherited.Value.String()); err != nil {
							return err
						}
						break
					}
				}
			}

			ctx := context.WithValue(c.Context(), products.Product, products.ProductKonnect)
			ctx = context.WithValue(ctx, verbs.Verb, verb)
			ctx = context.WithValue(ctx, helpers.SDKAPIFactoryKey, common.GetSDKFactoryForVerb(verb))
			c.SetContext(ctx)
			helper := cmdpkg.BuildHelper(c, args)
			cfg, err := helper.GetConfig()
			if err != nil {
				return err
			}
			for _, binding := range []struct{ flag, path string }{
				{common.BaseURLFlagName, common.BaseURLConfigPath},
				{common.RegionFlagName, common.RegionConfigPath},
				{common.PATFlagName, common.PATConfigPath},
				{common.RequestPageSizeFlagName, common.RequestPageSizeConfigPath},
			} {
				if err := verbs.BindFlag(cfg, c.Flags(), binding.flag, binding.path); err != nil {
					return err
				}
			}
			if verb == verbs.Delete {
				approved, _ := c.Flags().GetBool("auto-approve")
				cmdpkg.SetDeleteAutoApprove(c, approved)
			}
			return nil
		}
	}
	configure(leaf)
	if verb == verbs.Create || verb == verbs.Update {
		leaf.Flags().String("file", "", "JSON or YAML API request file (required)")
	}
	if verb == verbs.Adopt {
		leaf.Flags().String("namespace", "", "Namespace to adopt into (required)")
		leaf.Flags().Bool("overwrite-namespace", false, "Move an already managed meter to this namespace")
	}
	if verb == verbs.Delete {
		leaf.Flags().Bool("auto-approve", false, "Skip deletion confirmation")
	}
	leaf.RunE = func(c *cobra.Command, args []string) error { return runMeters(c, args, verb, false) }
	if verb == verbs.Get {
		query := &cobra.Command{
			Use:   "query <key|ULID>",
			Short: "Query operational usage (outside desired state)",
			Args:  cobra.ExactArgs(1),
		}
		configure(query)
		query.Flags().String("file", "", "JSON or YAML meter query request file (required)")
		query.RunE = func(c *cobra.Command, args []string) error { return runMeters(c, args, verb, true) }
		leaf.AddCommand(query)
	}
	family.AddCommand(leaf)
	features, err := newFeaturesCommand(verb, configure)
	if err != nil {
		return nil, err
	}
	family.AddCommand(features)
	if err := addCatalogCommands(family, verb, configure); err != nil {
		return nil, err
	}
	return family, nil
}

func readRequest(c *cobra.Command, target any) error {
	path, _ := c.Flags().GetString("file")
	if path == "" {
		return fmt.Errorf("--file is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read request file: %w", err)
	}
	data, err = yaml.YAMLToJSONStrict(data)
	if err != nil {
		return fmt.Errorf("parse request file: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid API request: %w", err)
	}
	return nil
}

func runMeters(c *cobra.Command, args []string, verb verbs.VerbValue, query bool) error {
	helper := cmdpkg.BuildHelper(c, args)
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
	api := sdk.GetMetersAPI()
	if api == nil {
		return fmt.Errorf("meters API not configured")
	}
	pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if pageSize < 1 {
		pageSize = common.DefaultRequestPageSize
	}
	var result any
	if verb == verbs.Create {
		type request kkComps.CreateMeterRequest
		var req request
		if err := readRequest(c, &req); err != nil {
			return err
		}
		create := kkComps.CreateMeterRequest(req)
		if err := validateImperativeMeterLabels(create.Labels); err != nil {
			return err
		}
		if err := helpers.ValidateMeterCreate(create); err != nil {
			return err
		}
		resp, err := api.CreateMeter(c.Context(), create)
		if err != nil {
			return fmt.Errorf("create meter: %w", err)
		}
		if resp == nil || resp.Meter == nil {
			return fmt.Errorf("create meter response missing data")
		}
		result = resp.Meter
	} else if (verb == verbs.Get || verb == verbs.List) && len(args) == 0 {
		values, err := helpers.ListAllMeters(c.Context(), api, pageSize)
		if err != nil {
			return err
		}
		result = values
	} else {
		meter, err := helpers.ResolveMeter(c.Context(), api, args[0], pageSize)
		if err != nil {
			return err
		}
		switch {
		case query:
			type request kkComps.MeterQueryRequest
			var req request
			if err := readRequest(c, &req); err != nil {
				return err
			}
			resp, err := api.QueryMeter(c.Context(), meter.ID, kkComps.MeterQueryRequest(req))
			if err != nil {
				return fmt.Errorf("query meter: %w", err)
			}
			if resp == nil || resp.MeterQueryResult == nil {
				return fmt.Errorf("query meter response missing data")
			}
			result = resp.MeterQueryResult
		case verb == verbs.Update:
			if labels.IsProtectedResource(meter.Labels) {
				return fmt.Errorf("meter %q is protected", meter.Key)
			}
			var req kkComps.UpdateMeterRequest
			if err := readRequest(c, &req); err != nil {
				return err
			}
			if err := validateImperativeMeterLabels(req.Labels); err != nil {
				return err
			}
			if req.Labels != nil {
				for key, value := range meter.Labels {
					if labels.IsKongctlLabel(key) {
						req.Labels[key] = value
					}
				}
			}
			if err := helpers.ValidateMeterUpdate(req); err != nil {
				return err
			}
			resp, err := api.UpdateMeter(c.Context(), meter.ID, req)
			if err != nil {
				return fmt.Errorf("update meter: %w", err)
			}
			if resp == nil || resp.Meter == nil {
				return fmt.Errorf("update meter response missing data")
			}
			result = resp.Meter
		case verb == verbs.Adopt:
			flags, err := adoptcommon.ReadAdoptFlags(c)
			if err != nil {
				return err
			}
			if labels.IsProtectedResource(meter.Labels) {
				return fmt.Errorf("meter %q is protected; unprotect it through declarative management before adoption", meter.Key)
			}
			if labels.IsManagedResource(meter.Labels) && !flags.OverwriteNamespace {
				return fmt.Errorf(
					"meter %q already belongs to namespace %q; use --overwrite-namespace to move it",
					meter.Key, meter.Labels[labels.NamespaceKey],
				)
			}
			owned := maps.Clone(meter.Labels)
			if owned == nil {
				owned = map[string]string{}
			}
			owned[labels.NamespaceKey] = flags.Namespace
			if err := helpers.ValidateMeterLabels(owned); err != nil {
				return err
			}
			resp, err := api.UpdateMeter(c.Context(), meter.ID, kkComps.UpdateMeterRequest{Labels: owned})
			if err != nil {
				return fmt.Errorf("adopt meter: %w", err)
			}
			if resp == nil || resp.Meter == nil {
				return fmt.Errorf("adopt meter response missing data")
			}
			result = resp.Meter
		case verb == verbs.Delete:
			if labels.IsProtectedResource(meter.Labels) {
				return fmt.Errorf("meter %q is protected", meter.Key)
			}
			if err := cmdpkg.ConfirmDelete(helper, fmt.Sprintf("meter %q", meter.Key)); err != nil {
				return err
			}
			if _, err := api.DeleteMeter(c.Context(), meter.ID); err != nil {
				return fmt.Errorf("delete meter: %w", err)
			}
			result = map[string]string{"id": meter.ID, "key": meter.Key, "status": "deleted"}
		default:
			result = meter
		}
	}
	printer, err := cli.Format(format.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	printer.Print(result)
	printer.Flush()
	return nil
}

func validateImperativeMeterLabels(values map[string]string) error {
	for key := range values {
		if labels.IsKongctlLabel(key) {
			return fmt.Errorf(
				"management label %q cannot be supplied in an imperative request; use adopt or declarative kongctl metadata",
				key,
			)
		}
	}
	return nil
}
