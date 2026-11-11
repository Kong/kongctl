package adopt

import (
	"fmt"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/spf13/cobra"
)

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

	adoptCommon.AddDeclarativeRootExportFlag(cmd)
	cmd.RunE = func(cobraCmd *cobra.Command, args []string) error {
		s, cleanup, err := adoptCommon.SetupExportRun(cobraCmd, args)
		if err != nil {
			return err
		}
		defer cleanup()
		helper, sdk, cfg, adoptFlags, outType := s.Helper, s.SDK, s.Cfg, s.AdoptFlags, s.OutType

		result, err := adoptPortal(
			helper,
			sdk.GetPortalAPI(),
			cfg,
			adoptFlags.Namespace,
			adoptFlags.OverwriteNamespace,
			strings.TrimSpace(args[0]),
		)
		return adoptCommon.PrintAdoptionOutcome(helper, outType, result, err, "portal")
	}

	return cmd, nil
}

func adoptPortal(
	helper cmdpkg.Helper,
	portalAPI helpers.PortalAPI,
	cfg config.Hook,
	namespace string,
	overwriteNamespace bool,
	identifier string,
) (*adoptCommon.AdoptResult, error) {
	portal, err := resolvePortal(helper, portalAPI, cfg, identifier)
	if err != nil {
		return nil, err
	}

	if existing := portal.Labels; existing != nil && !overwriteNamespace {
		if currentNamespace, ok := existing[labels.NamespaceKey]; ok && currentNamespace != "" {
			return nil, &cmdpkg.ConfigurationError{
				Err: fmt.Errorf("portal %q already has namespace label %q", portal.Name, currentNamespace),
			}
		}
	}

	updateReq := kkComps.UpdatePortal{
		Labels: adoptCommon.PointerLabelMap(portal.Labels, namespace),
	}

	ctx := adoptCommon.EnsureContext(helper.GetContext())

	if result, dryRun, err := adoptCommon.PrepareRootAdoption(ctx, adoptCommon.AdoptionChange{
		AdoptResult: adoptCommon.AdoptResult{
			ResourceType: string(resources.ResourceTypePortal),
			ID:           portal.ID,
			Name:         portal.Name,
			Namespace:    namespace,
		},
		MetadataLocation: resources.SchemaFieldLabels,
		Before:           portal.Labels,
		After:            adoptCommon.StringLabelMap(portal.Labels, namespace),
	}, *portal, labels.IsProtectedResource(portal.Labels)); err != nil || dryRun {
		return result, err
	}
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

	return adoptCommon.FinishAdoption(ctx, &adoptCommon.AdoptResult{
		ResourceType: "portal",
		ID:           updated.ID,
		Name:         updated.Name,
		Namespace:    ns,
	})
}

func resolvePortal(
	helper cmdpkg.Helper,
	portalAPI helpers.PortalAPI,
	cfg config.Hook,
	identifier string,
) (*kkComps.PortalResponse, error) {
	ctx := adoptCommon.EnsureContext(helper.GetContext())

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
			PageSize:   new(int64(pageSize)),
			PageNumber: new(pageNumber),
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
						attrs...,
					)
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
