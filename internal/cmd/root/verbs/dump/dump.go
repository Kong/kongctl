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

		# Export all portals as Terraform import blocks to a file
		%[1]s dump --resources=portal --output-file=portals.tf
		`, meta.CLIName)))

	resources             string
	includeChildResources bool
	outputFile            string
	dumpFormat            = cmd.NewEnum([]string{"tf-imports"}, "tf-imports")
)

// Maps resource types to their corresponding Terraform resource types
var resourceTypeMap = map[string]string{
	"portal": "kong_portal",
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
	escapedID := escapeTerraformString(resourceID)
	
	return fmt.Sprintf("import {\n  to = %s.%s\n  id = \"%s\"\n}\n",
		terraformType, safeName, escapedID)
}

// Helper function for the internal SDK
func Int64(v int64) *int64 {
	return &v
}

// dumpPortals exports all portals as Terraform import blocks
func dumpPortals(ctx context.Context, writer io.Writer, kkClient helpers.PortalAPI, requestPageSize int64) error {
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
			importBlock := formatTerraformImport("portal", portal.Name, portal.ID)
			if _, err := fmt.Fprintln(writer, importBlock); err != nil {
				return fmt.Errorf("failed to write portal import block: %w", err)
			}
		}
		
		pageNumber++
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
		
		if _, ok := resourceTypeMap[resource]; !ok {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("unsupported resource type: %s. Supported types: %s", 
					resource, strings.Join(getMapKeys(resourceTypeMap), ", ")),
			}
		}
	}
	
	config, err := helper.GetConfig()
	if err != nil {
		return err
	}
	
	pageSize := config.GetInt(konnectCommon.RequestPageSizeConfigPath)
	if pageSize < 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("%s must be greater than 0", konnectCommon.RequestPageSizeFlagName),
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
			requestPageSize := int64(cfg.GetInt(konnectCommon.RequestPageSizeConfigPath))
			if err := dumpPortals(helper.GetContext(), writer, sdk.GetPortalAPI(), requestPageSize); err != nil {
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
	
	// This shadows the global output flag
	dumpCommand.Flags().VarP(dumpFormat, common.OutputFlagName, common.OutputFlagShort,
		fmt.Sprintf(`Configures the format of data written to STDOUT.
- Allowed: [ %s ]`, strings.Join(dumpFormat.Allowed, "|")))
	
	rv := &dumpCmd{
		Command: dumpCommand,
	}
	
	rv.RunE = rv.runE
	
	return rv.Command, nil
}