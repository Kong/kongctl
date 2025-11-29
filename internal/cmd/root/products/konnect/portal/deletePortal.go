package portal

import (
	"fmt"
	"strings"

	kk "github.com/Kong/sdk-konnect-go"
	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

var (
	deletePortalShort = i18n.T("root.products.konnect.portal.deletePortalShort",
		"Delete a Konnect portal")
	deletePortalLong = i18n.T("root.products.konnect.portal.deletePortalLong",
		`Delete a portal by ID or name.

If the portal has published APIs, the deletion will fail unless the --force flag is used.
Using --force will delete the portal along with all API publications.

Use --approve to skip the confirmation prompt.`)
	deletePortalExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.deletePortalExamples",
			fmt.Sprintf(`
	# Delete a portal by ID
	%[1]s delete portal 12345678-1234-1234-1234-123456789012

	# Delete a portal by name
	%[1]s delete portal my-portal

	# Force delete a portal with published APIs
	%[1]s delete portal my-portal --force

	# Delete without confirmation prompt
	%[1]s delete portal my-portal --approve

	`, meta.CLIName)))
)

type deletePortalCmd struct {
	*cobra.Command
}

func (c *deletePortalCmd) validate(helper cmd.Helper) error {
	args := helper.GetArgs()
	if len(args) == 0 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("portal ID or name is required"),
		}
	}
	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Deleting a portal requires exactly 1 argument (name or ID)"),
		}
	}
	return nil
}

func (c *deletePortalCmd) runE(cobraCmd *cobra.Command, args []string) error {
	var e error
	helper := cmd.BuildHelper(cobraCmd, args)
	if e = c.validate(helper); e != nil {
		return e
	}

	logger, e := helper.GetLogger()
	if e != nil {
		return e
	}

	cfg, e := helper.GetConfig()
	if e != nil {
		return e
	}

	sdk, e := helper.GetKonnectSDK(cfg, logger)
	if e != nil {
		return e
	}

	// Get portal ID (resolve name if necessary)
	portalID := strings.TrimSpace(args[0])
	var portalName string

	// Check if argument is UUID
	isUUID := util.IsValidUUID(portalID)

	if !isUUID {
		// Resolve name to ID
		logger.Debug(fmt.Sprintf("Resolving portal name '%s' to ID", portalID))
		resolvedPortal, err := c.resolvePortalByName(portalID, sdk.GetPortalAPI(), helper, nil)
		if err != nil {
			return err
		}
		portalID = resolvedPortal.GetID()
		portalName = resolvedPortal.GetName()
	} else {
		// Get portal details for confirmation
		portalResponse, err := sdk.GetPortalAPI().GetPortal(helper.GetContext(), portalID)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			msg := common.BuildDetailedMessage("Failed to get portal details", attrs, err)
			return cmd.PrepareExecutionError(msg, err, helper.GetCmd(), attrs...)
		}
		if portalResponse.GetPortalResponse() == nil {
			return cmd.PrepareExecutionErrorMsg(helper, fmt.Sprintf("portal not found: %s", portalID))
		}
		pr := portalResponse.GetPortalResponse()
		portalName = pr.GetName()
	}

	forceDelete := cmd.DeleteForceEnabled(helper)
	labelWidth := len("Name")
	warnings := []string{
		formatPortalDetail("Name", portalName, labelWidth),
		formatPortalDetail("ID", portalID, labelWidth),
	}
	if !forceDelete {
		warnings = append(warnings,
			"Note: If this portal has published APIs, use --force to delete them along with the portal.")
	}

	if err := cmd.ConfirmDelete(
		helper,
		fmt.Sprintf("portal %q", portalName),
		warnings...,
	); err != nil {
		return err
	}

	// Delete the portal
	logger.Info(fmt.Sprintf("Deleting portal '%s' (ID: %s)", portalName, portalID))

	_, err := sdk.GetPortalAPI().DeletePortal(helper.GetContext(), portalID, forceDelete)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		apiErrDetails := common.ParseAPIErrorDetails(err)
		attrs = common.AppendAPIErrorAttrs(attrs, apiErrDetails)
		msg := common.BuildDetailedMessage("Failed to delete portal", attrs, err)

		if !forceDelete && shouldSuggestForce(apiErrDetails, err) {
			attrs = common.AppendIfMissingAttr(attrs, "suggestion", "Use --force to delete portal with published APIs")
		}

		return cmd.PrepareExecutionError(msg, err, helper.GetCmd(), attrs...)
	}

	// Format and output response
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	// Create success response
	response := map[string]any{
		"id":      portalID,
		"name":    portalName,
		"status":  "deleted",
		"message": fmt.Sprintf("Portal '%s' deleted successfully", portalName),
	}

	if outType == cmdCommon.TEXT {
		// For text output, just print the success message
		fmt.Fprintf(helper.GetStreams().Out, "Portal '%s' deleted successfully\n", portalName)
	} else {
		// For JSON/YAML output, print the structured response
		printer.Print(response)
	}

	return nil
}

func (c *deletePortalCmd) resolvePortalByName(
	name string,
	api helpers.PortalAPI,
	helper cmd.Helper,
	_ config.Hook,
) (*kkComps.ListPortalsResponsePortal, error) {
	pageSize := int64(common.DefaultRequestPageSize)
	pageNumber := int64(1)

	for {
		req := kkOps.ListPortalsRequest{
			PageSize:   kk.Int64(pageSize),
			PageNumber: kk.Int64(pageNumber),
		}

		res, err := api.ListPortals(helper.GetContext(), req)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return nil, cmd.PrepareExecutionError("Failed to list portals", err, c.Command, attrs...)
		}

		listResponse := res.GetListPortalsResponse()
		if listResponse == nil || listResponse.Data == nil {
			break
		}

		// Look for exact name match
		var matches []*kkComps.ListPortalsResponsePortal
		for _, portal := range listResponse.Data {
			if portal.Name == name {
				// Create a copy to avoid pointer issues
				p := portal
				matches = append(matches, &p)
			}
		}

		if len(matches) > 1 {
			return nil, cmd.PrepareExecutionErrorMsg(helper,
				fmt.Sprintf("multiple portals found with name '%s'. Please use ID instead", name))
		}

		if len(matches) == 1 {
			return matches[0], nil
		}

		// Check if there are more pages
		totalItems := listResponse.Meta.Page.Total
		if len(listResponse.Data) >= int(totalItems) {
			break
		}

		pageNumber++
	}

	return nil, cmd.PrepareExecutionErrorMsg(helper, fmt.Sprintf("portal not found: %s", name))
}

func shouldSuggestForce(details *common.APIErrorDetails, err error) bool {
	if details != nil {
		for _, param := range details.InvalidParameters {
			if strings.EqualFold(strings.TrimSpace(param.Field), "force") {
				return true
			}
		}
	}

	return err != nil && (strings.Contains(err.Error(), "published") || strings.Contains(err.Error(), "API"))
}

func formatPortalDetail(label, value string, width int) string {
	if width < len(label) {
		width = len(label)
	}
	return fmt.Sprintf("  %-*s %s", width+1, label+":", value)
}

func newDeletePortalCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *deletePortalCmd {
	rv := deletePortalCmd{
		Command: baseCmd,
	}

	rv.Short = deletePortalShort
	rv.Long = deletePortalLong
	rv.Example = deletePortalExample
	rv.Args = cobra.ExactArgs(1)

	if parentPreRun != nil {
		rv.PreRunE = parentPreRun
	}
	rv.RunE = rv.runE

	if addParentFlags != nil {
		addParentFlags(verb, rv.Command)
	}

	if applicationsCmd := newDeletePortalApplicationsCmd(verb, addParentFlags, parentPreRun); applicationsCmd != nil {
		rv.AddCommand(applicationsCmd)
	}

	if registrationsCmd := newDeletePortalApplicationRegistrationsCmd(
		verb,
		addParentFlags,
		parentPreRun,
	); registrationsCmd != nil {
		rv.AddCommand(registrationsCmd)
	}

	return &rv
}
