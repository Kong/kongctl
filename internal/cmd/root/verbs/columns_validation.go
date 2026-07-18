package verbs

import (
	"fmt"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/columns"
	"github.com/kong/kongctl/internal/cmd/output/jq"
	"github.com/kong/kongctl/internal/config"
)

func ValidateColumnFlags(helper cmdpkg.Helper, cfg config.Hook) error {
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}
	selected, err := columns.Resolve(helper.GetCmd(), outType)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	settings, err := jq.ResolveSettings(helper.GetCmd(), cfg)
	if err != nil {
		return err
	}
	if len(selected) > 0 && jq.HasFilter(settings) {
		return &cmdpkg.ConfigurationError{
			Err: fmt.Errorf("--%s cannot be combined with --%s", columns.FlagName, jq.FlagName),
		}
	}
	return nil
}
