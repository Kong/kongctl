package adopt

import (
	"encoding/json"
	"fmt"
	"strings"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/validator"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
)

const dcrProviderResourceType = "dcr_provider"

func NewDCRProviderCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	cmd := baseCmd
	if cmd == nil {
		cmd = &cobra.Command{}
	}

	cmd.Use = "dcr-provider <dcr-provider-id|dcr-provider-name>"
	cmd.Short = "Adopt an existing Konnect DCR provider into namespace management"
	cmd.Long = "Apply the KONGCTL-namespace label to an existing Konnect DCR provider " +
		"that is not currently managed by kongctl."
	cmd.Aliases = []string{"dcr-providers", "dcrp", "dcrps", "DCRP", "DCRPS"}
	cmd.Args = func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("exactly one DCR provider identifier (name or ID) is required")
		}
		if trimmed := strings.TrimSpace(args[0]); trimmed == "" {
			return fmt.Errorf("DCR provider identifier cannot be empty")
		}
		return nil
	}

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	if parentPreRun != nil {
		cmd.PreRunE = parentPreRun
	}

	cmd.Flags().String(adoptCommon.NamespaceFlagName, "", "Namespace label to apply to the resource")
	if err := cmd.MarkFlagRequired(adoptCommon.NamespaceFlagName); err != nil {
		return nil, err
	}

	cmd.RunE = func(cobraCmd *cobra.Command, args []string) error {
		helper := cmdpkg.BuildHelper(cobraCmd, args)

		namespace, err := cobraCmd.Flags().GetString(adoptCommon.NamespaceFlagName)
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

		result, err := adoptDCRProvider(
			helper,
			sdk.GetDCRProvidersAPI(),
			cfg,
			namespace,
			strings.TrimSpace(args[0]),
		)
		if err != nil {
			return err
		}

		streams := helper.GetStreams()
		if outType == cmdCommon.TEXT {
			name := result.Name
			if name == "" {
				name = result.ID
			}
			fmt.Fprintf(
				streams.Out,
				"Adopted DCR provider %q (%s) into namespace %q\n",
				name,
				result.ID,
				result.Namespace,
			)
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

func adoptDCRProvider(
	helper cmdpkg.Helper,
	api helpers.DCRProvidersAPI,
	cfg config.Hook,
	namespace string,
	identifier string,
) (*adoptCommon.AdoptResult, error) {
	provider, err := resolveDCRProvider(helper, api, cfg, identifier)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(provider.ID) == "" {
		return nil, &cmdpkg.ConfigurationError{
			Err: fmt.Errorf("unable to resolve DCR provider identifier"),
		}
	}

	if currentNamespace, ok := provider.Labels[labels.NamespaceKey]; ok && currentNamespace != "" {
		display := provider.Name
		if display == "" {
			display = provider.ID
		}
		return nil, &cmdpkg.ConfigurationError{
			Err: fmt.Errorf("DCR provider %q already has namespace label %q", display, currentNamespace),
		}
	}

	updateReq, err := buildDCRProviderAdoptRequest(provider.Labels, namespace)
	if err != nil {
		return nil, err
	}

	ctx := adoptCommon.EnsureContext(helper.GetContext())
	if _, err := api.UpdateDcrProvider(ctx, provider.ID, updateReq); err != nil {
		attrs := cmdpkg.TryConvertErrorToAttrs(err)
		return nil, cmdpkg.PrepareExecutionError("failed to update DCR provider", err, helper.GetCmd(), attrs...)
	}

	return &adoptCommon.AdoptResult{
		ResourceType: dcrProviderResourceType,
		ID:           provider.ID,
		Name:         provider.Name,
		Namespace:    namespace,
	}, nil
}

func resolveDCRProvider(
	helper cmdpkg.Helper,
	api helpers.DCRProvidersAPI,
	cfg config.Hook,
	identifier string,
) (*helpers.NormalizedDCRProviderPayload, error) {
	ctx := adoptCommon.EnsureContext(helper.GetContext())

	pageSize := cfg.GetInt(common.RequestPageSizeConfigPath)
	if pageSize < 1 {
		pageSize = common.DefaultRequestPageSize
	}

	pageSizeValue := int64(pageSize)
	isUUID := util.IsValidUUID(identifier)
	var pageNumber int64 = 1

	for {
		req := kkOps.ListDcrProvidersRequest{
			PageSize:   &pageSizeValue,
			PageNumber: &pageNumber,
		}

		res, err := api.ListDcrProviderPayloads(ctx, req)
		if err != nil {
			attrs := cmdpkg.TryConvertErrorToAttrs(err)
			return nil, cmdpkg.PrepareExecutionError("failed to list DCR providers", err, helper.GetCmd(), attrs...)
		}

		if res == nil || len(res.Data) == 0 {
			break
		}

		for _, payload := range res.Data {
			provider, err := helpers.NormalizeDCRProviderPayload(payload)
			if err != nil {
				return nil, err
			}

			if isUUID {
				if provider.ID == identifier {
					return provider, nil
				}
				continue
			}

			if strings.EqualFold(provider.Name, identifier) {
				return provider, nil
			}
		}

		if res.Total <= float64(pageSizeValue*pageNumber) {
			break
		}
		pageNumber++
	}

	return nil, &cmdpkg.ConfigurationError{
		Err: fmt.Errorf("DCR provider %q not found", identifier),
	}
}

func buildDCRProviderAdoptRequest(
	existingLabels map[string]string,
	namespace string,
) (kkComps.UpdateDcrProviderRequest, error) {
	var req kkComps.UpdateDcrProviderRequest

	payloadBytes, err := json.Marshal(map[string]any{
		"labels": adoptCommon.PointerLabelMap(existingLabels, namespace),
	})
	if err != nil {
		return req, fmt.Errorf("failed to build DCR provider adopt request: %w", err)
	}

	if err := json.Unmarshal(payloadBytes, &req); err != nil {
		return req, fmt.Errorf("failed to build DCR provider adopt request: %w", err)
	}

	return req, nil
}
