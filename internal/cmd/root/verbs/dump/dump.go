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

	kkSDK "github.com/Kong/sdk-konnect-go"
	kkOps "github.com/Kong/sdk-konnect-go/models/operations"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/common"
	konnectCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/log"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
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

		# Export with debug logging enabled (use --log-level=debug)
		%[1]s dump --resources=api --include-child-resources --log-level=debug
		`, meta.CLIName)))

	resources             string
	includeChildResources bool
	outputFile            string
	dumpFormat            = cmd.NewEnum([]string{"tf-imports"}, "tf-imports")
)

// Maps resource types to their corresponding Terraform resource types
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

// formatIDBlock formats the ID block for import statements
func formatIDBlock(resourceID, parentIDKey, parentID string) string {
	if parentIDKey != "" && parentID != "" {
		return fmt.Sprintf("  id = jsonencode({\n    \"id\": \"%s\",\n    \"%s\": \"%s\"\n  })",
			escapeTerraformString(resourceID), parentIDKey, escapeTerraformString(parentID))
	}
	return fmt.Sprintf("  id = \"%s\"", escapeTerraformString(resourceID))
}

// formatTerraformImport creates a Terraform import block
func formatTerraformImport(resourceType, resourceName, resourceID string, parentIDKey string, parentID string) string {
	terraformType, ok := resourceTypeMap[resourceType]
	if !ok {
		terraformType = "unknown_" + resourceType
	}

	safeName := sanitizeTerraformResourceName(resourceName)
	idBlock := formatIDBlock(resourceID, parentIDKey, parentID)

	// Build import block components
	var importLines []string
	importLines = append(importLines, "import {")
	importLines = append(importLines, fmt.Sprintf("  to = %s.%s", terraformType, safeName))

	// Add provider for all resources except app-auth-strategies
	if resourceType != "app-auth-strategies" {
		importLines = append(importLines, "  provider = konnect-beta")
	}

	importLines = append(importLines, idBlock)
	importLines = append(importLines, "}")

	return strings.Join(importLines, "\n") + "\n"
}

// formatTerraformImportForAPIPublication creates a Terraform import block specifically for API Publications
// API Publications use a composite key with both api_id and portal_id
func formatTerraformImportForAPIPublication(resourceType, resourceName, apiID, portalID string) string {
	terraformType, ok := resourceTypeMap[resourceType]
	if !ok {
		terraformType = "unknown_" + resourceType
	}

	safeName := sanitizeTerraformResourceName(resourceName)

	// For the import block, we always add a provider reference
	providerName := "konnect-beta"

	// API Publications use a different format for the ID with both api_id and portal_id
	idBlock := fmt.Sprintf("  id = jsonencode({\n    \"api_id\": \"%s\",\n    \"portal_id\": \"%s\"\n  })",
		escapeTerraformString(apiID), escapeTerraformString(portalID))

	debugf("Formatted API Publication import block with api_id=%s and portal_id=%s",
		apiID, portalID)

	return fmt.Sprintf("import {\n  to = %s.%s\n  provider = %s\n%s\n}\n",
		terraformType, safeName, providerName, idBlock)
}

// Helper function for the internal SDK
func Int64(v int64) *int64 {
	return &v
}

// debugf prints a debug message - kept for compatibility but does nothing
// TODO: Remove all debugf calls as proper slog logging is now available via context
func debugf(_ string, _ ...interface{}) {
	// No-op: Debug logging should use slog from context instead
}

// paginationHandler defines a function that performs paginated requests
type paginationHandler func(pageNumber int64) (hasMoreData bool, err error)

// processPaginatedRequests handles pagination logic for any paginated API
func processPaginatedRequests(handler paginationHandler) error {
	pageNumber := int64(1)

	for {
		hasMore, err := handler(pageNumber)
		if err != nil {
			return err
		}

		if !hasMore {
			break
		}

		pageNumber++
	}

	return nil
}

// paginationParams holds common pagination parameters
type paginationParams struct {
	pageSize   int64
	pageNumber int64
	totalItems float64
}

// hasMorePages checks if there are more pages to fetch
func (p paginationParams) hasMorePages() bool {
	return p.totalItems > float64(p.pageSize*p.pageNumber)
}

// extractResourceFields attempts to extract ID and Name fields from a generic resource interface
func extractResourceFields(resource interface{}, resourceType string) (id string, name string, ok bool) {
	// First try direct map access
	if resMap, isMap := resource.(map[string]interface{}); isMap {
		id, _ = resMap["id"].(string)
		name, _ = resMap["name"].(string)
		return id, name, true
	}

	// Try JSON marshaling approach
	resBytes, err := json.Marshal(resource)
	if err != nil {
		debugf("Failed to marshal %s: %v", resourceType, err)
		return "", "", false
	}

	debugf("Successfully serialized %s: %s", resourceType, string(resBytes))

	var resMap map[string]interface{}
	if err := json.Unmarshal(resBytes, &resMap); err != nil {
		debugf("Failed to unmarshal %s: %v", resourceType, err)
		return "", "", false
	}

	// Try to get ID and Name fields (case-insensitive)
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

// dumpPortals exports all portals as Terraform import blocks
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
			// Write the portal import block
			importBlock := formatTerraformImport("portal", portal.Name, portal.ID, "", "")
			if _, err := fmt.Fprintln(writer, importBlock); err != nil {
				return false, fmt.Errorf("failed to write portal import block: %w", err)
			}

			// If includeChildResources is true, dump the child resources as well
			if includeChildResources {
				if err := dumpPortalChildResources(ctx, writer, kkClient, portal.ID, portal.Name, requestPageSize); err != nil {
					// Log error but continue with other portals
					fmt.Fprintf(os.Stderr, "Warning: Failed to dump child resources for portal %s: %v\n", portal.Name, err)
				}
			}
		}

		return true, nil // Continue to next page
	})
}

// dumpAPIs exports all APIs as Terraform import blocks
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

	// Check what kind of client we have
	_, isPublicAPI := kkClient.(*helpers.APIAPIImpl)
	debugf("kkClient is APIAPIImpl: %v", isPublicAPI)

	return processPaginatedRequests(func(pageNumber int64) (bool, error) {
		// Create a request to list APIs with pagination
		req := kkOps.ListApisRequest{
			PageSize:   Int64(requestPageSize),
			PageNumber: Int64(pageNumber),
		}

		// Call the SDK's ListApis method
		res, err := kkClient.ListApis(ctx, req)
		if err != nil {
			return false, fmt.Errorf("failed to list APIs: %w", err)
		}

		// Check if we have data in the response
		if res == nil || res.ListAPIResponse == nil || len(res.ListAPIResponse.Data) == 0 {
			return false, nil
		}

		// Process each API in the response
		for _, api := range res.ListAPIResponse.Data {
			// Write the API import block
			importBlock := formatTerraformImport("api", api.Name, api.ID, "", "")
			if _, err := fmt.Fprintln(writer, importBlock); err != nil {
				return false, fmt.Errorf("failed to write API import block: %w", err)
			}

			// If includeChildResources is true, dump the child resources as well
			if includeChildResources {
				if err := dumpAPIChildResources(ctx, writer, kkClient, api.ID, api.Name); err != nil {
					// Log error but continue with other APIs
					fmt.Fprintf(os.Stderr, "Warning: Failed to dump child resources for API %s: %v\n", api.Name, err)
				}
			}
		}

		// If we've fetched all the data, stop
		params := paginationParams{
			pageSize:   requestPageSize,
			pageNumber: pageNumber,
			totalItems: res.ListAPIResponse.Meta.Page.Total,
		}
		return params.hasMorePages(), nil
	})
}

// dumpAPIChildResources exports all child resources of an API as Terraform import blocks
func dumpAPIChildResources(
	ctx context.Context,
	writer io.Writer,
	kkClient helpers.APIAPI,
	apiID string,
	apiName string,
) error {
	// Get logger from context if available
	var logger *slog.Logger
	loggerValue := ctx.Value(log.LoggerKey)
	if loggerValue != nil {
		logger = loggerValue.(*slog.Logger)
	}

	if logger != nil {
		logger.Debug("dumping API child resources", "api_id", apiID, "api_name", apiName)
	}

	// Get the SDK
	debugf("Attempting to get the API client services")

	// Try to convert to get the public SDK
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

	// Check if SDK is nil
	if sdk.SDK == nil {
		debugf("APIAPIImpl.SDK is nil")
		return fmt.Errorf("public SDK is nil")
	}

	// Process API Documents
	// Let's check if the SDK has a valid APIDocumentation field
	if sdk.SDK.APIDocumentation == nil {
		debugf("APIAPIImpl.SDK.APIDocumentation is nil")
		if logger != nil {
			logger.Warn("SDK.APIDocumentation is nil, skipping API documents")
		}
	} else {
		// Create an API document client using the existing SDK reference
		debugf("Creating API document client directly")
		apiDocAPI := &helpers.APIDocumentAPIImpl{SDK: sdk.SDK}
		debugf("Successfully obtained API document client")

		if logger != nil {
			logger.Debug("created API document client", "api_doc_api_nil", apiDocAPI == nil)
		}

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

					// Convert the interface{} to a map to access its properties
					// The SDK returns API document entries as generic objects
					debugf("Processing document %d, type: %T, value: %+v", i, docInterface, docInterface)

					// Try different approaches to access the document data
					docID := ""
					docName := ""

					// First try to access as a map
					doc, ok := docInterface.(map[string]interface{})
					if ok {
						debugf("Successfully converted document to map")
						docID, _ = doc["id"].(string)
						docName, _ = doc["name"].(string)
						debugf("From map - Document ID: %s, Name: %s", docID, docName)
					} else {
						debugf("Could not convert document to map, trying to decode it")

						// Try to serialize and deserialize the document
						docBytes, err := json.Marshal(docInterface)
						if err == nil {
							debugf("Successfully serialized document: %s", string(docBytes))

							// Try to unmarshal into a simple map
							var docMap map[string]interface{}
							if err := json.Unmarshal(docBytes, &docMap); err == nil {
								debugf("Successfully unmarshaled document to map")

								// Try to get the id/ID and name/Name fields
								for k, v := range docMap {
									lowercaseKey := strings.ToLower(k)
									switch lowercaseKey {
									case "id":
										if strValue, ok := v.(string); ok {
											docID = strValue
											debugf("Found ID field: %s", docID)
										}
									case "name":
										if strValue, ok := v.(string); ok {
											docName = strValue
											debugf("Found Name field: %s", docName)
										}
									}
								}
							}
						}

						if docID == "" {
							debugf("Failed to extract ID from document, doc type: %T", docInterface)
							if logger != nil {
								logger.Warn("failed to extract document ID", "index", i, "doc_type", fmt.Sprintf("%T", docInterface))
							}
							continue
						}
					}

					if docID == "" {
						debugf("Could not extract document ID")
						if logger != nil {
							logger.Warn("document missing ID", "index", i)
						}
						continue
					}

					debugf("Successfully extracted document ID: %s, Name: %s", docID, docName)

					if logger != nil {
						logger.Debug("document details", "id", docID, "name", docName)
					}

					// Use the document name if available, otherwise use a generic name
					var resourceName string
					if docName != "" {
						resourceName = fmt.Sprintf("%s_%s", apiName, docName)
					} else {
						resourceName = fmt.Sprintf("%s_doc_%s", apiName, docID[:8]) // Use first 8 chars of ID as identifier
					}

					// Format and write the import block with composite key
					importBlock := formatTerraformImport("api_document", resourceName, docID, "api_id", apiID)
					if logger != nil {
						logger.Debug("writing import block", "resource_name", resourceName, "doc_id", docID, "api_id", apiID)
					}

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
	}

	// Process API Versions (formerly Specifications)
	// Let's check if the SDK has a valid APIVersion field
	if sdk.SDK.APIVersion == nil {
		debugf("APIAPIImpl.SDK.APIVersion is nil")
		if logger != nil {
			logger.Warn("SDK.APIVersion is nil, skipping API versions")
		}
	} else {
		// Create an API version client using the existing SDK reference
		debugf("Creating API version client directly")
		apiVersionAPI := &helpers.APIVersionAPIImpl{SDK: sdk.SDK}
		debugf("Successfully obtained API version client")

		if logger != nil {
			logger.Debug("created API version client", "api_version_api_nil", apiVersionAPI == nil)
		}

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

					// Convert the interface{} to a map to access its properties
					// The SDK returns API version entries as generic objects
					debugf("Processing version %d, type: %T, value: %+v", i, versionInterface, versionInterface)

					// Try different approaches to access the version data
					versionID := ""
					versionName := ""

					// First try to access as a map
					version, ok := versionInterface.(map[string]interface{})
					if ok {
						debugf("Successfully converted version to map")
						versionID, _ = version["id"].(string)
						// For API versions, the name might be in the "version" field
						versionName, _ = version["version"].(string)
						debugf("From map - Version ID: %s, Version: %s", versionID, versionName)
					} else {
						debugf("Could not convert version to map, trying to decode it")

						// Try to serialize and deserialize the version
						versionBytes, err := json.Marshal(versionInterface)
						if err == nil {
							debugf("Successfully serialized version: %s", string(versionBytes))

							// Try to unmarshal into a simple map
							var versionMap map[string]interface{}
							if err := json.Unmarshal(versionBytes, &versionMap); err == nil {
								debugf("Successfully unmarshaled version to map")

								// Try to get the id/ID and version fields
								for k, v := range versionMap {
									lowercaseKey := strings.ToLower(k)
									switch lowercaseKey {
									case "id":
										if strValue, ok := v.(string); ok {
											versionID = strValue
											debugf("Found ID field: %s", versionID)
										}
									case "version":
										if strValue, ok := v.(string); ok {
											versionName = strValue
											debugf("Found version field: %s", versionName)
										}
									}
								}
							}
						}

						if versionID == "" {
							debugf("Failed to extract ID from version, version type: %T", versionInterface)
							if logger != nil {
								logger.Warn("failed to extract version ID", "index", i, "version_type", fmt.Sprintf("%T", versionInterface))
							}
							continue
						}
					}

					if versionID == "" {
						debugf("Could not extract version ID")
						if logger != nil {
							logger.Warn("version missing ID", "index", i)
						}
						continue
					}

					debugf("Successfully extracted version ID: %s, Version: %s", versionID, versionName)

					if logger != nil {
						logger.Debug("version details", "id", versionID, "version", versionName)
					}

					// Use the version name if available, otherwise use a generic name
					var resourceName string
					if versionName != "" {
						resourceName = fmt.Sprintf("%s_%s", apiName, versionName)
					} else {
						resourceName = fmt.Sprintf("%s_spec_%s", apiName, versionID[:8]) // Use first 8 chars of ID as identifier
					}

					// Format and write the import block with composite key
					// Note: We still use "api_specification" as the resource type for backwards compatibility
					importBlock := formatTerraformImport("api_specification", resourceName, versionID, "api_id", apiID)
					if logger != nil {
						logger.Debug("writing import block", "resource_name", resourceName, "version_id", versionID, "api_id", apiID)
					}

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
	}

	// Process API Publications
	// Let's check if the SDK has a valid APIPublication field
	if sdk.SDK.APIPublication == nil {
		debugf("InternalAPIAPI.SDK.APIPublication is nil")
		if logger != nil {
			logger.Warn("SDK.APIPublication is nil, skipping API publications")
		}
	} else {
		// Create an API publication client using the existing SDK reference
		debugf("Creating API publication client directly")
		apiPubAPI := &helpers.APIPublicationAPIImpl{SDK: sdk.SDK}
		debugf("Successfully obtained API publication client")

		if logger != nil {
			logger.Debug("created API publication client", "api_pub_api_nil", apiPubAPI == nil)
		}

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

					// Convert the interface{} to a map to access its properties
					// The SDK returns API publication entries as generic objects
					debugf("Processing publication %d, type: %T, value: %+v", i, pubInterface, pubInterface)

					// API Publications use a composite key of portal_id and api_id (no separate ID field)
					portalID := ""

					// First try to access as a map
					pub, ok := pubInterface.(map[string]interface{})
					if ok {
						debugf("Successfully converted publication to map")
						portalID, _ = pub["portal_id"].(string)
						debugf("From map - Portal ID: %s", portalID)
					} else {
						debugf("Could not convert publication to map, trying to decode it")

						// Try to serialize and deserialize the publication
						pubBytes, err := json.Marshal(pubInterface)
						if err == nil {
							debugf("Successfully serialized publication: %s", string(pubBytes))

							// Try to unmarshal into a simple map
							var pubMap map[string]interface{}
							if err := json.Unmarshal(pubBytes, &pubMap); err == nil {
								debugf("Successfully unmarshaled publication to map")

								// For publications, we need the portal_id
								portalID, _ = pubMap["portal_id"].(string)
								if portalID != "" {
									debugf("Found portal_id field: %s", portalID)
								}
							}
						}
					}

					if portalID == "" {
						debugf("Could not extract portal_id from publication, pub type: %T", pubInterface)
						if logger != nil {
							logger.Warn("publication missing portal_id", "index", i, "pub_type", fmt.Sprintf("%T", pubInterface))
						}
						continue
					}

					debugf("Successfully extracted portal ID: %s", portalID)

					if logger != nil {
						logger.Debug("publication details", "api_id", apiID, "portal_id", portalID)
					}

					// Create a resource name using the API ID and portal ID
					resourceName := fmt.Sprintf("%s_pub_%s", apiName, portalID[:8]) // Use first 8 chars of portal ID as identifier

					// For API publications, the import format is different - we need a composite key with both api_id and portal_id
					importBlock := formatTerraformImportForAPIPublication("api_publication", resourceName, apiID, portalID)
					if logger != nil {
						logger.Debug("writing import block", "resource_name", resourceName, "portal_id", portalID, "api_id", apiID)
					}

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
	}

	// Process API Implementations
	// Let's check if the SDK has a valid APIImplementation field
	if sdk.SDK.APIImplementation == nil {
		debugf("InternalAPIAPI.SDK.APIImplementation is nil")
		if logger != nil {
			logger.Warn("SDK.APIImplementation is nil, skipping API implementations")
		}
	} else {
		// Create an API implementation client using the existing SDK reference
		debugf("Creating API implementation client directly")
		apiImplAPI := &helpers.APIImplementationAPIImpl{SDK: sdk.SDK}
		debugf("Successfully obtained API implementation client")

		if logger != nil {
			logger.Debug("created API implementation client", "api_impl_api_nil", apiImplAPI == nil)
		}

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

					// Convert the interface{} to a map to access its properties
					// The SDK returns API implementation entries as generic objects
					debugf("Processing implementation %d, type: %T, value: %+v", i, implInterface, implInterface)

					// Try different approaches to access the implementation data
					implID := ""
					implName := ""

					// First try to access as a map
					impl, ok := implInterface.(map[string]interface{})
					if ok {
						debugf("Successfully converted implementation to map")
						implID, _ = impl["id"].(string)

						// For implementations, we might get the name from the service field
						if serviceMap, ok := impl["service"].(map[string]interface{}); ok {
							implName, _ = serviceMap["name"].(string)
						}

						debugf("From map - Implementation ID: %s, Name: %s", implID, implName)
					} else {
						debugf("Could not convert implementation to map, trying to decode it")

						// Try to serialize and deserialize the implementation
						implBytes, err := json.Marshal(implInterface)
						if err == nil {
							debugf("Successfully serialized implementation: %s", string(implBytes))

							// Try to unmarshal into a simple map
							var implMap map[string]interface{}
							if err := json.Unmarshal(implBytes, &implMap); err == nil {
								debugf("Successfully unmarshaled implementation to map")

								// Try to get the id field
								implID, _ = implMap["id"].(string)
								if implID != "" {
									debugf("Found ID field: %s", implID)
								}

								// For implementations, we might get the name from the service field
								if serviceMap, ok := implMap["service"].(map[string]interface{}); ok {
									implName, _ = serviceMap["name"].(string)
									if implName != "" {
										debugf("Found service.name field: %s", implName)
									}
								}
							}
						}
					}

					if implID == "" {
						debugf("Could not extract ID from implementation, impl type: %T", implInterface)
						if logger != nil {
							logger.Warn("implementation missing ID", "index", i, "impl_type", fmt.Sprintf("%T", implInterface))
						}
						continue
					}

					debugf("Successfully extracted implementation ID: %s, Service Name: %s", implID, implName)

					if logger != nil {
						logger.Debug("implementation details", "id", implID, "service_name", implName, "api_id", apiID)
					}

					// Use the service name if available, otherwise use a generic name
					var resourceName string
					if implName != "" {
						resourceName = fmt.Sprintf("%s_%s", apiName, sanitizeTerraformResourceName(implName))
					} else {
						resourceName = fmt.Sprintf("%s_impl_%s", apiName, implID[:8]) // Use first 8 chars of ID as identifier
					}

					// Format and write the import block with composite key
					importBlock := formatTerraformImport("api_implementation", resourceName, implID, "api_id", apiID)
					if logger != nil {
						logger.Debug("writing import block", "resource_name", resourceName, "impl_id", implID, "api_id", apiID)
					}

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
	_ int64, // requestPageSize - reserved for future use
) error {
	// Try to dump each type of child resource, but continue if any fail

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
			importBlock := formatTerraformImport("portal_page", resourceName, page.ID, "portal_id", portalID)
			if _, err := fmt.Fprintln(writer, importBlock); err != nil {
				return fmt.Errorf("failed to write portal page import block: %w", err)
			}
		}
	}

	// Snippets
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

	// Settings
	if helpers.HasPortalSettings(ctx, kkClient, portalID) {
		// No settings header needed

		resourceName := fmt.Sprintf("%s_settings", portalName)
		importBlock := formatTerraformImport("portal_settings", resourceName, portalID, "", "")
		if _, err := fmt.Fprintln(writer, importBlock); err != nil {
			return fmt.Errorf("failed to write portal settings import block: %w", err)
		}
	}

	// Custom Domain
	if helpers.HasCustomDomainForPortal(ctx, kkClient, portalID) {
		// No custom domain header needed

		resourceName := fmt.Sprintf("%s_custom_domain", portalName)
		importBlock := formatTerraformImport("portal_custom_domain", resourceName, portalID, "", "")
		if _, err := fmt.Fprintln(writer, importBlock); err != nil {
			return fmt.Errorf("failed to write portal custom domain import block: %w", err)
		}
	}

	// Auth Settings
	if helpers.HasPortalAuthSettings(ctx, kkClient, portalID) {
		// No auth settings header needed

		resourceName := fmt.Sprintf("%s_auth_settings", portalName)
		importBlock := formatTerraformImport("portal_auth_settings", resourceName, portalID, "", "")
		if _, err := fmt.Fprintln(writer, importBlock); err != nil {
			return fmt.Errorf("failed to write portal auth settings import block: %w", err)
		}
	}

	// Customization
	if helpers.HasPortalCustomization(ctx, kkClient, portalID) {
		// No customization header needed

		resourceName := fmt.Sprintf("%s_customization", portalName)
		importBlock := formatTerraformImport("portal_customization", resourceName, portalID, "", "")
		if _, err := fmt.Fprintln(writer, importBlock); err != nil {
			return fmt.Errorf("failed to write portal customization import block: %w", err)
		}
	}

	return nil
}

// dumpAppAuthStrategies exports all app auth strategies as Terraform import blocks
func dumpAppAuthStrategies(
	ctx context.Context,
	writer io.Writer,
	kkClient helpers.AppAuthStrategiesAPI,
	requestPageSize int64,
) error {
	debugf("dumpAppAuthStrategies called")

	if kkClient == nil {
		debugf("AppAuthStrategiesAPI client is nil")
		return fmt.Errorf("AppAuthStrategiesAPI client is nil")
	}

	return processPaginatedRequests(func(pageNumber int64) (bool, error) {
		// Create a request to list app auth strategies with pagination
		req := kkOps.ListAppAuthStrategiesRequest{
			PageSize:   kkSDK.Int64(requestPageSize),
			PageNumber: kkSDK.Int64(pageNumber),
		}

		// Call the SDK's ListAppAuthStrategies method
		res, err := kkClient.ListAppAuthStrategies(ctx, req)
		if err != nil {
			return false, fmt.Errorf("failed to list app auth strategies: %w", err)
		}

		// Check if we have data in the response
		if res == nil || res.ListAppAuthStrategiesResponse == nil ||
			len(res.ListAppAuthStrategiesResponse.Data) == 0 {
			return false, nil
		}

		// Process each app auth strategy in the response
		for _, strategy := range res.ListAppAuthStrategiesResponse.Data {
			strategyID, strategyName, ok := extractResourceFields(strategy, "app-auth-strategy")
			if !ok {
				debugf("Failed to extract app-auth-strategy fields, skipping")
				continue
			}

			debugf("Found strategy: ID=%s, Name=%s", strategyID, strategyName)

			// Use the strategy name if available, otherwise use a generic name
			resourceName := strategyName
			if resourceName == "" {
				resourceName = fmt.Sprintf("strategy_%s", strategyID[:8]) // Use first 8 chars of ID as identifier
			}

			// Write the app auth strategy import block
			importBlock := formatTerraformImport("app-auth-strategies", resourceName, strategyID, "", "")
			if _, err := fmt.Fprintln(writer, importBlock); err != nil {
				return false, fmt.Errorf("failed to write app auth strategy import block: %w", err)
			}
		}

		// If we've fetched all the data, stop
		params := paginationParams{
			pageSize:   requestPageSize,
			pageNumber: pageNumber,
			totalItems: res.ListAppAuthStrategiesResponse.Meta.Page.Total,
		}
		return params.hasMorePages(), nil
	})
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

		// Only portal, api, and app-auth-strategies are supported as top-level resources for the dump command
		// Child resources are handled automatically when --include-child-resources is true
		if resource != "portal" && resource != "api" && resource != "app-auth-strategies" {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("unsupported resource type: %s. Currently 'portal', 'api', and "+
					"'app-auth-strategies' are supported as top-level resources", resource),
			}
		}

		// Check if the resource type is known
		if _, ok := resourceTypeMap[resource]; !ok {
			supportedTypes := []string{"portal", "api", "app-auth-strategies"}
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
	// Debug mode is now handled via --log-level flag
	// Remove deprecated debug flag handling

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

		// Get the page size for all resources
		requestPageSize := int64(cfg.GetIntOrElse(
			konnectCommon.RequestPageSizeConfigPath,
			konnectCommon.DefaultRequestPageSize))

		switch resource {
		case "portal":
			// Handle portal resources
			if err := dumpPortals(
				helper.GetContext(),
				writer,
				sdk.GetPortalAPI(),
				requestPageSize,
				includeChildResources); err != nil {
				return err
			}
		case "api":
			// Handle API resources
			if err := dumpAPIs(
				helper.GetContext(),
				writer,
				sdk.GetAPIAPI(),
				requestPageSize,
				includeChildResources); err != nil {
				return err
			}
		case "app-auth-strategies":
			// Handle app auth strategy resources
			if err := dumpAppAuthStrategies(
				helper.GetContext(),
				writer,
				sdk.GetAppAuthStrategiesAPI(),
				requestPageSize); err != nil {
				return err
			}
		}
	}

	return nil
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

	// Debug logging is now controlled via --log-level flag

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
