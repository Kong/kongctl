package dump

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/pflag"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	konnectCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	"github.com/kong/kongctl/internal/config"
)

type paginationHandler func(pageNumber int64) (bool, error)

func bindFlag(cfg config.Hook, flags *pflag.FlagSet, flagName, configPath string) error {
	if f := flags.Lookup(flagName); f != nil {
		return cfg.BindFlag(configPath, f)
	}
	return nil
}

const (
	maxPaginationPages          int64 = 10000
	filterOpContains                  = "contains"
	resourceAPIs                      = "apis"
	resourceAnalyticsDashboards       = "analytics.dashboards"
)

type paginationParams struct {
	pageSize   int64
	pageNumber int64
	totalItems float64
}

func (p paginationParams) hasMorePages() bool {
	return p.totalItems > float64(p.pageSize*p.pageNumber)
}

func processPaginatedRequests(handler paginationHandler) error {
	pageNumber := int64(1)

	for {
		if pageNumber > maxPaginationPages {
			return fmt.Errorf("pagination exceeded safety limit of %d pages", maxPaginationPages)
		}

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

//go:fix inline
func Int64(v int64) *int64 {
	return new(v)
}

func getDumpWriter(helper cmdpkg.Helper, outputFile string) (io.Writer, func() error, error) {
	outputFile = strings.TrimSpace(outputFile)
	if outputFile != "" {
		outputPath, err := expandUserPath(outputFile)
		if err != nil {
			return nil, nil, err
		}

		file, err := os.Create(outputPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create output file: %w", err)
		}
		return file, file.Close, nil
	}

	return helper.GetStreams().Out, func() error { return nil }, nil
}

func expandUserPath(path string) (string, error) {
	switch {
	case path == "~":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to resolve home directory: %w", err)
		}
		return home, nil
	case strings.HasPrefix(path, "~/"):
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to resolve home directory: %w", err)
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	default:
		return path, nil
	}
}

func parseResourceList(resources string) []string {
	segments := strings.Split(resources, ",")
	result := make([]string, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment != "" {
			result = append(result, segment)
		}
	}
	return result
}

func validateResourceList(resources string, allowed map[string]struct{}) error {
	trimmed := strings.TrimSpace(resources)
	if trimmed == "" {
		return &cmdpkg.ConfigurationError{Err: fmt.Errorf("resources must have at least one valid value")}
	}

	allowedList := make([]string, 0, len(allowed))
	for key := range allowed {
		allowedList = append(allowedList, key)
	}
	sort.Strings(allowedList)

	for _, resource := range parseResourceList(resources) {
		if _, ok := allowed[resource]; !ok {
			return &cmdpkg.ConfigurationError{
				Err: fmt.Errorf("unsupported resource type: %s. Supported types: %s",
					resource, strings.Join(allowedList, ", ")),
			}
		}
	}

	return nil
}

func mapResourceName(name string) string {
	switch strings.TrimSpace(strings.ToLower(name)) {
	case "portal", "portals":
		return "portals"
	case "api", "apis":
		return resourceAPIs
	case "app-auth-strategies", "application_auth_strategies", "application-auth-strategies", "app_auth_strategies":
		return "application_auth_strategies"
	case "dcr-provider", "dcr-providers", "dcr_provider", "dcr_providers", "dcrprovider", "dcrproviders":
		return "dcr_providers"
	case "control-plane", "controlplane", "controlplanes", "control_planes":
		return "control_planes"
	case "dashboard", "dashboards", "analytics.dashboard", resourceAnalyticsDashboards:
		return resourceAnalyticsDashboards
	case "ai-gateway", "ai-gateways", "ai_gateway", "ai_gateways", "aigw":
		return "ai_gateways"
	case "ai-gateway-model", "ai-gateway-models", "ai_gateway_model", "ai_gateway_models":
		return "ai_gateway_models"
	case "org.team", "org.teams", "organization.team", "organization.teams":
		return "organization.teams"
	default:
		return name
	}
}

func normalizeResourceList(resources string, allowed map[string]struct{}) ([]string, error) {
	normalized := parseResourceList(resources)
	for i := range normalized {
		normalized[i] = mapResourceName(normalized[i])
	}

	joined := strings.Join(normalized, ",")
	if err := validateResourceList(joined, allowed); err != nil {
		return nil, err
	}

	return normalized, nil
}

const (
	filterNameFlagName = "filter-name"
	filterIDFlagName   = "filter-id"
)

type filterOptions struct {
	name string
	id   string
}

func (f filterOptions) hasFilter() bool {
	return f.name != "" || f.id != ""
}

func validateFilterOptions(f filterOptions) error {
	if f.name != "" && f.id != "" {
		return &cmdpkg.ConfigurationError{
			Err: fmt.Errorf("--%s and --%s are mutually exclusive", filterNameFlagName, filterIDFlagName),
		}
	}
	return nil
}

// parseFilterName inspects the value for leading/trailing '*' wildcards.
// If wildcards are present they are stripped and the "contains" operator
// is returned; otherwise the "eq" operator is used for exact matching.
func parseFilterName(value string) (op, val string) {
	if strings.HasPrefix(value, "*") || strings.HasSuffix(value, "*") {
		return filterOpContains, strings.Trim(value, "*")
	}
	return "eq", value
}

// filterByNameOrID applies client-side name or ID filtering to a slice of
// resources. The nameAndID function extracts the name and ID from each element.
// For name filtering, exact or contains matching is applied based on wildcards.
func filterByNameOrID[T any](items []T, filter filterOptions, nameAndID func(T) (string, string)) []T {
	if !filter.hasFilter() {
		return items
	}

	var result []T
	for _, item := range items {
		name, id := nameAndID(item)
		if filter.id != "" {
			if id == filter.id {
				result = append(result, item)
			}
		} else if filter.name != "" {
			op, val := parseFilterName(filter.name)
			if op == filterOpContains {
				if strings.Contains(name, val) {
					result = append(result, item)
				}
			} else if name == val {
				result = append(result, item)
			}
		}
	}
	return result
}

// buildStringFieldFilter creates a StringFieldFilter from a filter name value,
// using exact match by default or contains when wildcards are present.
func buildStringFieldFilter(name string) *kkComps.StringFieldFilter {
	op, val := parseFilterName(name)
	f := &kkComps.StringFieldFilter{}
	if op == filterOpContains {
		f.Contains = &val
	} else {
		f.Eq = &val
	}
	return f
}

func ensureNonNegativePageSize(helper cmdpkg.Helper) error {
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	pageSize := cfg.GetInt(konnectCommon.RequestPageSizeConfigPath)
	if pageSize < 0 {
		return &cmdpkg.ConfigurationError{
			Err: fmt.Errorf("%s must be greater than or equal to 0", konnectCommon.RequestPageSizeFlagName),
		}
	}

	return nil
}
