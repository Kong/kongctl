package dump

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	kkOps "github.com/Kong/sdk-konnect-go/models/operations"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	konnectCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/log"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
)

type tfImportOptions struct {
	resources             []string
	includeChildResources bool
	outputFile            string
}

var tfImportAllowedResources = map[string]struct{}{
	"portals":                     {},
	"apis":                        {},
	"application_auth_strategies": {},
	"control_planes":              {},
}

func newTFImportCmd() *cobra.Command {
	opts := &tfImportOptions{}

	cmd := &cobra.Command{
		Use:     formatTFImport,
		Aliases: []string{"tf-imports"},
		Short:   i18n.T("root.verbs.dump.tf.short", "Export resources as Terraform import blocks"),
		Long: normalizers.LongDesc(i18n.T("root.verbs.dump.tf.long",
			"Export existing Konnect resources as Terraform import blocks.")),
		RunE: func(cmd *cobra.Command, args []string) error {
			helper := cmdpkg.BuildHelper(cmd, args)
			resourcesFlag := cmd.Flags().Lookup("resources").Value.String()
			normalized, err := normalizeResourceList(resourcesFlag, tfImportAllowedResources)
			if err != nil {
				return err
			}
			opts.resources = normalized
			if err := ensureNonNegativePageSize(helper); err != nil {
				return err
			}
			return runTerraformDump(helper, *opts)
		},
	}

	cmd.Flags().String("resources", "",
		"Comma separated list of resource types to dump (portals, apis, application_auth_strategies, control_planes).")
	_ = cmd.MarkFlagRequired("resources")

	cmd.Flags().BoolVar(&opts.includeChildResources, "include-child-resources", false,
		"Include child resources in the dump.")

	cmd.Flags().StringVar(&opts.outputFile, "output-file", "",
		"File to write the output to. If not specified, output is written to stdout.")

	cmd.Flags().String(konnectCommon.BaseURLFlagName, "",
		fmt.Sprintf(`Base URL for Konnect API requests.
- Config path: [ %s ]
- Default   : [ %s ]`,
			konnectCommon.BaseURLConfigPath, konnectCommon.BaseURLDefault))

	cmd.Flags().String(konnectCommon.RegionFlagName, "",
		fmt.Sprintf(`Konnect region identifier (for example "eu"). Used to construct the base URL when --%s is not provided.
- Config path: [ %s ]`,
			konnectCommon.BaseURLFlagName, konnectCommon.RegionConfigPath))

	cmd.Flags().String(konnectCommon.PATFlagName, "",
		fmt.Sprintf(`Konnect Personal Access Token (PAT) used to authenticate the CLI.
Setting this value overrides tokens obtained from the login command.
- Config path: [ %s ]`,
			konnectCommon.PATConfigPath))

	cmd.Flags().Int(
		konnectCommon.RequestPageSizeFlagName,
		konnectCommon.DefaultRequestPageSize,
		fmt.Sprintf(`Max number of results to include per response page.
- Config path: [ %s ]`, konnectCommon.RequestPageSizeConfigPath))

	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		helper := cmdpkg.BuildHelper(c, args)
		cfg, err := helper.GetConfig()
		if err != nil {
			return err
		}

		if f := c.Flags().Lookup(konnectCommon.BaseURLFlagName); f != nil {
			if err := cfg.BindFlag(konnectCommon.BaseURLConfigPath, f); err != nil {
				return err
			}
		}

		if f := c.Flags().Lookup(konnectCommon.RegionFlagName); f != nil {
			if err := cfg.BindFlag(konnectCommon.RegionConfigPath, f); err != nil {
				return err
			}
		}

		if f := c.Flags().Lookup(konnectCommon.PATFlagName); f != nil {
			if err := cfg.BindFlag(konnectCommon.PATConfigPath, f); err != nil {
				return err
			}
		}

		return cfg.BindFlag(konnectCommon.RequestPageSizeConfigPath,
			c.Flags().Lookup(konnectCommon.RequestPageSizeFlagName))
	}

	return cmd
}

func runTerraformDump(helper cmdpkg.Helper, opts tfImportOptions) error {
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

	writer, cleanup, err := getDumpWriter(helper, opts.outputFile)
	if err != nil {
		return err
	}
	defer func() {
		_ = cleanup()
	}()

	for _, resource := range opts.resources {
		requestPageSize := int64(cfg.GetIntOrElse(
			konnectCommon.RequestPageSizeConfigPath,
			konnectCommon.DefaultRequestPageSize))

		switch resource {
		case "portals":
			if err := dumpPortals(
				helper.GetContext(),
				writer,
				sdk.GetPortalAPI(),
				requestPageSize,
				opts.includeChildResources,
			); err != nil {
				return err
			}
		case "apis":
			if err := dumpAPIs(
				helper.GetContext(),
				writer,
				sdk.GetAPIAPI(),
				requestPageSize,
				opts.includeChildResources,
			); err != nil {
				return err
			}
		case "application_auth_strategies":
			if err := dumpAppAuthStrategies(
				helper.GetContext(),
				writer,
				sdk.GetAppAuthStrategiesAPI(),
				requestPageSize,
			); err != nil {
				return err
			}
		case "control_planes":
			if err := dumpControlPlanes(
				helper.GetContext(),
				writer,
				sdk.GetControlPlaneAPI(),
				requestPageSize,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

var resourceTypeMap = map[string]string{
	"portal":               "konnect_portal",
	"portal_page":          "konnect_portal_page",
	"portal_settings":      "konnect_portal_settings",
	"portal_snippet":       "konnect_portal_snippet",
	"portal_custom_domain": "konnect_portal_custom_domain",
	"portal_auth_settings": "konnect_portal_auth",
	"portal_customization": "konnect_portal_customization",
	"api":                  "konnect_api",
	"api_document":         "konnect_api_document",
	"api_specification":    "konnect_api_specification",
	"api_publication":      "konnect_api_publication",
	"api_implementation":   "konnect_api_implementation",
	"app-auth-strategies":  "konnect_application_auth_strategy",
	"control_plane":        "konnect_control_plane",
}

var (
	reTerraformNonIdentifier   = regexp.MustCompile(`[^a-z0-9_]`)
	reTerraformLeadingAlpha    = regexp.MustCompile(`^[a-z]`)
	reTerraformMultiUnderscore = regexp.MustCompile(`__+`)
)

func sanitizeTerraformResourceName(name string) string {
	name = strings.ToLower(name)

	name = reTerraformNonIdentifier.ReplaceAllString(name, "_")

	if len(name) > 0 && !reTerraformLeadingAlpha.MatchString(name) {
		name = "resource_" + name
	}

	name = reTerraformMultiUnderscore.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")
	if name == "" {
		name = "resource"
	}
	return name
}

func escapeTerraformString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

func formatIDBlock(resourceID, parentIDKey, parentID string) string {
	if parentIDKey != "" && parentID != "" {
		return fmt.Sprintf("  id = jsonencode({\n    \"id\": \"%s\",\n    \"%s\": \"%s\"\n  })",
			escapeTerraformString(resourceID), parentIDKey, escapeTerraformString(parentID))
	}
	return fmt.Sprintf("  id = \"%s\"", escapeTerraformString(resourceID))
}

func formatTerraformImport(resourceType, resourceName, resourceID string, parentIDKey string, parentID string) string {
	terraformType, ok := resourceTypeMap[resourceType]
	if !ok {
		terraformType = "unknown_" + resourceType
	}

	safeName := sanitizeTerraformResourceName(resourceName)
	idBlock := formatIDBlock(resourceID, parentIDKey, parentID)

	var importLines []string
	importLines = append(importLines, "import {")
	importLines = append(importLines, fmt.Sprintf("  to = %s.%s", terraformType, safeName))

	if resourceType != "app-auth-strategies" {
		importLines = append(importLines, "  provider = konnect-beta")
	}

	importLines = append(importLines, idBlock)
	importLines = append(importLines, "}")

	return strings.Join(importLines, "\n") + "\n"
}

func formatTerraformImportForAPIPublication(resourceType, resourceName, apiID, portalID string) string {
	terraformType, ok := resourceTypeMap[resourceType]
	if !ok {
		terraformType = "unknown_" + resourceType
	}

	safeName := sanitizeTerraformResourceName(resourceName)
	providerName := "konnect-beta"
	idBlock := fmt.Sprintf("  id = jsonencode({\n    \"api_id\": \"%s\",\n    \"portal_id\": \"%s\"\n  })",
		escapeTerraformString(apiID), escapeTerraformString(portalID))

	debugf("Formatted API Publication import block with api_id=%s and portal_id=%s",
		apiID, portalID)

	return fmt.Sprintf("import {\n  to = %s.%s\n  provider = %s\n%s\n}\n",
		terraformType, safeName, providerName, idBlock)
}

func dumpPortals(
	ctx context.Context,
	writer io.Writer,
	kkClient helpers.PortalAPI,
	requestPageSize int64,
	includeChildResources bool,
) error {
	return processPaginatedRequests(func(pageNumber int64) (bool, error) {
		req := kkOps.ListPortalsRequest{
			PageSize:   Int64(requestPageSize),
			PageNumber: Int64(pageNumber),
		}

		res, err := kkClient.ListPortals(ctx, req)
		if err != nil {
			return false, fmt.Errorf("failed to list portals: %w", err)
		}

		if res.ListPortalsResponse == nil || len(res.ListPortalsResponse.Data) == 0 {
			return false, nil
		}

		for _, portal := range res.ListPortalsResponse.Data {
			importBlock := formatTerraformImport("portal", portal.Name, portal.ID, "", "")
			if _, err := fmt.Fprintln(writer, importBlock); err != nil {
				return false, fmt.Errorf("failed to write portal import block: %w", err)
			}

			if includeChildResources {
				if err := dumpPortalChildResources(ctx, writer, kkClient, portal.ID, portal.Name, requestPageSize); err != nil {
					fmt.Fprintf(
						os.Stderr,
						"Warning: Failed to dump child resources for portal %s: %v\n",
						portal.Name,
						err,
					)
				}
			}
		}

		return true, nil
	})
}

func dumpAPIs(
	ctx context.Context,
	writer io.Writer,
	kkClient helpers.APIAPI,
	requestPageSize int64,
	includeChildResources bool,
) error {
	debugf("dumpAPIs called, includeChildResources=%v", includeChildResources)

	if kkClient == nil {
		debugf("APIAPI client is nil")
		return fmt.Errorf("APIAPI client is nil")
	}

	return processPaginatedRequests(func(pageNumber int64) (bool, error) {
		req := kkOps.ListApisRequest{
			PageSize:   Int64(requestPageSize),
			PageNumber: Int64(pageNumber),
		}

		res, err := kkClient.ListApis(ctx, req)
		if err != nil {
			return false, fmt.Errorf("failed to list APIs: %w", err)
		}

		if res == nil || res.ListAPIResponse == nil || len(res.ListAPIResponse.Data) == 0 {
			return false, nil
		}

		for _, api := range res.ListAPIResponse.Data {
			importBlock := formatTerraformImport("api", api.Name, api.ID, "", "")
			if _, err := fmt.Fprintln(writer, importBlock); err != nil {
				return false, fmt.Errorf("failed to write API import block: %w", err)
			}

			if includeChildResources {
				if err := dumpAPIChildResources(ctx, writer, kkClient, api.ID, api.Name); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to dump child resources for API %s: %v\n", api.Name, err)
				}
			}
		}

		params := paginationParams{
			pageSize:   requestPageSize,
			pageNumber: pageNumber,
			totalItems: res.ListAPIResponse.Meta.Page.Total,
		}
		return params.hasMorePages(), nil
	})
}

func dumpAPIChildResources(
	ctx context.Context,
	writer io.Writer,
	kkClient helpers.APIAPI,
	apiID string,
	apiName string,
) error {
	var logger *slog.Logger
	loggerValue := ctx.Value(log.LoggerKey)
	if loggerValue != nil {
		logger = loggerValue.(*slog.Logger)
	}

	if logger != nil {
		logger.Debug("dumping API child resources", "api_id", apiID, "api_name", apiName)
	}

	debugf("Attempting to get the API client services")

	sdk, ok := kkClient.(*helpers.APIAPIImpl)
	if !ok {
		err := fmt.Errorf("failed to convert API client to public API client")
		if logger != nil {
			logger.Error("failed to convert API client", "error", err)
		}
		debugf("Could not convert kkClient to APIAPIImpl")
		return err
	}

	if logger != nil {
		logger.Debug("successfully obtained APIAPIImpl", "sdk_nil", sdk.SDK == nil)
	}

	debugf("Successfully converted to APIAPIImpl")

	if sdk.SDK == nil {
		debugf("APIAPIImpl.SDK is nil")
		return fmt.Errorf("public SDK is nil")
	}

	if sdk.SDK.APIDocumentation != nil {
		apiDocAPI := &helpers.APIDocumentAPIImpl{SDK: sdk.SDK}
		documents, err := helpers.GetDocumentsForAPI(ctx, apiDocAPI, apiID)
		if err != nil {
			if logger != nil {
				logger.Warn("failed to get documents for API", "api_id", apiID, "error", err)
			}
			debugf("Error fetching API documents: %v", err)
		} else {
			if logger != nil {
				logger.Debug("retrieved API documents", "api_id", apiID, "document_count", len(documents))
			}

			if len(documents) > 0 {
				for i, docInterface := range documents {
					if logger != nil {
						logger.Debug("processing document", "index", i, "doc_type", fmt.Sprintf("%T", docInterface))
					}

					docID := ""
					docName := ""

					doc, ok := docInterface.(map[string]any)
					if ok {
						docID, _ = doc["id"].(string)
						docName, _ = doc["name"].(string)
					} else {
						docBytes, err := json.Marshal(docInterface)
						if err == nil {
							var docMap map[string]any
							if err := json.Unmarshal(docBytes, &docMap); err == nil {
								for k, v := range docMap {
									switch strings.ToLower(k) {
									case "id":
										if strValue, ok := v.(string); ok {
											docID = strValue
										}
									case "name":
										if strValue, ok := v.(string); ok {
											docName = strValue
										}
									}
								}
							}
						}
					}

					if docID == "" {
						if logger != nil {
							logger.Warn("document missing ID", "index", i)
						}
						continue
					}

					var resourceName string
					if docName != "" {
						resourceName = fmt.Sprintf("%s_%s", apiName, docName)
					} else {
						resourceName = fmt.Sprintf("%s_doc_%s", apiName, docID[:8])
					}

					importBlock := formatTerraformImport("api_document", resourceName, docID, "api_id", apiID)
					if _, err := fmt.Fprintln(writer, importBlock); err != nil {
						if logger != nil {
							logger.Error("failed to write API document import block", "error", err)
						}
						return fmt.Errorf("failed to write API document import block: %w", err)
					}
				}
			} else {
				if logger != nil {
					logger.Info("no API documents found for API", "api_id", apiID, "api_name", apiName)
				}
			}
		}
	} else {
		if logger != nil {
			logger.Warn("SDK.APIDocumentation is nil, skipping API documents")
		}
	}

	if sdk.SDK.APIVersion != nil {
		apiVersionAPI := &helpers.APIVersionAPIImpl{SDK: sdk.SDK}
		versions, err := helpers.GetVersionsForAPI(ctx, apiVersionAPI, apiID)
		if err != nil {
			if logger != nil {
				logger.Warn("failed to get versions for API", "api_id", apiID, "error", err)
			}
			debugf("Error fetching API versions: %v", err)
		} else {
			if logger != nil {
				logger.Debug("retrieved API versions", "api_id", apiID, "version_count", len(versions))
			}

			if len(versions) > 0 {
				for i, versionInterface := range versions {
					if logger != nil {
						logger.Debug("processing version", "index", i, "version_type", fmt.Sprintf("%T", versionInterface))
					}

					versionID := ""
					versionName := ""

					version, ok := versionInterface.(map[string]any)
					if ok {
						versionID, _ = version["id"].(string)
						versionName, _ = version["version"].(string)
					} else {
						versionBytes, err := json.Marshal(versionInterface)
						if err == nil {
							var versionMap map[string]any
							if err := json.Unmarshal(versionBytes, &versionMap); err == nil {
								for k, v := range versionMap {
									switch strings.ToLower(k) {
									case "id":
										if strValue, ok := v.(string); ok {
											versionID = strValue
										}
									case "version":
										if strValue, ok := v.(string); ok {
											versionName = strValue
										}
									}
								}
							}
						}
					}

					if versionID == "" {
						if logger != nil {
							logger.Warn("version missing ID", "index", i)
						}
						continue
					}

					var resourceName string
					if versionName != "" {
						resourceName = fmt.Sprintf("%s_%s", apiName, versionName)
					} else {
						resourceName = fmt.Sprintf("%s_spec_%s", apiName, versionID[:8])
					}

					importBlock := formatTerraformImport("api_specification", resourceName, versionID, "api_id", apiID)
					if _, err := fmt.Fprintln(writer, importBlock); err != nil {
						if logger != nil {
							logger.Error("failed to write API version import block", "error", err)
						}
						return fmt.Errorf("failed to write API version import block: %w", err)
					}
				}
			} else {
				if logger != nil {
					logger.Info("no API versions found for API", "api_id", apiID, "api_name", apiName)
				}
			}
		}
	} else {
		if logger != nil {
			logger.Warn("SDK.APIVersion is nil, skipping API versions")
		}
	}

	if sdk.SDK.APIPublication != nil {
		apiPubAPI := &helpers.APIPublicationAPIImpl{SDK: sdk.SDK}
		publications, err := helpers.GetPublicationsForAPI(ctx, apiPubAPI, apiID)
		if err != nil {
			if logger != nil {
				logger.Warn("failed to get publications for API", "api_id", apiID, "error", err)
			}
			debugf("Error fetching API publications: %v", err)
		} else {
			if logger != nil {
				logger.Debug("retrieved API publications", "api_id", apiID, "publication_count", len(publications))
			}

			if len(publications) > 0 {
				for i, pubInterface := range publications {
					if logger != nil {
						logger.Debug("processing publication", "index", i, "pub_type", fmt.Sprintf("%T", pubInterface))
					}

					portalID := ""

					pub, ok := pubInterface.(map[string]any)
					if ok {
						portalID, _ = pub["portal_id"].(string)
					} else {
						pubBytes, err := json.Marshal(pubInterface)
						if err == nil {
							var pubMap map[string]any
							if err := json.Unmarshal(pubBytes, &pubMap); err == nil {
								portalID, _ = pubMap["portal_id"].(string)
							}
						}
					}

					if portalID == "" {
						if logger != nil {
							logger.Warn("publication missing portal_id", "index", i, "pub_type", fmt.Sprintf("%T", pubInterface))
						}
						continue
					}

					resourceName := fmt.Sprintf("%s_pub_%s", apiName, portalID[:8])
					importBlock := formatTerraformImportForAPIPublication("api_publication", resourceName, apiID, portalID)
					if _, err := fmt.Fprintln(writer, importBlock); err != nil {
						if logger != nil {
							logger.Error("failed to write API publication import block", "error", err)
						}
						return fmt.Errorf("failed to write API publication import block: %w", err)
					}
				}
			} else {
				if logger != nil {
					logger.Info("no API publications found for API", "api_id", apiID, "api_name", apiName)
				}
			}
		}
	} else {
		if logger != nil {
			logger.Warn("SDK.APIPublication is nil, skipping API publications")
		}
	}

	if sdk.SDK.APIImplementation != nil {
		apiImplAPI := &helpers.APIImplementationAPIImpl{SDK: sdk.SDK}
		implementations, err := helpers.GetImplementationsForAPI(ctx, apiImplAPI, apiID)
		if err != nil {
			if logger != nil {
				logger.Warn("failed to get implementations for API", "api_id", apiID, "error", err)
			}
			debugf("Error fetching API implementations: %v", err)
		} else {
			if logger != nil {
				logger.Debug("retrieved API implementations", "api_id", apiID, "implementation_count", len(implementations))
			}

			if len(implementations) > 0 {
				for i, implInterface := range implementations {
					if logger != nil {
						logger.Debug("processing implementation", "index", i, "impl_type", fmt.Sprintf("%T", implInterface))
					}

					implID := ""
					implName := ""

					impl, ok := implInterface.(map[string]any)
					if ok {
						implID, _ = impl["id"].(string)
						if serviceMap, ok := impl["service"].(map[string]any); ok {
							implName, _ = serviceMap["name"].(string)
						}
					} else {
						implBytes, err := json.Marshal(implInterface)
						if err == nil {
							var implMap map[string]any
							if err := json.Unmarshal(implBytes, &implMap); err == nil {
								implID, _ = implMap["id"].(string)
								if serviceMap, ok := implMap["service"].(map[string]any); ok {
									implName, _ = serviceMap["name"].(string)
								}
							}
						}
					}

					if implID == "" {
						if logger != nil {
							logger.Warn("implementation missing ID", "index", i, "impl_type", fmt.Sprintf("%T", implInterface))
						}
						continue
					}

					if implName == "" {
						implName = fmt.Sprintf("%s_impl_%s", apiName, implID[:8])
					}

					importBlock := formatTerraformImport("api_implementation", implName, implID, "api_id", apiID)
					if _, err := fmt.Fprintln(writer, importBlock); err != nil {
						if logger != nil {
							logger.Error("failed to write API implementation import block", "error", err)
						}
						return fmt.Errorf("failed to write API implementation import block: %w", err)
					}
				}
			} else {
				if logger != nil {
					logger.Info("no API implementations found for API", "api_id", apiID, "api_name", apiName)
				}
			}
		}
	} else {
		if logger != nil {
			logger.Warn("SDK.APIImplementation is nil, skipping API implementations")
		}
	}

	return nil
}

func dumpPortalChildResources(
	ctx context.Context,
	writer io.Writer,
	kkClient helpers.PortalAPI,
	portalID string,
	portalName string,
	_ int64,
) error {
	pages, err := helpers.GetPagesForPortal(ctx, kkClient, portalID)
	if err == nil && len(pages) > 0 {
		for _, page := range pages {
			pageName := page.Name
			if pageName == "" {
				pageName = page.Slug
			}
			resourceName := fmt.Sprintf("%s_%s", portalName, pageName)
			importBlock := formatTerraformImport("portal_page", resourceName, page.ID, "portal_id", portalID)
			if _, err := fmt.Fprintln(writer, importBlock); err != nil {
				return fmt.Errorf("failed to write portal page import block: %w", err)
			}
		}
	}

	snippets, err := helpers.GetSnippetsForPortal(ctx, kkClient, portalID)
	if err == nil && len(snippets) > 0 {
		for _, snippet := range snippets {
			resourceName := fmt.Sprintf("%s_%s", portalName, snippet.Name)
			importBlock := formatTerraformImport("portal_snippet", resourceName, snippet.ID, "portal_id", portalID)
			if _, err := fmt.Fprintln(writer, importBlock); err != nil {
				return fmt.Errorf("failed to write portal snippet import block: %w", err)
			}
		}
	}

	if helpers.HasPortalSettings(ctx, kkClient, portalID) {
		resourceName := fmt.Sprintf("%s_settings", portalName)
		importBlock := formatTerraformImport("portal_settings", resourceName, portalID, "", "")
		if _, err := fmt.Fprintln(writer, importBlock); err != nil {
			return fmt.Errorf("failed to write portal settings import block: %w", err)
		}
	}

	if helpers.HasCustomDomainForPortal(ctx, kkClient, portalID) {
		resourceName := fmt.Sprintf("%s_custom_domain", portalName)
		importBlock := formatTerraformImport("portal_custom_domain", resourceName, portalID, "", "")
		if _, err := fmt.Fprintln(writer, importBlock); err != nil {
			return fmt.Errorf("failed to write portal custom domain import block: %w", err)
		}
	}

	if helpers.HasPortalAuthSettings(ctx, kkClient, portalID) {
		resourceName := fmt.Sprintf("%s_auth_settings", portalName)
		importBlock := formatTerraformImport("portal_auth_settings", resourceName, portalID, "", "")
		if _, err := fmt.Fprintln(writer, importBlock); err != nil {
			return fmt.Errorf("failed to write portal auth settings import block: %w", err)
		}
	}

	if helpers.HasPortalCustomization(ctx, kkClient, portalID) {
		resourceName := fmt.Sprintf("%s_customization", portalName)
		importBlock := formatTerraformImport("portal_customization", resourceName, portalID, "", "")
		if _, err := fmt.Fprintln(writer, importBlock); err != nil {
			return fmt.Errorf("failed to write portal customization import block: %w", err)
		}
	}

	return nil
}

func dumpAppAuthStrategies(
	ctx context.Context,
	writer io.Writer,
	kkClient helpers.AppAuthStrategiesAPI,
	requestPageSize int64,
) error {
	debugf("dumpAppAuthStrategies called")

	if kkClient == nil {
		debugf("AppAuthStrategies API client is nil")
		return fmt.Errorf("AppAuthStrategies API client is nil")
	}

	return processPaginatedRequests(func(pageNumber int64) (bool, error) {
		req := kkOps.ListAppAuthStrategiesRequest{
			PageSize:   Int64(requestPageSize),
			PageNumber: Int64(pageNumber),
		}

		res, err := kkClient.ListAppAuthStrategies(ctx, req)
		if err != nil {
			return false, fmt.Errorf("failed to list app auth strategies: %w", err)
		}

		if res == nil || res.ListAppAuthStrategiesResponse == nil ||
			len(res.ListAppAuthStrategiesResponse.Data) == 0 {
			return false, nil
		}

		for _, strategy := range res.ListAppAuthStrategiesResponse.Data {
			strategyID, strategyName, ok := extractResourceFields(strategy, "app-auth-strategy")
			if !ok {
				continue
			}

			resourceName := strategyName
			if resourceName == "" {
				resourceName = fmt.Sprintf("strategy_%s", strategyID[:8])
			}

			importBlock := formatTerraformImport("app-auth-strategies", resourceName, strategyID, "", "")
			if _, err := fmt.Fprintln(writer, importBlock); err != nil {
				return false, fmt.Errorf("failed to write app auth strategy import block: %w", err)
			}
		}

		params := paginationParams{
			pageSize:   requestPageSize,
			pageNumber: pageNumber,
			totalItems: res.ListAppAuthStrategiesResponse.Meta.Page.Total,
		}
		return params.hasMorePages(), nil
	})
}

func dumpControlPlanes(
	ctx context.Context,
	writer io.Writer,
	kkClient helpers.ControlPlaneAPI,
	requestPageSize int64,
) error {
	if kkClient == nil {
		return fmt.Errorf("control plane API client is nil")
	}

	return processPaginatedRequests(func(pageNumber int64) (bool, error) {
		req := kkOps.ListControlPlanesRequest{
			PageSize:   Int64(requestPageSize),
			PageNumber: Int64(pageNumber),
		}

		res, err := kkClient.ListControlPlanes(ctx, req)
		if err != nil {
			return false, fmt.Errorf("failed to list control planes: %w", err)
		}

		if res == nil || res.ListControlPlanesResponse == nil || len(res.ListControlPlanesResponse.Data) == 0 {
			return false, nil
		}

		for _, cp := range res.ListControlPlanesResponse.Data {
			importBlock := formatTerraformImport("control_plane", cp.Name, cp.ID, "", "")
			if _, err := fmt.Fprintln(writer, importBlock); err != nil {
				return false, fmt.Errorf("failed to write control plane import block: %w", err)
			}
		}

		params := paginationParams{
			pageSize:   requestPageSize,
			pageNumber: pageNumber,
			totalItems: res.ListControlPlanesResponse.Meta.Page.Total,
		}
		return params.hasMorePages(), nil
	})
}

func extractResourceFields(resource any, resourceType string) (id string, name string, ok bool) {
	if resMap, isMap := resource.(map[string]any); isMap {
		id, _ = resMap["id"].(string)
		name, _ = resMap["name"].(string)
		return id, name, id != ""
	}

	resBytes, err := json.Marshal(resource)
	if err != nil {
		debugf("Failed to marshal %s: %v", resourceType, err)
		return "", "", false
	}

	debugf("Successfully serialized %s: %s", resourceType, string(resBytes))

	var resMap map[string]any
	if err := json.Unmarshal(resBytes, &resMap); err != nil {
		debugf("Failed to unmarshal %s: %v", resourceType, err)
		return "", "", false
	}

	for k, v := range resMap {
		switch strings.ToLower(k) {
		case "id":
			id, _ = v.(string)
		case "name":
			name, _ = v.(string)
		}
	}

	return id, name, id != ""
}

func debugf(_ string, _ ...any) {}
