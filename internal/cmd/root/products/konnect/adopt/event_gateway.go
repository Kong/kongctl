package adopt

import (
	"fmt"
	"net/url"
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

func NewEventGatewayControlPlaneCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	cmd := baseCmd
	if cmd == nil {
		cmd = &cobra.Command{}
	}

	cmd.Use = "event-gateway <event-gateway-id|event-gateway-name>"
	cmd.Short = "Adopt an existing Konnect Event Gateway Control Plane into namespace management"
	cmd.Long = "Apply the KONGCTL-namespace label to an existing Konnect Event Gateway Control Plane " +
		"that is not currently managed by kongctl."
	cmd.Args = func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("exactly one event gateway control plane identifier (name or ID) is required")
		}
		if trimmed := strings.TrimSpace(args[0]); trimmed == "" {
			return fmt.Errorf("event gateway control plane identifier cannot be empty")
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

		result, err := adoptEventGatewayControlPlane(
			helper,
			sdk.GetEventGatewayControlPlaneAPI(),
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
				"Adopted Event Gateway Control Plane %q (%s) into namespace %q\n",
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

func adoptEventGatewayControlPlane(
	helper cmdpkg.Helper,
	egwClient helpers.EGWControlPlaneAPI,
	cfg config.Hook,
	namespace string,
	identifier string,
) (*adoptResult, error) {
	egw, err := resolveEventGatewayControlPlane(helper, egwClient, cfg, identifier)
	if err != nil {
		return nil, err
	}

	if existing := egw.Labels; existing != nil {
		if currentNamespace, ok := existing[labels.NamespaceKey]; ok && currentNamespace != "" {
			return nil, &cmdpkg.ConfigurationError{
				Err: fmt.Errorf("event gateway control plane %q already has namespace label %q", egw.Name, currentNamespace),
			}
		}
	}

	updateReq := kkComps.UpdateGatewayRequest{
		Name:        &egw.Name,
		Description: egw.Description,
		Labels:      stringLabelMap(egw.Labels, namespace),
	}

	ctx := ensureContext(helper.GetContext())

	resp, err := egwClient.UpdateEGWControlPlane(ctx, egw.ID, updateReq)
	if err != nil {
		attrs := cmdpkg.TryConvertErrorToAttrs(err)
		return nil, cmdpkg.PrepareExecutionError(
			"failed to update Event Gateway Control Plane",
			err,
			helper.GetCmd(),
			attrs...,
		)
	}

	updated := resp.EventGatewayInfo
	if updated == nil {
		return nil, cmdpkg.PrepareExecutionErrorMsg(helper, "update Event Gateway Control Plane failed")
	}

	ns := namespace
	if updated.Labels != nil {
		if v, ok := updated.Labels[labels.NamespaceKey]; ok && v != "" {
			ns = v
		}
	}

	return &adoptResult{
		ResourceType: "event_gateway",
		ID:           updated.ID,
		Name:         updated.Name,
		Namespace:    ns,
	}, nil
}

func resolveEventGatewayControlPlane(
	helper cmdpkg.Helper,
	egwClient helpers.EGWControlPlaneAPI,
	cfg config.Hook,
	identifier string,
) (*kkComps.EventGatewayInfo, error) {
	ctx := ensureContext(helper.GetContext())

	if util.IsValidUUID(identifier) {
		res, err := egwClient.FetchEGWControlPlane(ctx, identifier)
		if err != nil {
			attrs := cmdpkg.TryConvertErrorToAttrs(err)
			return nil, cmdpkg.PrepareExecutionError(
				"failed to retrieve Event Gateway Control Plane",
				err,
				helper.GetCmd(),
				attrs...,
			)
		}
		egw := res.EventGatewayInfo
		if egw == nil {
			return nil, cmdpkg.PrepareExecutionErrorMsg(
				helper,
				fmt.Sprintf("event gateway control plane %s not found", identifier),
			)
		}
		return egw, nil
	}

	pageSize := cfg.GetInt(common.RequestPageSizeConfigPath)
	if pageSize < 1 {
		pageSize = common.DefaultRequestPageSize
	}

	var pageAfter *string
	for {
		req := kkOps.ListEventGatewaysRequest{
			PageSize: kk.Int64(int64(pageSize)),
		}

		if pageAfter != nil {
			req.PageAfter = pageAfter
		}

		res, err := egwClient.ListEGWControlPlanes(ctx, req)
		if err != nil {
			attrs := cmdpkg.TryConvertErrorToAttrs(err)
			return nil, cmdpkg.PrepareExecutionError(
				"failed to list Event Gateway Control Planes",
				err,
				helper.GetCmd(),
				attrs...,
			)
		}

		list := res.ListEventGatewaysResponse
		if list == nil || len(list.Data) == 0 {
			break
		}

		for _, egw := range list.Data {
			if egw.Name == identifier {
				egwCopy := egw
				return &egwCopy, nil
			}
		}

		if list.Meta.Page.Next == nil {
			break
		}

		// Page.Next contains a full URL; parse it and extract the cursor from
		// the `page[after]` query parameter so we can pass it to the next request.
		u, err := url.Parse(*list.Meta.Page.Next)
		if err != nil {
			attrs := cmdpkg.TryConvertErrorToAttrs(err)
			return nil, cmdpkg.PrepareExecutionError("failed to parse pagination URL", err, helper.GetCmd(), attrs...)
		}

		values := u.Query()
		after := values.Get("page[after]")
		if after == "" {
			break
		}
		// allocate a new string so the pointer remains valid across iterations
		tmp := after
		pageAfter = &tmp
	}

	return nil, &cmdpkg.ConfigurationError{
		Err: fmt.Errorf("event gateway control plane %q not found", identifier),
	}
}
