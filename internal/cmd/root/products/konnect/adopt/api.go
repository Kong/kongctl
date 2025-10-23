package adopt

import (
	"fmt"
	"strings"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/validator"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

func NewAPICmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	cmd := baseCmd
	if cmd == nil {
		cmd = &cobra.Command{}
	}

	cmd.Use = "api <api-id|api-name>"
	cmd.Short = "Adopt an existing Konnect API into namespace management"
	cmd.Long = "Apply the KONGCTL-namespace label to an existing Konnect API " +
		"that is not currently managed by kongctl."
	cmd.Args = func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("exactly one API identifier (name or ID) is required")
		}
		if trimmed := strings.TrimSpace(args[0]); trimmed == "" {
			return fmt.Errorf("API identifier cannot be empty")
		}
		return nil
	}

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	if parentPreRun != nil {
		cmd.PreRunE = parentPreRun
	}

	cmd.Flags().String(NamespaceFlagName, "", "Namespace label to apply to the resource")
	if err := cmd.MarkFlagRequired(NamespaceFlagName); err != nil {
		return nil, err
	}

	cmd.RunE = func(cobraCmd *cobra.Command, args []string) error {
		helper := cmdpkg.BuildHelper(cobraCmd, args)

		namespace, err := cobraCmd.Flags().GetString(NamespaceFlagName)
		if err != nil {
			return err
		}

		nsValidator := validator.NewNamespaceValidator()
		if err := nsValidator.ValidateNamespace(namespace); err != nil {
			return &cmdpkg.ConfigurationError{Err: err}
		}

		outType, err := helper.GetOutputFormat()
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

		result, err := adoptAPI(helper, sdk.GetAPIAPI(), cfg, namespace, strings.TrimSpace(args[0]))
		if err != nil {
			return err
		}

		streams := helper.GetStreams()
		if outType == cmdCommon.TEXT {
			name := result.Name
			if name == "" {
				name = result.ID
			}
			fmt.Fprintf(streams.Out, "Adopted API %q (%s) into namespace %q\n", name, result.ID, result.Namespace)
			return nil
		}

		printer, err := cli.Format(outType.String(), streams.Out)
		if err != nil {
			return err
		}
		defer printer.Flush()
		printer.Print(result)
		return nil
	}

	return cmd, nil
}

func adoptAPI(
	helper cmdpkg.Helper,
	apiClient helpers.APIAPI,
	cfg config.Hook,
	namespace string,
	identifier string,
) (*adoptResult, error) {
	api, err := resolveAPI(helper, apiClient, cfg, identifier)
	if err != nil {
		return nil, err
	}

	if existing := api.Labels; existing != nil {
		if currentNamespace, ok := existing[labels.NamespaceKey]; ok && currentNamespace != "" {
			return nil, &cmdpkg.ConfigurationError{
				Err: fmt.Errorf("API %q already has namespace label %q", api.Name, currentNamespace),
			}
		}
	}

	updateReq := kkComps.UpdateAPIRequest{
		Labels: pointerLabelMap(api.Labels, namespace),
	}

	ctx := ensureContext(helper.GetContext())

	resp, err := apiClient.UpdateAPI(ctx, api.ID, updateReq)
	if err != nil {
		attrs := cmdpkg.TryConvertErrorToAttrs(err)
		return nil, cmdpkg.PrepareExecutionError("failed to update API", err, helper.GetCmd(), attrs...)
	}

	updated := resp.APIResponseSchema
	if updated == nil {
		return nil, cmdpkg.PrepareExecutionErrorMsg(helper, "update API response missing data")
	}

	ns := namespace
	if updated.Labels != nil {
		if v, ok := updated.Labels[labels.NamespaceKey]; ok && v != "" {
			ns = v
		}
	}

	return &adoptResult{
		ResourceType: "api",
		ID:           updated.ID,
		Name:         updated.Name,
		Namespace:    ns,
	}, nil
}

func resolveAPI(
	helper cmdpkg.Helper,
	apiClient helpers.APIAPI,
	cfg config.Hook,
	identifier string,
) (*kkComps.APIResponseSchema, error) {
	ctx := ensureContext(helper.GetContext())

	if util.IsValidUUID(identifier) {
		res, err := apiClient.FetchAPI(ctx, identifier)
		if err != nil {
			attrs := cmdpkg.TryConvertErrorToAttrs(err)
			return nil, cmdpkg.PrepareExecutionError("failed to retrieve API", err, helper.GetCmd(), attrs...)
		}
		api := res.GetAPIResponseSchema()
		if api == nil {
			return nil, cmdpkg.PrepareExecutionErrorMsg(helper, fmt.Sprintf("API %s not found", identifier))
		}
		return api, nil
	}

	pageSize := cfg.GetInt(common.RequestPageSizeConfigPath)
	if pageSize < 1 {
		pageSize = common.DefaultRequestPageSize
	}

	var pageNumber int64 = 1
	for {
		req := kkOps.ListApisRequest{
			PageSize:   kk.Int64(int64(pageSize)),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := apiClient.ListApis(ctx, req)
		if err != nil {
			attrs := cmdpkg.TryConvertErrorToAttrs(err)
			return nil, cmdpkg.PrepareExecutionError("failed to list APIs", err, helper.GetCmd(), attrs...)
		}

		list := res.ListAPIResponse
		if list == nil || len(list.Data) == 0 {
			break
		}

		for _, api := range list.Data {
			if api.Name == identifier {
				apiCopy := api
				return &apiCopy, nil
			}
		}

		if len(list.Data) < pageSize {
			break
		}
		pageNumber++
	}

	return nil, &cmdpkg.ConfigurationError{
		Err: fmt.Errorf("API %q not found", identifier),
	}
}
