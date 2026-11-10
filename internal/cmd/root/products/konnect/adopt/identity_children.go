package adopt

import (
	"fmt"

	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
)

func runIdentityGroupAdoptions(cmd *cobra.Command, args []string,
	setup adoptCommon.AdoptRunSetup, directory string,
) error {
	selection, err := readChildSelection(cmd, args, "", "")
	if err != nil {
		return err
	}
	ctx := setup.Helper.GetContext()
	api := setup.SDK.GetIdentityGroupsAPI()
	if !selection.all && selection.pattern == "" {
		result, err := adoptIdentityGroup(ctx, api, directory, selection.identifier,
			setup.AdoptFlags.Namespace, setup.AdoptFlags.OverwriteNamespace)
		if err != nil {
			return err
		}
		return adoptCommon.PrintAdoptResult(setup.Helper, setup.OutType, result, "Kong Identity group")
	}
	values, err := helpers.ListIdentityGroups(ctx, api, directory, 100)
	if err != nil {
		return err
	}
	candidates := make([]childCandidate, 0, len(values))
	for _, value := range values {
		candidates = append(candidates, childCandidate{value.ID, value.Name})
	}
	selected, err := selectChildren(candidates, selection)
	if err != nil {
		return err
	}
	return runChildAdoptions(setup, selected, func(value childCandidate) (preparedChildAdoption, error) {
		return prepareAdoptIdentityGroup(ctx,
			api,
			directory,
			value.id,
			setup.AdoptFlags.Namespace,
			setup.AdoptFlags.OverwriteNamespace,
			value.name)
	})
}

func runIdentityClientAdoptions(cmd *cobra.Command, args []string,
	setup adoptCommon.AdoptRunSetup, parent string,
) error {
	selection, err := readChildSelection(cmd, args, "", "client-name")
	if err != nil {
		return err
	}
	// OAuth IDs are opaque; the positional argument retains its ID-only meaning.
	if len(args) != 0 {
		selection.idOnly = true
	}
	ctx := setup.Helper.GetContext()
	api := setup.SDK.GetIdentityAuthServerClientsAPI()
	if !selection.all && selection.pattern == "" {
		result, err := adoptIdentityAuthServerClient(ctx, api, parent, selection.identifier, selection.nameOnly,
			setup.AdoptFlags.Namespace, setup.AdoptFlags.OverwriteNamespace)
		if err != nil {
			return err
		}
		return adoptCommon.PrintAdoptResult(setup.Helper, setup.OutType, result, "Kong Identity auth-server client")
	}
	values, err := helpers.ListIdentityAuthServerClients(ctx, api, parent, 100)
	if err != nil {
		return err
	}
	candidates := make([]childCandidate, 0, len(values))
	for _, value := range values {
		candidates = append(candidates, childCandidate{value.ID, value.Name})
	}
	selected, err := selectChildren(candidates, selection)
	if err != nil {
		return err
	}
	return runChildAdoptions(setup, selected, func(value childCandidate) (preparedChildAdoption, error) {
		return prepareAdoptIdentityAuthServerClient(ctx,
			api,
			parent,
			value.id,
			false,
			setup.AdoptFlags.Namespace,
			setup.AdoptFlags.OverwriteNamespace,
			value.name)
	})
}

func runIdentityBasicAuthAdoptions(cmd *cobra.Command, args []string,
	setup adoptCommon.AdoptRunSetup, directory, principal string,
) error {
	selection, err := readChildSelection(cmd, args, "", "username")
	if err != nil {
		return err
	}
	if len(args) != 0 {
		selection.idOnly = true
	}
	ctx := setup.Helper.GetContext()
	api := setup.SDK.GetIdentityBasicAuthAPI()
	if !selection.all && selection.pattern == "" {
		id, username := "", selection.identifier
		if selection.idOnly {
			id, username = selection.identifier, ""
		}
		result, err := adoptIdentityBasicAuth(ctx, api, directory, principal, id, username,
			setup.AdoptFlags.Namespace, setup.AdoptFlags.OverwriteNamespace)
		if err != nil {
			return err
		}
		return adoptCommon.PrintAdoptResult(setup.Helper, setup.OutType, result, "Kong Identity basic-auth credential")
	}
	values, err := helpers.ListIdentityBasicAuths(ctx, api, directory, principal, 100)
	if err != nil {
		return err
	}
	candidates := make([]childCandidate, 0, len(values))
	for _, value := range values {
		candidates = append(candidates, childCandidate{value.ID, value.Username})
	}
	selected, err := selectChildren(candidates, selection)
	if err != nil {
		return err
	}
	return runChildAdoptions(setup, selected, func(value childCandidate) (preparedChildAdoption, error) {
		write, err := prepareAdoptIdentityBasicAuth(
			ctx,
			api,
			directory,
			principal,
			value.id,
			"",
			setup.AdoptFlags.Namespace,
			setup.AdoptFlags.OverwriteNamespace,
			value.name,
		)
		if err != nil {
			return nil, fmt.Errorf("basic-auth username %q: %w", value.name, err)
		}
		return write, nil
	})
}
