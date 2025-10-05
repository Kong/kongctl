package adopt

import (
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/validator"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/segmentio/cli"
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

		result, err := adoptControlPlane(helper, sdk.GetControlPlaneAPI(), namespace, strings.TrimSpace(args[0]))
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
				"Adopted control plane %q (%s) into namespace %q\n",
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

func adoptControlPlane(
	helper cmdpkg.Helper,
	controlPlaneAPI helpers.ControlPlaneAPI,
	namespace string,
	identifier string,
) (*adoptResult, error) {
	ctx := ensureContext(helper.GetContext())

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

	if existing := cp.Labels; existing != nil {
		if currentNamespace, ok := existing[labels.NamespaceKey]; ok && currentNamespace != "" {
			return nil, &cmdpkg.ConfigurationError{
				Err: fmt.Errorf("control plane %q already has namespace label %q", cp.Name, currentNamespace),
			}
		}
	}

	updateReq := kkComps.UpdateControlPlaneRequest{
		Labels: stringLabelMap(cp.Labels, namespace),
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

	ns := namespace
	if updated.Labels != nil {
		if v, ok := updated.Labels[labels.NamespaceKey]; ok && v != "" {
			ns = v
		}
	}

	return &adoptResult{
		ResourceType: "control_plane",
		ID:           updated.ID,
		Name:         updated.Name,
		Namespace:    ns,
	}, nil
}
