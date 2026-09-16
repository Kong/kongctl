package mesh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/kong/kongctl/internal/cmd"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	meshcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh/common"
	"github.com/kong/kongctl/internal/config"
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

// listPageSize is the page size requested when collecting a whole collection.
// The control plane may return fewer, which is why termination is driven by the
// reported total rather than by a short page.
const listPageSize = 100

// listAll pages through a collection and returns every item.
//
// The `next` link in the response cannot be followed: on Konnect hosted
// control planes it carries an internal cluster address. Pagination is
// therefore driven by constructing offset and size directly.
func listAll(helper cmd.Helper, path string) ([]map[string]any, error) {
	return paginate(func(offset int) (listEnvelope, error) {
		query := url.Values{}
		query.Set("size", fmt.Sprint(listPageSize))
		if offset > 0 {
			query.Set("offset", fmt.Sprint(offset))
		}

		body, err := fetch(helper, path+"?"+query.Encode())
		if err != nil {
			return listEnvelope{}, err
		}

		var envelope listEnvelope
		if err := json.Unmarshal(body, &envelope); err != nil {
			return listEnvelope{}, fmt.Errorf("failed to decode %s: %w", path, err)
		}
		return envelope, nil
	})
}

// paginate collects every page, asking fetchPage for the page at each offset.
//
// The fetching is supplied by the caller so that the termination rules can be
// exercised without HTTP.
func paginate(fetchPage func(offset int) (listEnvelope, error)) ([]map[string]any, error) {
	var items []map[string]any
	offset := 0

	for {
		envelope, err := fetchPage(offset)
		if err != nil {
			return nil, err
		}

		items = append(items, envelope.Items...)

		// Termination is driven by the reported total, not by a short page:
		// the control plane caps its own page size, so a page smaller than
		// the one requested is normal and says nothing about being the last.
		// A page that returns nothing ends the loop regardless, so a control
		// plane that keeps offering a next link cannot spin here.
		if len(envelope.Items) == 0 || len(items) >= envelope.Total {
			return items, nil
		}
		offset += len(envelope.Items)
	}
}

// listPayload rebuilds a collection envelope from everything listAll
// collected, for the structured output forms that print the payload.
//
// `next` is deliberately absent: every page has already been fetched, so
// echoing a link would suggest there is more to read.
func listPayload(items []map[string]any) map[string]any {
	if items == nil {
		items = []map[string]any{}
	}
	return map[string]any{"total": len(items), "items": items}
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

// sendForWrite performs a write and returns the response status together with
// any warnings the control plane reported.
//
// A successful create or update is answered with {"warnings":[...]} carrying
// deprecation notices for the resource that was just written. They are the only
// place the control plane reports a resource it accepted but wants changed, so
// they are read here rather than discarded with the rest of the body.
func sendForWrite(helper cmd.Helper, method, path string, body []byte) (int, []string, error) {
	respBody, status, err := send(helper, method, path, body)
	if err != nil {
		return status, nil, err
	}
	return status, parseWarnings(respBody), nil
}

// parseWarnings reads the warnings from a successful write response.
//
// The warnings are advisory, so a body that is empty, is not JSON, or carries
// no warnings yields none rather than an error: failing a write that the
// control plane accepted would be worse than losing the notice.
func parseWarnings(body []byte) []string {
	if len(body) == 0 {
		return nil
	}

	var payload struct {
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil
	}

	warnings := make([]string, 0, len(payload.Warnings))
	for _, warning := range payload.Warnings {
		if trimmed := strings.TrimSpace(warning); trimmed != "" {
			warnings = append(warnings, trimmed)
		}
	}
	if len(warnings) == 0 {
		return nil
	}
	return warnings
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

	baseURL, err := resolveBaseURL(helper, cfg)
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

// resolveBaseURL determines which control plane a command addresses.
//
// meshcommon resolves an explicit URL or an identifier from configuration
// alone. A name needs a Konnect call to become an identifier, which cannot
// live there, so it is resolved here and the identifier written back to
// configuration — leaving the composition itself in one place.
func resolveBaseURL(helper cmd.Helper, cfg config.Hook) (string, error) {
	// A selector named on the command line is this invocation's intent, so it
	// decides the target even when configuration names a different one. Without
	// this, a configured ID answers an explicit --control-plane-name, and since
	// this resolver also serves writes and deletes that would act on a control
	// plane the operator did not choose.
	selector, err := explicitControlPlaneSelector(helper)
	if err != nil {
		return "", err
	}

	switch selector {
	case meshcommon.ControlPlaneURLFlagName:
		return meshcommon.ResolveControlPlaneAPIURL(cfg)
	case meshcommon.ControlPlaneIDFlagName:
		return meshcommon.ControlPlaneAPIURLForID(
			cfg, cfg.GetString(meshcommon.ControlPlaneIDConfigPath))
	case meshcommon.ControlPlaneNameFlagName:
		return resolveBaseURLByName(
			helper, cfg, cfg.GetString(meshcommon.ControlPlaneNameConfigPath))
	}

	// Nothing was named on the command line, so configuration decides in the
	// documented order: URL, then ID, then name.
	baseURL, err := meshcommon.ResolveControlPlaneAPIURL(cfg)
	if err == nil {
		return baseURL, nil
	}

	name := strings.TrimSpace(cfg.GetString(meshcommon.ControlPlaneNameConfigPath))
	if name == "" {
		// Nothing identifies a control plane, so report that rather than the
		// failure to resolve a name that was never given.
		return "", err
	}

	return resolveBaseURLByName(helper, cfg, name)
}

// resolveBaseURLByName turns a control plane name into its API URL.
//
// A name is not resolvable from configuration alone, so it is looked up against
// Konnect and the resulting ID is recorded for the rest of the invocation.
func resolveBaseURLByName(helper cmd.Helper, cfg config.Hook, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", meshcommon.ErrNoControlPlaneSelected
	}

	controlPlaneID, err := resolveControlPlaneIDByName(helper, name)
	if err != nil {
		return "", err
	}

	cfg.SetString(meshcommon.ControlPlaneIDConfigPath, controlPlaneID)
	return meshcommon.ControlPlaneAPIURLForID(cfg, controlPlaneID)
}

// controlPlaneSelectorFlags are the mutually exclusive ways to name a control
// plane, in the order configuration consults them.
var controlPlaneSelectorFlags = []string{
	meshcommon.ControlPlaneURLFlagName,
	meshcommon.ControlPlaneIDFlagName,
	meshcommon.ControlPlaneNameFlagName,
}

// explicitControlPlaneSelector reports which selector was given on the command
// line, or "" when none was.
//
// Two selectors at once is rejected rather than resolved by precedence: the
// operator has asked for two different control planes and guessing which one
// they meant is worse than making them choose.
func explicitControlPlaneSelector(helper cmd.Helper) (string, error) {
	if helper == nil {
		return "", nil
	}
	command := helper.GetCmd()
	if command == nil {
		return "", nil
	}

	var named []string
	for _, flag := range controlPlaneSelectorFlags {
		if command.Flags().Changed(flag) {
			named = append(named, flag)
		}
	}

	switch len(named) {
	case 0:
		return "", nil
	case 1:
		return named[0], nil
	default:
		quoted := make([]string, 0, len(named))
		for _, flag := range named {
			quoted = append(quoted, "--"+flag)
		}
		return "", &cmd.ConfigurationError{Err: fmt.Errorf(
			"%s select different control planes; provide only one",
			strings.Join(quoted, " and "))}
	}
}
