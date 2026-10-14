package meteringbilling

import (
	"fmt"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/maturity"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

func newFeaturesCommand(verb verbs.VerbValue, configure func(*cobra.Command)) *cobra.Command {
	leaf := &cobra.Command{Use: "features [key|ULID]", Aliases: []string{"feature"}, Short: "Manage billing features"}
	configure(leaf)
	//exhaustive:ignore // Scoped imperative verbs only.
	switch verb {
	case verbs.Get, verbs.List:
		leaf.Args = cobra.MaximumNArgs(1)
	case verbs.Create:
		leaf.Args = cobra.NoArgs
	default:
		leaf.Args = cobra.ExactArgs(1)
	}
	if verb == verbs.Create || verb == verbs.Update {
		leaf.Flags().String("file", "", "JSON or YAML API request file (required)")
	}
	if verb == verbs.Delete {
		leaf.Flags().Bool("auto-approve", false, "Skip deletion confirmation")
	}
	leaf.RunE = func(c *cobra.Command, args []string) error { return runFeatures(c, args, verb, false) }
	if verb == verbs.Get {
		query := &cobra.Command{
			Use:   "cost-query <key|ULID>",
			Short: "Query internal operational costs",
			Args:  cobra.ExactArgs(1),
		}
		configure(query)
		query.Flags().String("file", "", "JSON or YAML cost query request file (required)")
		query.RunE = func(c *cobra.Command, args []string) error { return runFeatures(c, args, verb, true) }
		if err := maturity.AnnotateCommand(query, maturity.Metadata{Level: maturity.LevelBeta}); err != nil {
			panic(err)
		}
		leaf.AddCommand(query)
	}
	return leaf
}

func runFeatures(c *cobra.Command, args []string, verb verbs.VerbValue, query bool) error {
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
	api := sdk.GetFeaturesAPI()
	if api == nil {
		return fmt.Errorf("features API not configured")
	}
	pageSize := int64(cfg.GetInt(common.RequestPageSizeConfigPath))
	if pageSize < 1 {
		pageSize = common.DefaultRequestPageSize
	}
	var result any
	if verb == verbs.Create {
		var req helpers.FeatureCreateRequest
		if err := readRequest(c, &req); err != nil {
			return err
		}
		create, err := req.SDKRequest()
		if err != nil {
			return err
		}
		if err := validateImperativeMeterLabels(create.Labels); err != nil {
			return err
		}
		if err := helpers.ValidateFeatureCreate(create); err != nil {
			return err
		}
		resp, err := api.CreateFeature(c.Context(), create)
		if err != nil {
			return fmt.Errorf("create feature: %w", err)
		}
		if resp == nil || resp.Feature == nil {
			return fmt.Errorf("create feature response missing data")
		}
		result = resp.Feature
	} else if (verb == verbs.Get || verb == verbs.List) && len(args) == 0 {
		values, err := helpers.ListAllFeatures(c.Context(), api, pageSize)
		if err != nil {
			return err
		}
		result = values
	} else {
		feature, err := helpers.ResolveFeature(c.Context(), api, args[0], pageSize)
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
			resp, err := api.QueryFeatureCost(c.Context(), feature.ID, (*kkComps.MeterQueryRequest)(&req))
			if err != nil {
				return fmt.Errorf("query feature: %w", err)
			}
			if resp == nil || resp.FeatureCostQueryResult == nil {
				return fmt.Errorf("query feature response missing data")
			}
			result = resp.FeatureCostQueryResult
		case verb == verbs.Update:
			if labels.IsProtectedResource(feature.Labels) {
				return fmt.Errorf("feature %q is protected", feature.Key)
			}
			var req helpers.FeatureUpdateRequest
			if err := readRequest(c, &req); err != nil {
				return err
			}
			if err := req.Validate(); err != nil {
				return err
			}
			resp, err := api.UpdateFeature(c.Context(), feature.ID, req)
			if err != nil {
				return fmt.Errorf("update feature: %w", err)
			}
			if resp == nil || resp.Feature == nil {
				return fmt.Errorf("update feature response missing data")
			}
			result = resp.Feature
		case verb == verbs.Adopt:
			return fmt.Errorf("feature adoption is unsupported: PATCH accepts only unit_cost and cannot " +
				"update ownership labels; create a feature with a new key")
		case verb == verbs.Delete:
			if labels.IsProtectedResource(feature.Labels) {
				return fmt.Errorf("feature %q is protected", feature.Key)
			}
			if err := cmdpkg.ConfirmDelete(helper, fmt.Sprintf("feature %q", feature.Key)); err != nil {
				return err
			}
			if _, err := api.DeleteFeature(c.Context(), feature.ID); err != nil {
				return fmt.Errorf("delete feature: %w", err)
			}
			result = map[string]string{"id": feature.ID, "key": feature.Key, "status": "deleted"}
		default:
			result = feature
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
