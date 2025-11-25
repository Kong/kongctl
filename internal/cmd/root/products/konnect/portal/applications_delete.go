package portal

import (
	"fmt"
	"strings"

	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	konnectCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"

	"github.com/kong/kongctl/internal/cmd"
)

var (
	deleteApplicationsShort = i18n.T("root.products.konnect.portal.deleteApplicationsShort",
		"Delete portal applications for a Konnect portal")
	deleteApplicationsLong = normalizers.LongDesc(i18n.T("root.products.konnect.portal.deleteApplicationsLong",
		`Use the delete verb to remove developer applications from a specific Konnect portal.`))
	deleteApplicationsExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.deleteApplicationsExamples",
			fmt.Sprintf(`
# Delete an application by ID
%[1]s delete portal application --portal-id <portal-id> <application-id>
# Delete an application by name
%[1]s delete portal application --portal-name my-portal checkout-app
`, meta.CLIName)))
)

func newDeletePortalApplicationsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s <application-id|name>", applicationsCommandName),
		Short:   deleteApplicationsShort,
		Long:    deleteApplicationsLong,
		Example: deleteApplicationsExample,
		Aliases: []string{"application", "apps"},
		Args:    cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			return bindPortalChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := portalApplicationDeleteHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addPortalChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

type portalApplicationDeleteHandler struct {
	cmd *cobra.Command
}

func (h portalApplicationDeleteHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	portalID, portalName := getPortalIdentifiers(cfg)
	if portalID != "" && portalName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("only one of --%s or --%s can be provided", portalIDFlagName, portalNameFlagName),
		}
	}

	if portalID == "" && portalName == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf(
				"a portal identifier is required. Provide --%s or --%s",
				portalIDFlagName,
				portalNameFlagName,
			),
		}
	}

	if portalID == "" {
		portalID, err = resolvePortalIDByName(portalName, sdk.GetPortalAPI(), helper, cfg)
		if err != nil {
			return err
		}
	}

	appAPI := sdk.GetPortalApplicationAPI()
	if appAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Portal applications client is not available",
			Err: fmt.Errorf("portal applications client not configured"),
		}
	}

	identifier := strings.TrimSpace(args[0])
	if identifier == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("application identifier is required"),
		}
	}

	appID := identifier
	var applicationName string

	if !util.IsValidUUID(identifier) {
		apps, err := fetchPortalApplications(helper, appAPI, portalID, cfg)
		if err != nil {
			return err
		}
		match := findApplicationByName(apps, identifier)
		if match == nil {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("application %q not found", identifier),
			}
		}
		appID = matchID(*match)
		if appID == "" {
			return &cmd.ExecutionError{
				Msg: "Application ID could not be determined",
				Err: fmt.Errorf("application record missing identifier"),
			}
		}
		_, applicationName = helpers.ApplicationSummary(*match)
	}

	if strings.TrimSpace(appID) == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("application identifier is required"),
		}
	}

	targetName := applicationName
	if targetName == "" {
		targetName = appID
	}
	desc := fmt.Sprintf("portal application %q (portal ID: %s)", targetName, portalID)
	if portalName != "" {
		desc = fmt.Sprintf("portal application %q in portal %q", targetName, portalName)
	}
	if err := cmd.ConfirmDelete(helper, desc); err != nil {
		return err
	}

	_, err = appAPI.DeleteApplication(helper.GetContext(), portalID, appID)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		details := konnectCommon.ParseAPIErrorDetails(err)
		attrs = konnectCommon.AppendAPIErrorAttrs(attrs, details)
		msg := konnectCommon.BuildDetailedMessage("Failed to delete portal application", attrs, err)
		return cmd.PrepareExecutionError(msg, err, helper.GetCmd(), attrs...)
	}

	if outType == cmdCommon.TEXT {
		target := appID
		if applicationName != "" {
			target = applicationName
		}
		fmt.Fprintf(helper.GetStreams().Out, "Portal application %q deleted successfully\n", target)
		return nil
	}

	result := map[string]any{
		"portal_id":        portalID,
		"application_id":   appID,
		"application_name": applicationName,
		"status":           "deleted",
	}

	// Remove the name field if it was not resolved to avoid emitting empty strings
	if applicationName == "" {
		delete(result, "application_name")
	}

	printer.Print(result)
	return nil
}
