package adopt

import (
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/spf13/cobra"
)

func NewAIGatewayCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	cmd := baseCmd
	if cmd == nil {
		cmd = &cobra.Command{}
	}

	cmd.Use = "ai-gateway <ai-gateway-id|ai-gateway-display-name>"
	cmd.Aliases = []string{"ai-gateways", "aigw"}
	cmd.Short = "Adopt an existing Konnect AI Gateway into namespace management"
	cmd.Long = "Apply the KONGCTL-namespace label to an existing Konnect AI Gateway " +
		"that is not currently managed by kongctl."
	cmd.Args = func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("exactly one AI Gateway identifier (display name or ID) is required")
		}
		if trimmed := strings.TrimSpace(args[0]); trimmed == "" {
			return fmt.Errorf("AI Gateway identifier cannot be empty")
		}
		return nil
	}

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	if parentPreRun != nil {
		cmd.PreRunE = parentPreRun
	}

	cmd.RunE = func(cobraCmd *cobra.Command, args []string) error {
		s, err := adoptCommon.SetupAdoptRun(cobraCmd, args)
		if err != nil {
			return err
		}

		result, err := adoptAIGateway(
			s.Helper,
			s.SDK.GetAIGatewayAPI(),
			s.Cfg,
			s.AdoptFlags.Namespace,
			s.AdoptFlags.OverwriteNamespace,
			strings.TrimSpace(args[0]),
		)
		if err != nil {
			return err
		}

		return adoptCommon.PrintAdoptResult(s.Helper, s.OutType, result, "AI Gateway")
	}

	return cmd, nil
}

func adoptAIGateway(
	helper cmdpkg.Helper,
	api helpers.AIGatewayAPI,
	cfg config.Hook,
	namespace string,
	overwriteNamespace bool,
	identifier string,
) (*adoptCommon.AdoptResult, error) {
	gateway, err := resolveAIGateway(helper, api, cfg, identifier)
	if err != nil {
		return nil, err
	}

	if existing := gateway.Labels; existing != nil && !overwriteNamespace {
		if currentNamespace, ok := existing[labels.NamespaceKey]; ok && currentNamespace != "" {
			return nil, &cmdpkg.ConfigurationError{
				Err: fmt.Errorf("AI Gateway %q already has namespace label %q", gateway.DisplayName, currentNamespace),
			}
		}
	}
	if gateway.ID == "" {
		return nil, fmt.Errorf("AI Gateway %q is missing an ID", gateway.DisplayName)
	}

	updateReq := kkComps.UpdateAIGatewayRequest{
		DisplayName: gateway.DisplayName,
		Name:        gateway.Name,
		Description: gateway.Description,
		ProxyUrls:   gateway.ProxyUrls,
		Labels:      adoptCommon.StringLabelMap(gateway.Labels, namespace),
	}

	ctx := adoptCommon.EnsureContext(helper.GetContext())
	resp, err := api.UpdateAiGateway(ctx, gateway.ID, updateReq)
	if err != nil {
		attrs := cmdpkg.TryConvertErrorToAttrs(err)
		return nil, cmdpkg.PrepareExecutionError("failed to update AI Gateway", err, helper.GetCmd(), attrs...)
	}

	updated := resp.GetAIGateway()
	if updated == nil {
		return nil, fmt.Errorf("update AI Gateway response missing AI Gateway data")
	}

	ns := namespace
	if updated.Labels != nil {
		if v, ok := updated.Labels[labels.NamespaceKey]; ok && v != "" {
			ns = v
		}
	}

	return &adoptCommon.AdoptResult{
		ResourceType: string(resources.ResourceTypeAIGateway),
		ID:           updated.ID,
		Name:         updated.DisplayName,
		Namespace:    ns,
	}, nil
}

func resolveAIGateway(
	helper cmdpkg.Helper,
	api helpers.AIGatewayAPI,
	cfg config.Hook,
	identifier string,
) (*kkComps.AIGateway, error) {
	if api == nil {
		return nil, fmt.Errorf("AI Gateway API client is not configured")
	}

	ctx := adoptCommon.EnsureContext(helper.GetContext())

	if util.IsValidUUID(identifier) {
		resp, err := api.GetAiGateway(ctx, identifier)
		if err != nil {
			attrs := cmdpkg.TryConvertErrorToAttrs(err)
			return nil, cmdpkg.PrepareExecutionError("failed to retrieve AI Gateway", err, helper.GetCmd(), attrs...)
		}
		gateway := resp.GetAIGateway()
		if gateway == nil {
			return nil, fmt.Errorf("AI Gateway %s not found", identifier)
		}
		return gateway, nil
	}

	pageSize := common.ResolveRequestPageSize(cfg)
	pageNumber := int64(1)
	var matches []kkComps.AIGateway

	for {
		resp, err := api.ListAiGateways(ctx, &pageSize, &pageNumber)
		if err != nil {
			attrs := cmdpkg.TryConvertErrorToAttrs(err)
			return nil, cmdpkg.PrepareExecutionError("failed to list AI Gateways", err, helper.GetCmd(), attrs...)
		}

		if resp == nil {
			break
		}
		list := resp.ListAIGatewaysResponse
		if list == nil || len(list.Data) == 0 {
			break
		}

		for _, gateway := range list.Data {
			if gateway.DisplayName == identifier {
				matches = append(matches, gateway)
			}
		}

		if !common.HasMorePageNumberResults(int(list.Meta.Page.Total), int(pageNumber*pageSize), len(list.Data)) {
			break
		}
		pageNumber++
	}

	switch len(matches) {
	case 0:
		return nil, &cmdpkg.ConfigurationError{Err: fmt.Errorf("AI Gateway %q not found", identifier)}
	case 1:
		return &matches[0], nil
	default:
		return nil, &cmdpkg.ConfigurationError{
			Err: fmt.Errorf("AI Gateway display name %q matches %d AI Gateways; use the AI Gateway ID",
				identifier, len(matches)),
		}
	}
}
