package helpers

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Kong/sdk-konnect-go/models/sdkerrors"
)

func sameIdentityCredentialExpiration(a, b *time.Time) bool {
	return a == nil && b == nil || a != nil && b != nil && a.Equal(*b)
}

// Error payloads can echo supplied material. Report status without retaining a
// raw SDK error as an unwrap target or formatting its body in normal output.
func identitySecretAPIError(operation string, err error) error {
	var generic *sdkerrors.SDKError
	if errors.As(err, &generic) && generic.StatusCode >= 400 && generic.StatusCode <= 599 {
		return fmt.Errorf("failed to %s: HTTP %d", operation, generic.StatusCode)
	}
	for cause := err; cause != nil; cause = errors.Unwrap(cause) {
		var body struct {
			Status int `json:"status"`
		}
		if json.Unmarshal([]byte(cause.Error()), &body) == nil && body.Status >= 400 && body.Status <= 599 {
			return fmt.Errorf("failed to %s: HTTP %d", operation, body.Status)
		}
	}
	return fmt.Errorf("failed to %s: request did not complete successfully", operation)
}
