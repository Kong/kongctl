package telemetry

import (
	"context"
	"errors"
	"net"
	"net/url"

	"github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/kong/kongctl/internal/cmd"
)

// Outcome is the central, vendor-independent outcome category for a command
// execution. New values are stable contract — only add, never repurpose.
type Outcome string

const (
	OutcomeSuccess       Outcome = "success"
	OutcomeUserError     Outcome = "user_error"
	OutcomeAPIError      Outcome = "api_error"
	OutcomeNetworkError  Outcome = "network_error"
	OutcomeInternalError Outcome = "internal_error"
	OutcomeInterrupted   Outcome = "interrupted"
)

// Categorize maps an arbitrary error into a single Outcome. All categorization
// logic lives here so subcommands never need to know the taxonomy.
//
// Bands (in order of evaluation):
//   - nil                                      -> success
//   - context.Canceled                         -> interrupted
//   - context.DeadlineExceeded                 -> network_error
//   - *cmd.ConfigurationError                  -> user_error
//   - *sdkerrors.SDKError 401/403              -> user_error
//   - *sdkerrors.SDKError other 4xx, 5xx       -> api_error
//   - *url.Error / net.Error                   -> network_error
//   - *cmd.ExecutionError                      -> internal_error
//   - anything else                            -> internal_error
func Categorize(err error) Outcome {
	switch {
	case err == nil:
		return OutcomeSuccess
	case errors.Is(err, context.Canceled):
		return OutcomeInterrupted
	case errors.Is(err, context.DeadlineExceeded):
		return OutcomeNetworkError
	}

	var cfgErr *cmd.ConfigurationError
	if errors.As(err, &cfgErr) {
		return OutcomeUserError
	}

	var sdkErr *sdkerrors.SDKError
	if errors.As(err, &sdkErr) {
		switch {
		case sdkErr.StatusCode == 401 || sdkErr.StatusCode == 403:
			return OutcomeUserError
		case sdkErr.StatusCode >= 400 && sdkErr.StatusCode < 600:
			return OutcomeAPIError
		}
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return OutcomeNetworkError
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return OutcomeNetworkError
	}

	var execErr *cmd.ExecutionError
	if errors.As(err, &execErr) {
		return OutcomeInternalError
	}

	return OutcomeInternalError
}
