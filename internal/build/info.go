package build

import "context"

type Key struct{}

var InfoKey = Key{}

type Info struct {
	Version, Commit, Date string
}

// Version returns the running binary's version, including "dev" for local builds.
func Version(ctx context.Context) string {
	if info, ok := ctx.Value(InfoKey).(*Info); ok && info != nil && info.Version != "" {
		return info.Version
	}
	return "dev"
}
