package portal

import (
	"fmt"
	"strings"

	kkOps "github.com/Kong/sdk-konnect-go/models/operations"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	konnectCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"

	"github.com/kong/kongctl/internal/cmd"
)

var (
	deleteRegistrationsShort = i18n.T("root.products.konnect.portal.deleteRegistrationsShort",
		"Delete portal application registrations for a Konnect portal")
	deleteRegistrationsLong = normalizers.LongDesc(i18n.T("root.products.konnect.portal.deleteRegistrationsLong",
		`Use the delete verb to remove application registrations from a specific Konnect portal.`))
	deleteRegistrationsExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.deleteRegistrationsExamples",
			fmt.Sprintf(`
# Delete a registration by ID
%[1]s delete portal application registration --portal-id <portal-id> <registration-id>
# Delete a registration by ID with portal name
%[1]s delete portal application registration --portal-name my-portal <registration-id>
`, meta.CLIName)))
)

func newDeletePortalApplicationRegistrationsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s <registration-id>", registrationsUse),
		Short:   deleteRegistrationsShort,
		Long:    deleteRegistrationsLong,
		Example: deleteRegistrationsExample,
		Aliases: []string{
			"registration",
			"registrations",
			"application-registration",
			"application-registrations",
		},
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			if err := bindPortalChildFlags(cmd, args); err != nil {
				return err
			}
			return bindRegistrationFilterFlags(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := portalApplicationRegistrationDeleteHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addPortalChildFlags(cmd)
	addRegistrationFilterFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

type portalApplicationRegistrationDeleteHandler struct {
	cmd *cobra.Command
}

func (h portalApplicationRegistrationDeleteHandler) run(args []string) error {
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

	regAPI := sdk.GetPortalApplicationRegistrationAPI()
	if regAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Portal application registrations client is not available",
			Err: fmt.Errorf("portal application registrations client not configured"),
		}
	}

	filters := registrationFiltersFromFlags(h.cmd)

	registrationID := strings.TrimSpace(args[0])
	if registrationID == "" {
		return &cmd.ConfigurationError{Err: fmt.Errorf("registration identifier is required")}
	}

	applicationID := strings.TrimSpace(filters.ApplicationID)
	applicationName := strings.TrimSpace(filters.ApplicationName)
	if applicationID == "" {
		regs, err := fetchPortalApplicationRegistrations(helper, regAPI, portalID, cfg, registrationFilters{})
		if err != nil {
			return err
		}
		match := findRegistrationByID(regs, registrationID)
		if match == nil {
			return &cmd.ConfigurationError{Err: fmt.Errorf("registration %q not found", registrationID)}
		}
		matchApp := match.GetApplication()
		applicationID = matchApp.ID
		applicationName = matchApp.Name
	}

	if applicationID == "" {
		return &cmd.ExecutionError{
			Msg: "Application identifier for registration could not be determined",
			Err: fmt.Errorf("missing application id for registration %s", registrationID),
		}
	}

	desc := fmt.Sprintf("portal application registration %q (portal ID: %s)", registrationID, portalID)
	if portalName != "" {
		desc = fmt.Sprintf("portal application registration %q in portal %q", registrationID, portalName)
	}
	if applicationName != "" {
		desc = fmt.Sprintf("%s for application %q", desc, applicationName)
	}
	if err := cmd.ConfirmDelete(helper, desc); err != nil {
		return err
	}

	req := kkOps.DeleteApplicationRegistrationRequest{
		PortalID:       portalID,
		ApplicationID:  applicationID,
		RegistrationID: registrationID,
	}

	_, err = regAPI.DeleteApplicationRegistration(helper.GetContext(), req)
	if err != nil {
		attrs := cmd.TryConvertErrorToAttrs(err)
		details := konnectCommon.ParseAPIErrorDetails(err)
		attrs = konnectCommon.AppendAPIErrorAttrs(attrs, details)
		msg := konnectCommon.BuildDetailedMessage("Failed to delete portal application registration", attrs, err)
		return cmd.PrepareExecutionError(msg, err, helper.GetCmd(), attrs...)
	}

	if outType == cmdCommon.TEXT {
		fmt.Fprintf(
			helper.GetStreams().Out,
			"Portal application registration %q deleted successfully\n",
			registrationID,
		)
		return nil
	}

	return nil
}
