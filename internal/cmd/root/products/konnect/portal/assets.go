package portal

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	assetsCommandName = "assets"
	assetLogoName     = "logo"
	assetFaviconName  = "favicon"

	outputFileFlagName = "output-file"
	outputFileConfigPath = "konnect.portal.assets.output_file"
)

var (
	assetsUse = assetsCommandName

	assetsShort = i18n.T("root.products.konnect.portal.assetsShort",
		"Retrieve portal assets (logo, favicon)")
	assetsLong = normalizers.LongDesc(i18n.T("root.products.konnect.portal.assetsLong",
		`Use the assets command to fetch logo and favicon images for a Konnect portal.

By default, assets are saved as binary image files. Use --output json or --output yaml
to retrieve the data URL instead.`))
	assetsExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.assetsExamples",
			fmt.Sprintf(`
# Get logo for a portal by ID (saves to logo.png by default)
%[1]s get portal assets logo --portal-id <portal-id>

# Get logo with custom output filename
%[1]s get portal assets logo --portal-name my-portal --output-file my-logo.png

# Get logo as data URL in JSON format
%[1]s get portal assets logo --portal-id <portal-id> --output json

# Get favicon for a portal
%[1]s get portal assets favicon --portal-id <portal-id>
`, meta.CLIName)))

	logoShort = i18n.T("root.products.konnect.portal.logoShort",
		"Retrieve portal logo image")
	logoExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.logoExamples",
			fmt.Sprintf(`
# Get logo for a portal by ID
%[1]s get portal assets logo --portal-id <portal-id>

# Get logo with custom filename
%[1]s get portal assets logo --portal-name my-portal --output-file brand-logo.png
`, meta.CLIName)))

	faviconShort = i18n.T("root.products.konnect.portal.faviconShort",
		"Retrieve portal favicon image")
	faviconExample = normalizers.Examples(
		i18n.T("root.products.konnect.portal.faviconExamples",
			fmt.Sprintf(`
# Get favicon for a portal by ID
%[1]s get portal assets favicon --portal-id <portal-id>

# Get favicon with custom filename
%[1]s get portal assets favicon --portal-name my-portal --output-file site-icon.ico
`, meta.CLIName)))
)

func newGetPortalAssetsCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     assetsUse,
		Short:   assetsShort,
		Long:    assetsLong,
		Example: assetsExample,
		Aliases: []string{"asset"},
	}

	// Add logo subcommand
	logoCmd := newGetPortalAssetLogoCmd(verb, addParentFlags, parentPreRun)
	cmd.AddCommand(logoCmd)

	// Add favicon subcommand
	faviconCmd := newGetPortalAssetFaviconCmd(verb, addParentFlags, parentPreRun)
	cmd.AddCommand(faviconCmd)

	return cmd
}

func newGetPortalAssetLogoCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     assetLogoName,
		Short:   logoShort,
		Example: logoExample,
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			return bindAssetFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			return runGetPortalAsset(c, args, assetLogoName)
		},
	}

	addAssetFlags(cmd, "logo.png")

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

func newGetPortalAssetFaviconCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     assetFaviconName,
		Short:   faviconShort,
		Example: faviconExample,
		PreRunE: func(c *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(c, args); err != nil {
					return err
				}
			}
			return bindAssetFlags(c, args)
		},
		RunE: func(c *cobra.Command, args []string) error {
			return runGetPortalAsset(c, args, assetFaviconName)
		},
	}

	addAssetFlags(cmd, "favicon.ico")

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

func addAssetFlags(cmd *cobra.Command, defaultFilename string) {
	addPortalChildFlags(cmd)
	cmd.Flags().String(outputFileFlagName, defaultFilename,
		fmt.Sprintf(`Output file path for the binary asset.
Only used when output format is not json/yaml.
- Config path: [ %s ]`, outputFileConfigPath))
}

func bindAssetFlags(c *cobra.Command, args []string) error {
	if err := bindPortalChildFlags(c, args); err != nil {
		return err
	}

	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	if flag := c.Flags().Lookup(outputFileFlagName); flag != nil {
		if err := cfg.BindFlag(outputFileConfigPath, flag); err != nil {
			return err
		}
	}

	return nil
}

func runGetPortalAsset(c *cobra.Command, args []string, assetType string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(args, ", "))
	}

	helper := cmd.BuildHelper(c, args)
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	outType, err := helper.GetOutputFormat()
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

	// Resolve portal ID
	portalID, portalName := getPortalIdentifiers(cfg)
	if portalID == "" && portalName == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("either --%s or --%s is required", portalIDFlagName, portalNameFlagName),
		}
	}

	if portalID == "" {
		portalID, err = resolvePortalIDByName(portalName, sdk.GetPortalAPI(), helper, cfg)
		if err != nil {
			return err
		}
	}

	// Get assets API
	assetsAPI := sdk.GetAssetsAPI()
	if assetsAPI == nil {
		return &cmd.ExecutionError{
			Msg: "Portal assets client is not available",
			Err: fmt.Errorf("portal assets client not configured"),
		}
	}

	// Fetch the asset
	var dataURL string
	switch assetType {
	case assetLogoName:
		res, err := assetsAPI.GetPortalAssetLogo(helper.GetContext(), portalID)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return cmd.PrepareExecutionError("Failed to get portal logo", err, helper.GetCmd(), attrs...)
		}
		if res.PortalAssetResponse == nil {
			return &cmd.ExecutionError{
				Msg: "Failed to get portal logo",
				Err: fmt.Errorf("empty response from Konnect"),
			}
		}
		dataURL = res.PortalAssetResponse.Data

	case assetFaviconName:
		res, err := assetsAPI.GetPortalAssetFavicon(helper.GetContext(), portalID)
		if err != nil {
			attrs := cmd.TryConvertErrorToAttrs(err)
			return cmd.PrepareExecutionError("Failed to get portal favicon", err, helper.GetCmd(), attrs...)
		}
		if res.PortalAssetResponse == nil {
			return &cmd.ExecutionError{
				Msg: "Failed to get portal favicon",
				Err: fmt.Errorf("empty response from Konnect"),
			}
		}
		dataURL = res.PortalAssetResponse.Data

	default:
		return fmt.Errorf("unknown asset type: %s", assetType)
	}

	// Handle output based on format
	if outType.String() == "json" || outType.String() == "yaml" {
		// Output as JSON/YAML with data URL
		printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
		if err != nil {
			return err
		}
		defer printer.Flush()

		result := map[string]string{
			"portal_id": portalID,
			"type":      assetType,
			"data":      dataURL,
		}

		printer.Print(result)
	} else {
		// Output as binary file
		outputFile := cfg.GetString(outputFileConfigPath)
		if outputFile == "" {
			if assetType == assetLogoName {
				outputFile = "logo.png"
			} else {
				outputFile = "favicon.ico"
			}
		}

		// Decode data URL and write to file
		if err := writeDataURLToFile(dataURL, outputFile); err != nil {
			return &cmd.ExecutionError{
				Msg: fmt.Sprintf("Failed to write %s to file", assetType),
				Err: err,
			}
		}

		absPath := outputFile
		if ap, err := filepath.Abs(outputFile); err == nil {
			absPath = ap
		}
		fmt.Fprintf(helper.GetStreams().Out, "Successfully saved %s to: %s\n", assetType, absPath)
	}

	return nil
}

// writeDataURLToFile decodes a data URL and writes the binary content to a file
func writeDataURLToFile(dataURL, filename string) error {
	// Parse data URL format: data:<mimetype>;base64,<data>
	if !strings.HasPrefix(dataURL, "data:") {
		return fmt.Errorf("invalid data URL format")
	}

	// Find the base64 data part
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid data URL format: missing comma separator")
	}

	encodedData := parts[1]

	// Decode base64
	decodedData, err := base64.StdEncoding.DecodeString(encodedData)
	if err != nil {
		return fmt.Errorf("failed to decode base64 data: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filename, decodedData, 0o600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
