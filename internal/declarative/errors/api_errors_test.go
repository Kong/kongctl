package decerrors

import (
	"fmt"
	"testing"

	kkErrors "github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/stretchr/testify/require"
)

func TestExtractStatusCodeFromTypedAndWrappedSDKErrors(t *testing.T) {
	require.Equal(t, 404, ExtractStatusCodeFromError(fmt.Errorf("get resource: %w",
		&kkErrors.NotFoundError{Status: 404})))
	require.Equal(t, 403, ExtractStatusCodeFromError(fmt.Errorf("get resource: %w",
		&kkErrors.SDKError{StatusCode: 403})))
	require.Equal(t, 429, ExtractStatusCodeFromError(fmt.Errorf("request returned HTTP 429")))
}
