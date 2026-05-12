package telemetry

import "strings"

const (
	AreaDeclarative       = "declarative"
	AreaKonnectImperative = "konnect-imperative"
	AreaAuth              = "auth"
	AreaConfig            = "config"
	AreaAuditLog          = "audit-log"
	AreaOther             = "other"
)

// AreaFor classifies a fully-qualified cobra command path
// (e.g. "kongctl get orgs") into a high-level execution area.
func AreaFor(commandPath string) string {
	fields := strings.Fields(commandPath)
	if len(fields) < 2 {
		return AreaOther
	}
	switch fields[1] {
	case "api", "apply", "sync", "diff", "plan", "adopt", "export", "dump", "patch":
		return AreaDeclarative
	case "get", "list", "create", "update", "delete", "view", "ps":
		return AreaKonnectImperative
	case "login", "logout":
		return AreaAuth
	case "config":
		return AreaConfig
	case "listen", "tail":
		return AreaAuditLog
	default:
		return AreaOther
	}
}
