package adopt

import (
	"fmt"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	declexport "github.com/kong/kongctl/internal/declarative/export"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/spf13/cobra"
)

func NewControlPlaneCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	cmd := baseCmd
	if cmd == nil {
		cmd = &cobra.Command{}
	}

	cmd.Use = "control-plane <control-plane-id|control-plane-name>"
	cmd.Short = "Adopt an existing Konnect control plane into namespace management"
	cmd.Long = "Apply the KONGCTL-namespace label to an existing Konnect control plane " +
		"that is not currently managed by kongctl."
	cmd.Args = func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("exactly one control plane identifier (name or ID) is required")
		}
		if trimmed := strings.TrimSpace(args[0]); trimmed == "" {
			return fmt.Errorf("control plane identifier cannot be empty")
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

		result, err := adoptControlPlane(
			s.Helper,
			s.SDK.GetControlPlaneAPI(),
			s.AdoptFlags.Namespace,
			s.AdoptFlags.OverwriteNamespace,
			strings.TrimSpace(args[0]),
			s.SDK.GetControlPlaneGroupsAPI(),
		)
		return adoptCommon.PrintAdoptionOutcome(s.Helper, s.OutType, result, err, "control plane")
	}

	return cmd, nil
}

func adoptControlPlane(
	helper cmdpkg.Helper,
	controlPlaneAPI helpers.ControlPlaneAPI,
	namespace string,
	overwriteNamespace bool,
	identifier string,
	groupsAPI helpers.ControlPlaneGroupsAPI,
) (*adoptCommon.AdoptResult, error) {
	ctx := adoptCommon.EnsureContext(helper.GetContext())

	id := strings.TrimSpace(identifier)
	if !util.IsValidUUID(id) {
		resolvedID, err := helpers.GetControlPlaneID(ctx, controlPlaneAPI, id)
		if err != nil {
			attrs := cmdpkg.TryConvertErrorToAttrs(err)
			return nil, cmdpkg.PrepareExecutionError("failed to resolve control plane", err, helper.GetCmd(), attrs...)
		}
		id = resolvedID
	}

	res, err := controlPlaneAPI.GetControlPlane(ctx, id)
	if err != nil {
		attrs := cmdpkg.TryConvertErrorToAttrs(err)
		return nil, cmdpkg.PrepareExecutionError("failed to retrieve control plane", err, helper.GetCmd(), attrs...)
	}
	cp := res.GetControlPlane()
	if cp == nil {
		return nil, fmt.Errorf("control plane %s not found", id)
	}

	if existing := cp.Labels; existing != nil && !overwriteNamespace {
		if currentNamespace, ok := existing[labels.NamespaceKey]; ok && currentNamespace != "" {
			return nil, &cmdpkg.ConfigurationError{
				Err: fmt.Errorf("control plane %q already has namespace label %q", cp.Name, currentNamespace),
			}
		}
	}

	updateReq := kkComps.UpdateControlPlaneRequest{
		Labels: adoptCommon.StringLabelMap(cp.Labels, namespace),
	}

	var exportPayload any = *cp
	if adoptCommon.AdoptionExportFromContext(ctx) != nil {
		var members []string
		if cp.Config.ClusterType == kkComps.ControlPlaneClusterTypeClusterTypeControlPlaneGroup {
			if groupsAPI == nil {
				return nil, fmt.Errorf("control plane group membership API is required for declarative export")
			}
			members, err = declexport.FetchControlPlaneGroupMembers(ctx, groupsAPI, cp.ID)
			if err != nil {
				return nil, err
			}
		}
		resource := declexport.MapControlPlaneToDeclarativeResource(*cp, members)
		exportPayload = &resource
	}

	if result, dryRun, err := adoptCommon.PrepareRootAdoption(ctx, adoptCommon.AdoptionChange{
		AdoptResult: adoptCommon.AdoptResult{
			ResourceType: string(resources.ResourceTypeControlPlane),
			ID:           cp.ID,
			Name:         cp.Name,
			Namespace:    namespace,
		},
		MetadataLocation: resources.SchemaFieldLabels,
		Before:           cp.Labels,
		After:            updateReq.Labels,
	}, exportPayload, labels.IsProtectedResource(cp.Labels)); err != nil || dryRun {
		return result, err
	}

	updateRes, err := controlPlaneAPI.UpdateControlPlane(ctx, id, updateReq)
	if err != nil {
		attrs := cmdpkg.TryConvertErrorToAttrs(err)
		return nil, cmdpkg.PrepareExecutionError("failed to update control plane", err, helper.GetCmd(), attrs...)
	}

	updated := updateRes.GetControlPlane()
	if updated == nil {
		return nil, fmt.Errorf("update control plane response missing data")
	}

	ns := adoptCommon.NamespaceFromLabels(updated.Labels, namespace)

	return adoptCommon.FinishAdoption(ctx, &adoptCommon.AdoptResult{
		ResourceType: "control_plane",
		ID:           updated.ID,
		Name:         updated.Name,
		Namespace:    ns,
	})
}
