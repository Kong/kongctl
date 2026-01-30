package team

import (
	"fmt"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdCommon "github.com/kong/kongctl/internal/cmd/common"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/validator"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/util"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

func NewTeamCmd(
	verb verbs.VerbValue,
	baseCmd *cobra.Command,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) (*cobra.Command, error) {
	cmd := baseCmd
	if cmd == nil {
		cmd = &cobra.Command{}
	}

	cmd.Use = "team team-id"
	cmd.Short = "Adopt an existing Konnect team into namespace management"
	cmd.Long = "Apply the KONGCTL-namespace label to an existing Konnect team " +
		"that is not currently managed by kongctl."
	cmd.Args = func(_ *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("exactly one team identifier (ID) is required")
		}
		if trimmed := strings.TrimSpace(args[0]); trimmed == "" {
			return fmt.Errorf("team identifier cannot be empty")
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

		result, err := adoptTeam(helper, sdk.GetOrganizationTeamAPI(), cfg, namespace, strings.TrimSpace(args[0]))
		if err != nil {
			return err
		}

		streams := helper.GetStreams()
		if outType == cmdCommon.TEXT {
			name := result.Name
			if name == "" {
				name = result.ID
			}
			fmt.Fprintf(streams.Out, "Adopted team %q (%s) into namespace %q\n", name, result.ID, result.Namespace)
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

func adoptTeam(
	helper cmdpkg.Helper,
	teamAPI helpers.OrganizationTeamAPI,
	cfg config.Hook,
	namespace string,
	identifier string,
) (*adoptCommon.AdoptResult, error) {
	team, err := resolveTeam(helper, teamAPI, cfg, identifier)
	if err != nil {
		return nil, err
	}

	if existing := team.Labels; existing != nil {
		if currentNamespace, ok := existing[labels.NamespaceKey]; ok && currentNamespace != "" {
			return nil, &cmdpkg.ConfigurationError{
				Err: fmt.Errorf("team %q already has namespace label %q", *team.Name, currentNamespace),
			}
		}
	}

	updateReq := kkComps.UpdateTeam{
		Name:        team.Name,
		Description: team.Description,
		Labels:      adoptCommon.PointerLabelMap(team.Labels, namespace),
	}

	ctx := adoptCommon.EnsureContext(helper.GetContext())

	resp, err := teamAPI.UpdateTeam(ctx, identifier, &updateReq)
	if err != nil {
		fmt.Println("Failed to update team labels:", updateReq.Labels, err)
		attrs := cmdpkg.TryConvertErrorToAttrs(err)
		return nil, cmdpkg.PrepareExecutionError("failed to update team", err, helper.GetCmd(), attrs...)
	}

	updated := resp.GetTeam()
	if updated == nil {
		return nil, fmt.Errorf("update team response missing team data")
	}

	ns := namespace
	if updated.Labels != nil {
		if v, ok := updated.Labels[labels.NamespaceKey]; ok && v != "" {
			ns = v
		}
	}

	return &adoptCommon.AdoptResult{
		ResourceType: "team",
		ID:           *updated.ID,
		Name:         *updated.Name,
		Namespace:    ns,
	}, nil
}

func resolveTeam(
	helper cmdpkg.Helper,
	teamAPI helpers.OrganizationTeamAPI,
	_ config.Hook,
	identifier string,
) (*kkComps.Team, error) {
	ctx := adoptCommon.EnsureContext(helper.GetContext())

	if !util.IsValidUUID(identifier) {
		return nil, &cmdpkg.ConfigurationError{
			Err: fmt.Errorf("identifier %q is not a valid UUID", identifier),
		}
	}

	resp, err := teamAPI.GetTeam(ctx, identifier)
	if err != nil {
		attrs := cmdpkg.TryConvertErrorToAttrs(err)
		return nil, cmdpkg.PrepareExecutionError("failed to retrieve team", err, helper.GetCmd(), attrs...)
	}
	team := resp.GetTeam()
	if team == nil {
		return nil, fmt.Errorf("team %s not found", identifier)
	}
	return team, nil
}
