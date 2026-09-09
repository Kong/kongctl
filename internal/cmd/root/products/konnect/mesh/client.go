package mesh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kong/kongctl/internal/cmd"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	meshcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh/common"
	"github.com/kong/kongctl/internal/konnect/apiutil"
	"github.com/kong/kongctl/internal/konnect/httpclient"
)

// apiError is the error envelope Kong Mesh control planes return. It follows
// AIP-193, the same shape Konnect uses, so the control plane's own wording can
// be surfaced rather than a status code.
//
// Observed deviations from the published schema: type is a bare path such as
// "/std-errors" rather than a URI, and detail is duplicated as details. Only
// detail is read.
type apiError struct {
	Status            int                `json:"status"`
	Title             string             `json:"title"`
	Detail            string             `json:"detail"`
	Instance          string             `json:"instance"`
	InvalidParameters []invalidParameter `json:"invalid_parameters,omitempty"`
}

// invalidParameter carries field level validation feedback.
type invalidParameter struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
	Source string `json:"source"`
}

// Error renders the control plane's own description of the failure, preferring
// detail because that is the field carrying the actionable wording.
func (e apiError) Error() string {
	msg := strings.TrimSpace(e.Detail)
	if title := strings.TrimSpace(e.Title); title != "" {
		if msg == "" || strings.EqualFold(msg, title) {
			msg = title
		} else {
			msg = title + ": " + msg
		}
	}
	if msg == "" {
		msg = fmt.Sprintf("control plane returned status %d", e.Status)
	}

	for _, p := range e.InvalidParameters {
		msg += fmt.Sprintf("\n  %s (%s): %s", p.Field, p.Source, p.Reason)
	}
	return msg
}

// fetch performs a GET against the selected control plane and returns the
// response body.
func fetch(helper cmd.Helper, path string) ([]byte, error) {
	body, _, err := send(helper, http.MethodGet, path, nil)
	return body, err
}

// sendForStatus performs a request and returns only the response status, for
// callers that distinguish a created resource from a replaced one.
func sendForStatus(helper cmd.Helper, method, path string, body []byte) (int, error) {
	_, status, err := send(helper, method, path, body)
	return status, err
}

// send performs a request against the selected control plane and returns the
// response body.
//
// Every mesh call goes through here so that control plane resolution,
// credentials, and error rendering behave identically across commands.
func send(helper cmd.Helper, method, path string, body []byte) ([]byte, int, error) {
	cfg, err := helper.GetConfig()
	if err != nil {
		return nil, 0, err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return nil, 0, err
	}

	baseURL, err := meshcommon.ResolveControlPlaneAPIURL(cfg)
	if err != nil {
		return nil, 0, err
	}

	tokenSource, err := konnectcommon.GetAccessTokenSource(cfg, logger)
	if err != nil {
		return nil, 0, fmt.Errorf("resolve Konnect access token: %w", err)
	}

	ctx := helper.GetContext()
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := konnectcommon.ResolveAccessToken(ctx, cfg, tokenSource); err != nil {
		return nil, 0, fmt.Errorf("resolve Konnect access token: %w", err)
	}

	var (
		payload io.Reader
		headers map[string]string
	)
	if body != nil {
		payload = bytes.NewReader(body)
		headers = map[string]string{"Content-Type": "application/json"}
	}

	result, err := apiutil.RequestWithTokenSource(
		ctx,
		httpclient.NewLoggingHTTPClient(logger),
		method,
		baseURL,
		path,
		tokenSource,
		headers,
		payload,
	)
	if err != nil {
		return nil, 0, err
	}

	logger.Debug("mesh control plane call completed",
		"method", method, "path", path, "status_code", result.StatusCode)

	if result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusMultipleChoices {
		return nil, result.StatusCode, buildAPIError(result.StatusCode, result.Body)
	}

	return result.Body, result.StatusCode, nil
}

// buildAPIError turns a failed response into an actionable error, using the
// control plane's own envelope when it sent one.
func buildAPIError(statusCode int, body []byte) error {
	var envelope apiError
	if err := json.Unmarshal(body, &envelope); err == nil {
		if strings.TrimSpace(envelope.Detail) != "" || strings.TrimSpace(envelope.Title) != "" {
			if envelope.Status == 0 {
				envelope.Status = statusCode
			}
			return envelope
		}
	}

	// No envelope to quote, so fall back to the causes an operator can address.
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf(
			"not authorized to read the Kong Mesh control plane (status %d); "+
				"check that the credential grants access to this control plane", statusCode)
	case http.StatusNotFound:
		return fmt.Errorf(
			"control plane API not found (status %d); "+
				"check the control plane selection, and that it is running Kong Mesh 3.0 or later", statusCode)
	}

	if detail := strings.TrimSpace(string(body)); detail != "" {
		return fmt.Errorf("control plane request failed with status %d: %s", statusCode, detail)
	}
	return fmt.Errorf("control plane request failed with status %d", statusCode)
}
