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

const NamespaceFlagName = "namespace"

func NewPortalCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	cmd := baseCmd
	if cmd == nil {
		cmd = &cobra.Command{}
	}

	cmd.Use = "portal <portal-id|portal-name>"
	cmd.Short = "Adopt an existing Konnect portal into namespace management"
	cmd.Long = "Apply the KONGCTL-namespace label to an existing Konnect portal " +
		"that is not currently managed by kongctl."
	cmd.Args = func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("exactly one portal identifier (name or ID) is required")
		}
		if trimmed := strings.TrimSpace(args[0]); trimmed == "" {
			return fmt.Errorf("portal identifier cannot be empty")
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

		result, err := adoptPortal(helper, sdk.GetPortalAPI(), cfg, namespace, strings.TrimSpace(args[0]))
		if err != nil {
			return err
		}

		streams := helper.GetStreams()
		if outType == cmdCommon.TEXT {
			name := result.Name
			if name == "" {
				name = result.ID
			}
			fmt.Fprintf(streams.Out, "Adopted portal %q (%s) into namespace %q\n", name, result.ID, result.Namespace)
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

func adoptPortal(
	helper cmdpkg.Helper,
	portalAPI helpers.PortalAPI,
	cfg config.Hook,
	namespace string,
	identifier string,
) (*adoptResult, error) {
	portal, err := resolvePortal(helper, portalAPI, cfg, identifier)
	if err != nil {
		return nil, err
	}

	if existing := portal.Labels; existing != nil {
		if currentNamespace, ok := existing[labels.NamespaceKey]; ok && currentNamespace != "" {
			return nil, &cmdpkg.ConfigurationError{
				Err: fmt.Errorf("portal %q already has namespace label %q", portal.Name, currentNamespace),
			}
		}
	}

	updateReq := kkComps.UpdatePortal{
		Labels: pointerLabelMap(portal.Labels, namespace),
	}

	ctx := ensureContext(helper.GetContext())

	resp, err := portalAPI.UpdatePortal(ctx, portal.ID, updateReq)
	if err != nil {
		attrs := cmdpkg.TryConvertErrorToAttrs(err)
		return nil, cmdpkg.PrepareExecutionError("failed to update portal", err, helper.GetCmd(), attrs...)
	}

	updated := resp.GetPortalResponse()
	if updated == nil {
		return nil, fmt.Errorf("update portal response missing portal data")
	}

	ns := namespace
	if updated.Labels != nil {
		if v, ok := updated.Labels[labels.NamespaceKey]; ok && v != "" {
			ns = v
		}
	}

	return &adoptResult{
		ResourceType: "portal",
		ID:           updated.ID,
		Name:         updated.Name,
		Namespace:    ns,
	}, nil
}

func resolvePortal(
	helper cmdpkg.Helper,
	portalAPI helpers.PortalAPI,
	cfg config.Hook,
	identifier string,
) (*kkComps.PortalResponse, error) {
	ctx := ensureContext(helper.GetContext())

	if util.IsValidUUID(identifier) {
		resp, err := portalAPI.GetPortal(ctx, identifier)
		if err != nil {
			attrs := cmdpkg.TryConvertErrorToAttrs(err)
			return nil, cmdpkg.PrepareExecutionError("failed to retrieve portal", err, helper.GetCmd(), attrs...)
		}
		portal := resp.GetPortalResponse()
		if portal == nil {
			return nil, fmt.Errorf("portal %s not found", identifier)
		}
		return portal, nil
	}

	pageSize := cfg.GetInt(common.RequestPageSizeConfigPath)
	if pageSize < 1 {
		pageSize = common.DefaultRequestPageSize
	}

	var pageNumber int64 = 1
	for {
		req := kkOps.ListPortalsRequest{
			PageSize:   kk.Int64(int64(pageSize)),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := portalAPI.ListPortals(ctx, req)
		if err != nil {
			attrs := cmdpkg.TryConvertErrorToAttrs(err)
			return nil, cmdpkg.PrepareExecutionError("failed to list portals", err, helper.GetCmd(), attrs...)
		}

		list := res.GetListPortalsResponse()
		if list == nil || len(list.Data) == 0 {
			break
		}

		for _, p := range list.Data {
			if p.Name == identifier {
				portalResp, err := portalAPI.GetPortal(ctx, p.ID)
				if err != nil {
					attrs := cmdpkg.TryConvertErrorToAttrs(err)
					return nil, cmdpkg.PrepareExecutionError(
						"failed to retrieve portal",
						err,
						helper.GetCmd(),
						attrs...)
				}
				portal := portalResp.GetPortalResponse()
				if portal == nil {
					return nil, fmt.Errorf("portal %s not found", p.ID)
				}
				return portal, nil
			}
		}

		if len(list.Data) < pageSize {
			break
		}
		pageNumber++
	}

	return nil, &cmdpkg.ConfigurationError{
		Err: fmt.Errorf("portal %q not found", identifier),
	}
}
