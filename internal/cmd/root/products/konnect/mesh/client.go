package mesh

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/kong/kongctl/internal/cmd"
	konnectcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/common"
	meshcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh/common"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/konnect/apiutil"
	"github.com/kong/kongctl/internal/konnect/auth"
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

// Mesh talks to three kinds of destination, and only one of them may be
// presented the control plane's TLS identity. Timeout and transport behaviour
// are shared by all three, so they resolve through the same helpers the rest
// of the Konnect operations use; TLS identity and trust are scoped per
// destination.
//
// A single builder installed the control plane's client certificate, private
// CA and tls-skip-verify on every destination. A remote `-f https://...` input
// therefore presented the operator's control plane certificate to whatever
// server held the file, and accepted that server's certificate through a
// skip-verify meant for the control plane. Omitting the bearer token was not
// enough: TLS is its own credential.

// newControlPlaneClient builds the client for control plane requests. Only a
// self managed target's TLS material is installed; Konnect is reached over its
// own certificates.
func newControlPlaneClient(
	cfg config.Hook, target meshTarget, logger *slog.Logger,
) (*httpclient.LoggingHTTPClient, error) {
	clientConfig, err := controlPlaneClientConfig(cfg, target)
	if err != nil {
		return nil, err
	}
	return newLoggingClient(clientConfig, logger), nil
}

// controlPlaneClientConfig is the control plane's client settings, separated
// from the client itself so that what TLS material a target receives can be
// asserted directly.
func controlPlaneClientConfig(
	cfg config.Hook, target meshTarget,
) (httpclient.ClientConfig, error) {
	clientConfig, err := baseClientConfig(cfg)
	if err != nil {
		return httpclient.ClientConfig{}, err
	}

	if target.selfManaged {
		tlsConfig, err := selfManagedTLSConfig(cfg)
		if err != nil {
			return httpclient.ClientConfig{}, err
		}
		clientConfig.TransportOptions.TLSClientConfig = tlsConfig
	}

	return clientConfig, nil
}

// newKonnectClient builds the client for Konnect's own API, such as listing
// control planes. Konnect presents its own certificates, so no control plane
// TLS material applies.
func newKonnectClient(cfg config.Hook, logger *slog.Logger) (*httpclient.LoggingHTTPClient, error) {
	clientConfig, err := baseClientConfig(cfg)
	if err != nil {
		return nil, err
	}
	return newLoggingClient(clientConfig, logger), nil
}

// newInputClient builds the client that downloads resource documents named by
// `-f <url>`. That URL is not the control plane, so it is sent neither the
// control plane's credential nor its TLS identity, and it is not trusted
// through the control plane's skip-verify option.
func newInputClient(cfg config.Hook, logger *slog.Logger) (*httpclient.LoggingHTTPClient, error) {
	clientConfig, err := baseClientConfig(cfg)
	if err != nil {
		return nil, err
	}
	return newLoggingClient(clientConfig, logger), nil
}

func newLoggingClient(
	clientConfig httpclient.ClientConfig, logger *slog.Logger,
) *httpclient.LoggingHTTPClient {
	return httpclient.NewLoggingHTTPClientWithClient(
		httpclient.NewHTTPClientWithConfig(clientConfig), logger)
}

// meshTarget is the control plane an invocation addresses, resolved once.
//
// Whether a target is self managed decides three things at once: the base URL,
// which credential is sent, and whose TLS material applies. Deciding it twice
// from different inputs is how a self managed token reached a hosted endpoint:
// the URL resolver honoured an explicit --control-plane-id while a separate
// check asked only whether a URL happened to sit in configuration. One value,
// resolved once, keeps the three consistent.
type meshTarget struct {
	baseURL string
	// selfManaged is true when the resolved target is a control plane the
	// operator runs, which authenticates its own callers.
	selfManaged bool
}

// selfManagedTLSConfig builds the TLS settings for a self managed control
// plane, or nil when none were given and Go's defaults apply.
func selfManagedTLSConfig(cfg config.Hook) (*tls.Config, error) {
	var (
		caCertFile     = strings.TrimSpace(cfg.GetString(meshcommon.CACertFileConfigPath))
		clientCertFile = strings.TrimSpace(cfg.GetString(meshcommon.ClientCertFileConfigPath))
		clientKeyFile  = strings.TrimSpace(cfg.GetString(meshcommon.ClientKeyFileConfigPath))
		skipVerify     = cfg.GetBool(meshcommon.TLSSkipVerifyConfigPath)
	)

	if caCertFile == "" && clientCertFile == "" && clientKeyFile == "" && !skipVerify {
		return nil, nil
	}

	// A certificate without its key, or the reverse, cannot be presented, so
	// say which half is missing rather than failing the handshake later.
	if (clientCertFile == "") != (clientKeyFile == "") {
		return nil, &cmd.ConfigurationError{Err: fmt.Errorf(
			"--%s and --%s are used together; provide both",
			meshcommon.ClientCertFileFlagName, meshcommon.ClientKeyFileFlagName)}
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		// #nosec G402 -- opt in, named --tls-skip-verify, for a control plane
		// whose certificate the operator cannot yet verify.
		InsecureSkipVerify: skipVerify,
	}

	if caCertFile != "" {
		pem, err := os.ReadFile(caCertFile)
		if err != nil {
			return nil, &cmd.ConfigurationError{
				Err: fmt.Errorf("failed to read %s: %w", meshcommon.CACertFileFlagName, err),
			}
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, &cmd.ConfigurationError{Err: fmt.Errorf(
				"%s holds no PEM certificate: %s", meshcommon.CACertFileFlagName, caCertFile)}
		}
		tlsConfig.RootCAs = pool
	}

	if clientCertFile != "" {
		certificate, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
		if err != nil {
			return nil, &cmd.ConfigurationError{
				Err: fmt.Errorf("failed to load the client certificate: %w", err),
			}
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}

	return tlsConfig, nil
}

// baseClientConfig resolves the configured HTTP behaviour shared by every
// mesh destination. It carries no TLS identity: that is destination specific.
//
// Separated from the client it builds because the wrapped client keeps its
// settings private, so this is where the resolution can be asserted.
func baseClientConfig(cfg config.Hook) (httpclient.ClientConfig, error) {
	timeout, err := konnectcommon.ResolveHTTPTimeout(cfg)
	if err != nil {
		return httpclient.ClientConfig{}, err
	}

	transportOptions, err := konnectcommon.ResolveHTTPTransportOptions(cfg)
	if err != nil {
		return httpclient.ClientConfig{}, err
	}

	return httpclient.ClientConfig{
		Timeout:          timeout,
		TransportOptions: transportOptions,
	}, nil
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

	target, err := resolveTarget(helper, cfg)
	if err != nil {
		return nil, 0, err
	}

	ctx := helper.GetContext()
	if ctx == nil {
		ctx = context.Background()
	}

	// A self managed control plane authenticates its own callers, so Konnect
	// credentials are neither required nor sent. Resolving them regardless
	// meant an unauthenticated local control plane never received a request:
	// the command failed first on the missing Konnect token.
	//
	// This reads the resolved target rather than asking configuration again,
	// so an explicit --control-plane-id carries the Konnect credential even
	// when a self managed URL is also configured.
	var tokenSource *auth.TokenSource
	if !target.selfManaged {
		tokenSource, err = konnectcommon.GetAccessTokenSource(cfg, logger)
		if err != nil {
			return nil, 0, fmt.Errorf("resolve Konnect access token: %w", err)
		}
		if _, err := konnectcommon.ResolveAccessToken(ctx, cfg, tokenSource); err != nil {
			return nil, 0, fmt.Errorf("resolve Konnect access token: %w", err)
		}
	}

	var (
		payload io.Reader
		headers map[string]string
	)
	if body != nil {
		payload = bytes.NewReader(body)
		headers = map[string]string{"Content-Type": "application/json"}
	}

	client, err := newControlPlaneClient(cfg, target, logger)
	if err != nil {
		return nil, 0, err
	}

	var result *apiutil.Result
	if target.selfManaged {
		// An empty token sends no authorization header, which is what a
		// control plane reached over loopback expects: Kuma authenticates
		// such a caller as admin.
		result, err = apiutil.Request(ctx, client, method, target.baseURL, path,
			strings.TrimSpace(cfg.GetString(meshcommon.ControlPlaneTokenConfigPath)), headers, payload)
	} else {
		result, err = apiutil.RequestWithTokenSource(
			ctx, client, method, target.baseURL, path, tokenSource, headers, payload)
	}
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

// resolveTarget determines which control plane a command addresses, and
// whether it is one the operator runs.
//
// meshcommon resolves an explicit URL or an identifier from configuration
// alone. A name needs a Konnect call to become an identifier, which cannot
// live there, so it is resolved here and the identifier written back to
// configuration — leaving the composition itself in one place.
func resolveTarget(helper cmd.Helper, cfg config.Hook) (meshTarget, error) {
	// A selector named on the command line is this invocation's intent, so it
	// decides the target even when configuration names a different one. Without
	// this, a configured ID answers an explicit --control-plane-name, and since
	// this resolver also serves writes and deletes that would act on a control
	// plane the operator did not choose.
	selector, err := explicitControlPlaneSelector(helper)
	if err != nil {
		return meshTarget{}, err
	}

	// Which selector won decides self managed, not whether a URL exists in
	// configuration. An explicit ID or name addresses Konnect even when a self
	// managed URL is also configured, and it must then carry the Konnect
	// credential rather than the self managed one.
	switch selector {
	case meshcommon.ControlPlaneURLFlagName:
		baseURL, err := meshcommon.ResolveControlPlaneAPIURL(cfg)
		if err != nil {
			return meshTarget{}, err
		}
		return meshTarget{baseURL: baseURL, selfManaged: true}, nil
	case meshcommon.ControlPlaneIDFlagName:
		baseURL, err := meshcommon.ControlPlaneAPIURLForID(
			cfg, cfg.GetString(meshcommon.ControlPlaneIDConfigPath))
		if err != nil {
			return meshTarget{}, err
		}
		return meshTarget{baseURL: baseURL}, nil
	case meshcommon.ControlPlaneNameFlagName:
		baseURL, err := resolveBaseURLByName(
			helper, cfg, cfg.GetString(meshcommon.ControlPlaneNameConfigPath))
		if err != nil {
			return meshTarget{}, err
		}
		return meshTarget{baseURL: baseURL}, nil
	}

	// Nothing was named on the command line, so configuration decides in the
	// documented order: URL, then ID, then name. Only the URL branch is self
	// managed.
	baseURL, err := meshcommon.ResolveControlPlaneAPIURL(cfg)
	if err == nil {
		return meshTarget{
			baseURL:     baseURL,
			selfManaged: strings.TrimSpace(cfg.GetString(meshcommon.ControlPlaneURLConfigPath)) != "",
		}, nil
	}

	name := strings.TrimSpace(cfg.GetString(meshcommon.ControlPlaneNameConfigPath))
	if name == "" {
		// Nothing identifies a control plane, so report that rather than the
		// failure to resolve a name that was never given.
		return meshTarget{}, err
	}

	byName, err := resolveBaseURLByName(helper, cfg, name)
	if err != nil {
		return meshTarget{}, err
	}
	return meshTarget{baseURL: byName}, nil
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
