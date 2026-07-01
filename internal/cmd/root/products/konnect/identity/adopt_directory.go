package identity

import (
	"fmt"
	"maps"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"
	cmdpkg "github.com/kong/kongctl/internal/cmd"
	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/spf13/cobra"
)

const directoryResourceType = "identity_directory"

func newAdoptDirectoryCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "directory <directory-id|directory-name>",
		Aliases: []string{"directories", "dir", "dirs"},
		Short:   "Adopt an existing Kong Identity directory into namespace management",
		Long: "Apply the KONGCTL-namespace label to an existing Kong Identity directory " +
			"that is not currently managed by kongctl.",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("exactly one directory identifier (name or ID) is required")
			}
			if trimmed := strings.TrimSpace(args[0]); trimmed == "" {
				return fmt.Errorf("directory identifier cannot be empty")
			}
			return nil
		},
	}

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	if parentPreRun != nil {
		cmd.PreRunE = parentPreRun
	}

	cmd.RunE = func(cobraCmd *cobra.Command, args []string) error {
		s, err := adoptCommon.SetupAdoptRun(cobraCmd, args)
		if err != nil {
			return err
		}

		result, err := adoptDirectory(
			s.Helper,
			s.SDK.GetIdentityDirectoryAPI(),
			s.Cfg,
			s.AdoptFlags.Namespace,
			s.AdoptFlags.OverwriteNamespace,
			strings.TrimSpace(args[0]),
		)
		if err != nil {
			return err
		}

		return adoptCommon.PrintAdoptResult(s.Helper, s.OutType, result, "identity_directory")
	}

	return cmd
}

func adoptDirectory(
	helper cmdpkg.Helper,
	api helpers.IdentityDirectoryAPI,
	cfg config.Hook,
	namespace string,
	overwriteNamespace bool,
	identifier string,
) (*adoptCommon.AdoptResult, error) {
	directory, err := resolveDirectory(helper, api, cfg, identifier)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(directory.ID) == "" {
		return nil, &cmdpkg.ConfigurationError{Err: fmt.Errorf("unable to resolve identity directory identifier")}
	}

	if currentNamespace, ok := directory.Labels[labels.NamespaceKey]; ok && currentNamespace != "" &&
		!overwriteNamespace {
		display := directory.Name
		if display == "" {
			display = directory.ID
		}
		return nil, &cmdpkg.ConfigurationError{
			Err: fmt.Errorf("identity directory %q already has namespace label %q", display, currentNamespace),
		}
	}

	req := kkComps.ReplaceDirectoryBody{
		Name:                  directory.Name,
		AllowedControlPlanes:  directory.AllowedControlPlanes,
		AllowAllControlPlanes: directory.AllowAllControlPlanes,
		TTLSecs:               directory.TTLSecs,
		NegativeTTLSecs:       directory.NegativeTTLSecs,
		Labels:                directoryAdoptLabels(directory.Labels, namespace),
		ManagedBy:             directory.ManagedBy,
	}
	if strings.TrimSpace(directory.Description) != "" {
		description := directory.Description
		req.Description = &description
	}

	ctx := adoptCommon.EnsureContext(helper.GetContext())
	resp, err := api.ReplaceDirectory(ctx, directory.ID, req)
	if err != nil {
		attrs := cmdpkg.TryConvertErrorToAttrs(err)
		return nil, cmdpkg.PrepareExecutionError("failed to update identity_directory", err, helper.GetCmd(), attrs...)
	}
	updated := directory
	if resp != nil && resp.GetKongDirectory() != nil {
		updated = normalizeDirectory(*resp.GetKongDirectory())
	}

	ns := namespace
	if updated.Labels != nil {
		if v, ok := updated.Labels[labels.NamespaceKey]; ok && v != "" {
			ns = v
		}
	}

	return &adoptCommon.AdoptResult{
		ResourceType: directoryResourceType,
		ID:           updated.ID,
		Name:         updated.Name,
		Namespace:    ns,
	}, nil
}

func resolveDirectory(
	helper cmdpkg.Helper,
	api helpers.IdentityDirectoryAPI,
	cfg config.Hook,
	identifier string,
) (directoryResource, error) {
	if api == nil {
		return directoryResource{}, fmt.Errorf("identity directory API is not configured")
	}

	directories, err := runDirectoryList(api, helper, cfg)
	if err != nil {
		return directoryResource{}, err
	}

	for _, directory := range directories {
		if directory.ID == identifier || strings.EqualFold(directory.Name, identifier) {
			return directory, nil
		}
	}

	return directoryResource{}, &cmdpkg.ConfigurationError{
		Err: fmt.Errorf("identity directory %q not found", identifier),
	}
}

func directoryAdoptLabels(existing map[string]string, namespace string) map[string]string {
	result := make(map[string]string, len(existing)+1)
	maps.Copy(result, existing)
	result[labels.NamespaceKey] = namespace
	return result
}
