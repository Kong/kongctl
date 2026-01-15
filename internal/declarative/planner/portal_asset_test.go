package planner

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"testing"

	kkErrors "github.com/Kong/sdk-konnect-go/models/sdkerrors"
	"github.com/stretchr/testify/require"
)

func makeDataURL(mime string, payload []byte) string {
	encoded := base64.StdEncoding.EncodeToString(payload)
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded)
}

func TestDataURLsEqual_Base64(t *testing.T) {
	t.Parallel()

	payload := []byte("hello")
	first := makeDataURL("image/png", payload)
	second := makeDataURL("image/jpeg", payload)

	equal, err := dataURLsEqual(first, second)
	require.NoError(t, err)
	require.True(t, equal)
}

func TestPortalAssetNeedsUpdate_NotFound(t *testing.T) {
	t.Parallel()

	desired := makeDataURL("image/png", []byte("asset"))
	planner := &Planner{}

	needsUpdate, err := planner.portalAssetNeedsUpdate(
		context.Background(),
		"portal-id",
		desired,
		func(_ context.Context, _ string) (string, error) {
			return "", &kkErrors.SDKError{StatusCode: http.StatusNotFound}
		},
	)

	require.NoError(t, err)
	require.True(t, needsUpdate)
}

func TestPortalAssetNeedsUpdate_NoChange(t *testing.T) {
	t.Parallel()

	payload := []byte("asset")
	desired := makeDataURL("image/png", payload)
	planner := &Planner{}

	needsUpdate, err := planner.portalAssetNeedsUpdate(
		context.Background(),
		"portal-id",
		desired,
		func(_ context.Context, _ string) (string, error) {
			return makeDataURL("image/jpeg", payload), nil
		},
	)

	require.NoError(t, err)
	require.False(t, needsUpdate)
}

func TestPortalAssetNeedsUpdate_Change(t *testing.T) {
	t.Parallel()

	desired := makeDataURL("image/png", []byte("asset"))
	planner := &Planner{}

	needsUpdate, err := planner.portalAssetNeedsUpdate(
		context.Background(),
		"portal-id",
		desired,
		func(_ context.Context, _ string) (string, error) {
			return makeDataURL("image/png", []byte("different")), nil
		},
	)

	require.NoError(t, err)
	require.True(t, needsUpdate)
}
