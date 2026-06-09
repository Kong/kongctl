package common

import "github.com/kong/kongctl/internal/config"

// ResolveRequestPageSize returns the configured page size or the default when unset or invalid.
func ResolveRequestPageSize(cfg config.Hook) int64 {
	pageSize := int64(cfg.GetInt(RequestPageSizeConfigPath))
	if pageSize < 1 {
		return int64(DefaultRequestPageSize)
	}
	return pageSize
}

// HasMorePageNumberResults reports whether another page-number request is required.
func HasMorePageNumberResults(total, collected, pageItems int) bool {
	return total > 0 && collected < total && pageItems > 0
}
