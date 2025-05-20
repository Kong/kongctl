package dump

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	kkInternalOps "github.com/Kong/sdk-konnect-go-internal/models/operations"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/common"
	konnectCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/spf13/cobra"
)

const (
	Verb = verbs.Dump
)

var (
	dumpUse = Verb.String()

	dumpShort = i18n.T("root.verbs.dump.dumpShort", "Dump objects")

	dumpLong = normalizers.LongDesc(i18n.T("root.verbs.dump.dumpLong",
		`Use dump to export an object or list of objects.`))

	dumpExamples = normalizers.Examples(i18n.T("root.verbs.dump.dumpExamples",
		fmt.Sprintf(`
		# Export all portals as Terraform import blocks to stdout
		%[1]s dump --resources=portal 

		# Export all portals and their child resources (documents, specifications, pages, settings)
		%[1]s dump --resources=portal --include-child-resources

		# Export all portals as Terraform import blocks to a file
		%[1]s dump --resources=portal --output-file=portals.tf

		# Export all portals with their child resources to a file
		%[1]s dump --resources=portal --include-child-resources --output-file=portals.tf
		`, meta.CLIName)))

	resources             string
	includeChildResources bool
	outputFile            string
	dumpFormat            = cmd.NewEnum([]string{"tf-imports"}, "tf-imports")
)

// Maps resource types to their corresponding Terraform resource types
var resourceTypeMap = map[string]string{
	"portal":               "konnect_portal",
	"portal_document":      "konnect_portal_document",
	"portal_specification": "konnect_portal_specification",
	"portal_page":          "konnect_portal_page",
	"portal_settings":      "konnect_portal_settings",
}

// Maps parent resources to their child resource types
var parentChildResourceMap = map[string][]string{
	"portal": {
		"portal_document",
		"portal_specification",
		"portal_page",
		"portal_settings",
	},
}

// sanitizeTerraformResourceName converts a resource name to a valid Terraform identifier
func sanitizeTerraformResourceName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace spaces and special characters with underscores
	reg := regexp.MustCompile(`[^a-z0-9_]`)
	name = reg.ReplaceAllString(name, "_")

	// Ensure it starts with a letter
	if len(name) > 0 && !regexp.MustCompile(`^[a-z]`).MatchString(name) {
		name = "resource_" + name
	}

	// Ensure no double underscores
	name = regexp.MustCompile(`__+`).ReplaceAllString(name, "_")

	// Trim underscores from start and end
	name = strings.Trim(name, "_")

	// If empty (unlikely), use a default
	if name == "" {
		name = "resource"
	}

	return name
}

// escapeTerraformString properly escapes a string for use in HCL
func escapeTerraformString(s string) string {
	// Replace backslashes with double backslashes
	s = strings.ReplaceAll(s, "\\", "\\\\")

	// Replace double quotes with escaped double quotes
	s = strings.ReplaceAll(s, "\"", "\\\"")

	return s
}

// formatTerraformImport creates a Terraform import block
func formatTerraformImport(resourceType, resourceName, resourceID string) string {
	terraformType, ok := resourceTypeMap[resourceType]
	if !ok {
		terraformType = "unknown_" + resourceType
	}

	safeName := sanitizeTerraformResourceName(resourceName)
	
	// For the import block, we always add a provider reference
	providerName := "konnect-beta"

	// Format the ID based on the resource type
	var idBlock string
	if strings.HasPrefix(resourceType, "portal_") && resourceType != "portal" && strings.Contains(resourceID, ":") {
		parts := strings.Split(resourceID, ":")
		if len(parts) == 2 {
			portalID := parts[0]
			resourceComponentID := parts[1]
			
			idBlock = fmt.Sprintf("  id = jsonencode({\n    id: \"%s\"\n    portal_id: \"%s\"\n  })",
				resourceComponentID, portalID)
		} else {
			idBlock = fmt.Sprintf("  id = \"%s\"", escapeTerraformString(resourceID))
		}
	} else {
		idBlock = fmt.Sprintf("  id = \"%s\"", escapeTerraformString(resourceID))
	}

	return fmt.Sprintf("import {\n  to = %s.%s\n  provider = %s\n%s\n}\n",
		terraformType, safeName, providerName, idBlock)
}

// Helper function for the internal SDK
func Int64(v int64) *int64 {
	return &v
}

// dumpPortals exports all portals as Terraform import blocks
func dumpPortals(
	ctx context.Context,
	writer io.Writer,
	kkClient helpers.PortalAPI,
	requestPageSize int64,
	includeChildResources bool) error {
	var pageNumber int64 = 1

	for {
		req := kkInternalOps.ListPortalsRequest{
			PageSize:   Int64(requestPageSize),
			PageNumber: Int64(pageNumber),
		}

		res, err := kkClient.ListPortals(ctx, req)
		if err != nil {
			return fmt.Errorf("failed to list portals: %w", err)
		}

		if res.ListPortalsResponseV3 == nil || len(res.ListPortalsResponseV3.Data) == 0 {
			break
		}

		for _, portal := range res.ListPortalsResponseV3.Data {
			// Write the portal import block
			importBlock := formatTerraformImport("portal", portal.Name, portal.ID)
			if _, err := fmt.Fprintln(writer, importBlock); err != nil {
				return fmt.Errorf("failed to write portal import block: %w", err)
			}

			// If includeChildResources is true, dump the child resources as well
			if includeChildResources {
				if err := dumpPortalChildResources(ctx, writer, kkClient, portal.ID, portal.Name, requestPageSize); err != nil {
					// Log error but continue with other portals
					fmt.Fprintf(os.Stderr, "Warning: Failed to dump child resources for portal %s: %v\n", portal.Name, err)
				}
			}
		}

		pageNumber++
	}

	return nil
}

// dumpPortalChildResources exports all child resources of a portal as Terraform import blocks
func dumpPortalChildResources(
	ctx context.Context,
	writer io.Writer,
	kkClient helpers.PortalAPI,
	portalID string,
	portalName string,
	requestPageSize int64,
) error {
	// Start with a header comment
	// No header comment needed

	// Try to dump each type of child resource, but continue if any fail
	// Documents
	docs, err := helpers.GetDocumentsForPortal(ctx, kkClient, portalID)
	if err == nil && len(docs) > 0 {
		// No documents header needed

		for _, doc := range docs {
			resourceName := fmt.Sprintf("%s_%s", portalName, doc.Slug)
			resourceID := fmt.Sprintf("%s:%s", portalID, doc.ID)
			importBlock := formatTerraformImport("portal_document", resourceName, resourceID)
			if _, err := fmt.Fprintln(writer, importBlock); err != nil {
				return fmt.Errorf("failed to write portal document import block: %w", err)
			}
		}
	}

	// Specifications
	specs, err := helpers.GetSpecificationsForPortal(ctx, kkClient, portalID)
	if err == nil && len(specs) > 0 {
		// No specifications header needed

		for _, spec := range specs {
			resourceName := fmt.Sprintf("%s_%s", portalName, spec.Name)
			resourceID := fmt.Sprintf("%s:%s", portalID, spec.ID)
			importBlock := formatTerraformImport("portal_specification", resourceName, resourceID)
			if _, err := fmt.Fprintln(writer, importBlock); err != nil {
				return fmt.Errorf("failed to write portal specification import block: %w", err)
			}
		}
	}

	// Pages
	pages, err := helpers.GetPagesForPortal(ctx, kkClient, portalID)
	if err == nil && len(pages) > 0 {
		// No pages header needed

		for _, page := range pages {
			pageName := page.Name
			if pageName == "" {
				pageName = page.Slug
			}
			resourceName := fmt.Sprintf("%s_%s", portalName, pageName)
			resourceID := fmt.Sprintf("%s:%s", portalID, page.ID)
			importBlock := formatTerraformImport("portal_page", resourceName, resourceID)
			if _, err := fmt.Fprintln(writer, importBlock); err != nil {
				return fmt.Errorf("failed to write portal page import block: %w", err)
			}
		}
	}

	// Settings
	if helpers.HasPortalSettings(ctx, kkClient, portalID) {
		// No settings header needed

		resourceName := fmt.Sprintf("%s_settings", portalName)
		importBlock := formatTerraformImport("portal_settings", resourceName, portalID)
		if _, err := fmt.Fprintln(writer, importBlock); err != nil {
			return fmt.Errorf("failed to write portal settings import block: %w", err)
		}
	}

	return nil
}

type dumpCmd struct {
	*cobra.Command
}

func (c *dumpCmd) validate(helper cmd.Helper) error {
	resourceList := strings.Split(resources, ",")
	for _, resource := range resourceList {
		resource = strings.TrimSpace(resource)
		if resource == "" {
			continue
		}

		// For now, only portal is supported as a top-level resource for the dump command
		// Child resources are handled automatically when --include-child-resources is true
		if resource != "portal" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("unsupported resource type: %s. Currently only 'portal' is supported as a top-level resource", resource),
			}
		}

		// Check if the resource type is known
		if _, ok := resourceTypeMap[resource]; !ok {
			supportedTypes := []string{"portal"}
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("unsupported resource type: %s. Supported types: %s",
					resource, strings.Join(supportedTypes, ", ")),
			}
		}
	}

	config, err := helper.GetConfig()
	if err != nil {
		return err
	}

	// Check the page size
	pageSize := config.GetInt(konnectCommon.RequestPageSizeConfigPath)
	if pageSize < 0 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("%s must be greater than or equal to 0", konnectCommon.RequestPageSizeFlagName),
		}
	}

	return nil
}

func (c *dumpCmd) runE(cobraCmd *cobra.Command, args []string) error {
	helper := cmd.BuildHelper(cobraCmd, args)
	if err := c.validate(helper); err != nil {
		return err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	// Determine the output writer (file or stdout)
	var writer io.Writer
	if outputFile != "" {
		file, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		writer = file
	} else {
		writer = helper.GetStreams().Out
	}

	// Process each resource type
	resourceList := strings.Split(resources, ",")
	for _, resource := range resourceList {
		resource = strings.TrimSpace(resource)
		if resource == "" {
			continue
		}

		switch resource {
		case "portal":
			// Use the page size configured in the config system
			// The system will use the default value (DefaultRequestPageSize) if not explicitly set
			requestPageSize := int64(cfg.GetIntOrElse(
				konnectCommon.RequestPageSizeConfigPath,
				konnectCommon.DefaultRequestPageSize))
			if err := dumpPortals(
				helper.GetContext(),
				writer,
				sdk.GetPortalAPI(),
				requestPageSize,
				includeChildResources); err != nil {
				return err
			}
		}
	}

	return nil
}

// Helper function to get map keys as a slice
func getMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func NewDumpCmd() (*cobra.Command, error) {
	dumpCommand := &cobra.Command{
		Use:     dumpUse,
		Short:   dumpShort,
		Long:    dumpLong,
		Example: dumpExamples,
		Aliases: []string{"d", "D"},
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			cmd.SetContext(context.WithValue(cmd.Context(), verbs.Verb, Verb))
		},
	}

	dumpCommand.Flags().StringVarP(&resources, "resources",
		"r",
		"",
		"Comma separated list of resource types to dump.")
	if err := dumpCommand.MarkFlagRequired("resources"); err != nil {
		return nil, err
	}

	dumpCommand.Flags().BoolVar(&includeChildResources, "include-child-resources",
		false,
		"Include child resources in the dump.")

	dumpCommand.Flags().StringVar(&outputFile, "output-file",
		"",
		"File to write the output to. If not specified, output is written to stdout.")

	// Add the page size flag with the same default as other commands
	dumpCommand.Flags().Int(
		konnectCommon.RequestPageSizeFlagName,
		konnectCommon.DefaultRequestPageSize,
		fmt.Sprintf(`Max number of results to include per response page.
- Config path: [ %s ]`,
			konnectCommon.RequestPageSizeConfigPath))

	// This shadows the global output flag
	dumpCommand.Flags().VarP(dumpFormat, common.OutputFlagName, common.OutputFlagShort,
		fmt.Sprintf(`Configures the format of data written to STDOUT.
- Allowed: [ %s ]`, strings.Join(dumpFormat.Allowed, "|")))

	rv := &dumpCmd{
		Command: dumpCommand,
	}

	rv.RunE = rv.runE

	// Bind the page-size flag to the config system
	f := dumpCommand.Flags().Lookup(konnectCommon.RequestPageSizeFlagName)
	dumpCommand.PersistentPreRunE = func(c *cobra.Command, args []string) error {
		// Call the original PersistentPreRun
		c.SetContext(context.WithValue(c.Context(), verbs.Verb, Verb))

		// Set the SDK factory in the context
		c.SetContext(context.WithValue(c.Context(),
			helpers.SDKAPIFactoryKey, helpers.SDKAPIFactory(konnectCommon.KonnectSDKFactory)))

		// Bind flags to config
		helper := cmd.BuildHelper(c, args)
		cfg, err := helper.GetConfig()
		if err != nil {
			return err
		}

		return cfg.BindFlag(konnectCommon.RequestPageSizeConfigPath, f)
	}

	return rv.Command, nil
}
