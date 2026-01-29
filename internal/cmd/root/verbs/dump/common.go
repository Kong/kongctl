package dump

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	konnectCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
)

type paginationHandler func(pageNumber int64) (bool, error)

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

func Int64(v int64) *int64 {
	return &v
}

func getDumpWriter(helper cmdpkg.Helper, outputFile string) (io.Writer, func() error, error) {
	if strings.TrimSpace(outputFile) != "" {
		file, err := os.Create(outputFile)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create output file: %w", err)
		}
		return file, file.Close, nil
	}

	return helper.GetStreams().Out, func() error { return nil }, nil
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
		return &cmdpkg.ConfigurationError{Err: fmt.Errorf("resources cannot be empty")}
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
		return "apis"
	case "app-auth-strategies", "application_auth_strategies", "application-auth-strategies", "app_auth_strategies":
		return "application_auth_strategies"
	case "control-plane", "controlplane", "controlplanes", "control_planes":
		return "control_planes"
	case "teams", "team":
		return "teams"
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
