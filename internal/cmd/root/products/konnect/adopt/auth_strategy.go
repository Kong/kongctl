package adopt

import (
	"fmt"
	"strings"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
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
)

func NewAuthStrategyCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	cmd := baseCmd
	if cmd == nil {
		cmd = &cobra.Command{}
	}

	cmd.Use = "auth-strategy <auth-strategy-id|auth-strategy-name>"
	cmd.Short = "Adopt an existing Konnect auth strategy into namespace management"
	cmd.Long = "Apply the KONGCTL-namespace label to an existing Konnect auth strategy " +
		"that is not currently managed by kongctl."
	cmd.Args = func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("exactly one auth strategy identifier (name or ID) is required")
		}
		if trimmed := strings.TrimSpace(args[0]); trimmed == "" {
			return fmt.Errorf("auth strategy identifier cannot be empty")
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

		result, err := adoptAuthStrategy(
			helper,
			sdk.GetAppAuthStrategiesAPI(),
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
				"Adopted auth strategy %q (%s) into namespace %q\n",
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

func adoptAuthStrategy(
	helper cmdpkg.Helper,
	api helpers.AppAuthStrategiesAPI,
	cfg config.Hook,
	namespace string,
	identifier string,
) (*adoptCommon.AdoptResult, error) {
	strategy, err := resolveAuthStrategy(helper, api, cfg, identifier)
	if err != nil {
		return nil, err
	}

	id, name, existingLabels := authStrategyDetails(*strategy)
	if id == "" {
		return nil, &cmdpkg.ConfigurationError{
			Err: fmt.Errorf("unable to resolve auth strategy identifier"),
		}
	}

	if existingLabels != nil {
		if currentNamespace, ok := existingLabels[labels.NamespaceKey]; ok && currentNamespace != "" {
			display := name
			if display == "" {
				display = id
			}
			return nil, &cmdpkg.ConfigurationError{
				Err: fmt.Errorf("auth strategy %q already has namespace label %q", display, currentNamespace),
			}
		}
	}

	updateReq := kkComps.UpdateAppAuthStrategyRequest{
		Labels: adoptCommon.PointerLabelMap(existingLabels, namespace),
	}

	ctx := adoptCommon.EnsureContext(helper.GetContext())
	if _, err := api.UpdateAppAuthStrategy(ctx, id, updateReq); err != nil {
		attrs := cmdpkg.TryConvertErrorToAttrs(err)
		return nil, cmdpkg.PrepareExecutionError("failed to update auth strategy", err, helper.GetCmd(), attrs...)
	}

	return &adoptCommon.AdoptResult{
		ResourceType: "auth_strategy",
		ID:           id,
		Name:         name,
		Namespace:    namespace,
	}, nil
}

func resolveAuthStrategy(
	helper cmdpkg.Helper,
	api helpers.AppAuthStrategiesAPI,
	cfg config.Hook,
	identifier string,
) (*kkComps.AppAuthStrategy, error) {
	ctx := adoptCommon.EnsureContext(helper.GetContext())

	if util.IsValidUUID(identifier) {
		resp, err := api.GetAppAuthStrategy(ctx, identifier)
		if err != nil {
			attrs := cmdpkg.TryConvertErrorToAttrs(err)
			return nil, cmdpkg.PrepareExecutionError("failed to retrieve auth strategy", err, helper.GetCmd(), attrs...)
		}
		strategy, err := convertCreateResponseToStrategy(resp.GetCreateAppAuthStrategyResponse())
		if err != nil {
			return nil, err
		}
		return strategy, nil
	}

	pageSize := cfg.GetInt(common.RequestPageSizeConfigPath)
	if pageSize < 1 {
		pageSize = common.DefaultRequestPageSize
	}

	var pageNumber int64 = 1
	for {
		req := kkOps.ListAppAuthStrategiesRequest{
			PageSize:   kk.Int64(int64(pageSize)),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := api.ListAppAuthStrategies(ctx, req)
		if err != nil {
			attrs := cmdpkg.TryConvertErrorToAttrs(err)
			return nil, cmdpkg.PrepareExecutionError("failed to list auth strategies", err, helper.GetCmd(), attrs...)
		}

		list := res.GetListAppAuthStrategiesResponse()
		if list == nil || len(list.Data) == 0 {
			break
		}

		for _, strategy := range list.Data {
			if strings.EqualFold(authStrategyName(strategy), identifier) {
				return &strategy, nil
			}
		}

		if len(list.Data) < pageSize {
			break
		}
		pageNumber++
	}

	return nil, &cmdpkg.ConfigurationError{
		Err: fmt.Errorf("auth strategy %q not found", identifier),
	}
}

func authStrategyDetails(strategy kkComps.AppAuthStrategy) (id, name string, lbls map[string]string) {
	if key := strategy.AppAuthStrategyKeyAuthResponseAppAuthStrategyKeyAuthResponse; key != nil {
		return key.ID, key.Name, key.Labels
	}
	if oidc := strategy.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyOpenIDConnectResponse; oidc != nil {
		return oidc.ID, oidc.Name, oidc.Labels
	}
	return "", "", nil
}

func authStrategyName(strategy kkComps.AppAuthStrategy) string {
	_, name, _ := authStrategyDetails(strategy)
	return name
}

func convertCreateResponseToStrategy(resp *kkComps.CreateAppAuthStrategyResponse) (*kkComps.AppAuthStrategy, error) {
	if resp == nil {
		return nil, fmt.Errorf("unexpected nil auth strategy response")
	}

	if resp.AppAuthStrategyKeyAuthResponse != nil {
		return convertKeyAuthResponseToStrategy(resp.AppAuthStrategyKeyAuthResponse), nil
	}

	if resp.AppAuthStrategyOpenIDConnectResponse != nil {
		return convertOpenIDConnectResponseToStrategy(resp.AppAuthStrategyOpenIDConnectResponse), nil
	}

	return nil, fmt.Errorf("unexpected auth strategy response type")
}

func convertKeyAuthResponseToStrategy(key *kkComps.AppAuthStrategyKeyAuthResponse) *kkComps.AppAuthStrategy {
	if key == nil {
		return nil
	}

	// Convert DcrProvider type if present
	var dcrProvider *kkComps.AppAuthStrategyKeyAuthResponseDcrProvider
	if key.DcrProvider != nil {
		dcrProvider = &kkComps.AppAuthStrategyKeyAuthResponseDcrProvider{
			ID:   key.DcrProvider.ID,
			Name: key.DcrProvider.Name,
		}
	}

	strategy := kkComps.CreateAppAuthStrategyKeyAuth(
		kkComps.AppAuthStrategyKeyAuthResponseAppAuthStrategyKeyAuthResponse{
			ID:          key.ID,
			Name:        key.Name,
			DisplayName: key.DisplayName,
			StrategyType: kkComps.AppAuthStrategyKeyAuthResponseAppAuthStrategyStrategyType(
				key.StrategyType),
			Configs: kkComps.AppAuthStrategyKeyAuthResponseAppAuthStrategyConfigs{
				KeyAuth: key.Configs.KeyAuth,
			},
			Active:      key.Active,
			DcrProvider: dcrProvider,
			Labels:      key.Labels,
			CreatedAt:   key.CreatedAt,
			UpdatedAt:   key.UpdatedAt,
		},
	)
	return &strategy
}

func convertOpenIDConnectResponseToStrategy(
	oidc *kkComps.AppAuthStrategyOpenIDConnectResponse,
) *kkComps.AppAuthStrategy {
	if oidc == nil {
		return nil
	}

	// Convert DcrProvider type if present
	var dcrProvider *kkComps.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyDcrProvider
	if oidc.DcrProvider != nil {
		dcrProvider = &kkComps.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyDcrProvider{
			ID:   oidc.DcrProvider.ID,
			Name: oidc.DcrProvider.Name,
		}
	}

	strategy := kkComps.CreateAppAuthStrategyOpenidConnect(
		kkComps.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyOpenIDConnectResponse{
			ID:          oidc.ID,
			Name:        oidc.Name,
			DisplayName: oidc.DisplayName,
			StrategyType: kkComps.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyStrategyType(
				oidc.StrategyType),
			Configs: kkComps.AppAuthStrategyOpenIDConnectResponseAppAuthStrategyConfigs{
				OpenidConnect: oidc.Configs.OpenidConnect,
			},
			Active:      oidc.Active,
			DcrProvider: dcrProvider,
			Labels:      oidc.Labels,
			CreatedAt:   oidc.CreatedAt,
			UpdatedAt:   oidc.UpdatedAt,
		},
	)
	return &strategy
}
