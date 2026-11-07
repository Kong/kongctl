package helpers_test

import (
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kk "github.com/Kong/sdk-konnect-go"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/stretchr/testify/require"
)

const enrollmentFixtureURL = "https://enrollment.example.test/start?token=private-capability"

type enrollmentFixture struct {
	t                              *testing.T
	mode                           string
	path                           string
	records                        []map[string]any
	creates, mints, deletes, lists int
}

func (f *enrollmentFixture) api() helpers.IdentityVaultEnrollmentAPI {
	transport := httpclient.NewRetryingHTTPClient(f, httpclient.RetryConfig{
		Strategy: "backoff", MaxAttempts: 3, InitialIntervalMS: 1, MaxIntervalMS: 2, BackoffFactor: 1,
	}, nil)
	sdk := kk.New(
		kk.WithClient(transport),
		kk.WithRetryConfig(retry.Config{Strategy: "backoff", Backoff: &retry.BackoffStrategy{
			InitialInterval: 1, MaxInterval: 2, Exponent: 1, MaxElapsedTime: 10,
		}}),
	)
	return (&helpers.KonnectSDK{SDK: sdk}).GetIdentityVaultEnrollmentAPI()
}

func enrollmentRecord(routing helpers.IdentityVaultCredentialRouting) map[string]any {
	value := staticCredentialRecord(writeFixturePrincipal, routing)
	value["status"] = "pending"
	return value
}

func (f *enrollmentFixture) Do(r *http.Request) (*http.Response, error) {
	t := f.t
	if r.URL.Path == providerWritePath+"/"+writeFixtureID {
		require.Equal(t, http.MethodGet, r.Method)
		value := providerWriteRecord()
		value["credential_type"] = "shared"
		switch f.mode {
		case "wrong-provider":
			value["name"] = "changed"
		case "wrong-directory":
			value["directory_id"] = writeFixturePrincipal
		case "user-provider":
			value["credential_type"] = "user"
		case "static":
			value["template_name"] = "static_secret"
		}
		return providerWriteResponse(t, r, 200, value), nil
	}
	if strings.HasPrefix(r.URL.Path, "/v2/credential-provider-templates/") {
		kind := "authorization_code"
		name := "custom-oauth"
		if f.mode == "static" {
			kind, name = "static_secret", "static_secret"
		}
		value := map[string]any{
			"name": name, "display_name": "fixture", "secret_type": kind,
			"bearer_methods_supported": []string{"header"}, "default_scopes": []string{}, "pkce_required": false,
			"token_endpoint": nil, "authorization_endpoint": nil, "registration_endpoint": nil,
		}
		return providerWriteResponse(t, r, 200, value), nil
	}
	if r.Method == http.MethodGet && r.URL.Path == staticCredentialPath {
		f.lists++
		if f.mode == "later-page-error" && r.URL.Query().Get("page[after]") != "" {
			return secretResponse(r, 403, `{"message":"private-capability"}`), nil
		}
		next := any(nil)
		if f.mode == "later-page-error" {
			next = "second"
		}
		values := f.records
		if values == nil {
			values = []map[string]any{}
		}
		if f.mode == "readback-missing" && (f.creates > 0 || f.mints > 0) {
			values = []map[string]any{}
		}
		return providerWriteResponse(t, r, 200, staticCredentialPage(values, next)), nil
	}
	if r.Method == http.MethodDelete {
		f.deletes++
		require.Equal(t, staticCredentialPath+"/"+writeFixturePrincipal, r.URL.Path)
		if f.mode == "error" {
			return secretResponse(r, 503, `{"message":"private-capability"}`), nil
		}
		if f.mode != "delete-retained" {
			f.records = nil
		}
		return secretResponse(r, 204, ""), nil
	}
	// Every capability-producing call must find its file already reserved.
	data, err := os.ReadFile(f.path)
	require.NoError(t, err)
	require.Contains(t, string(data), "creation_not_confirmed")
	require.NotContains(t, string(data), enrollmentFixtureURL)
	info, err := os.Stat(f.path)
	require.NoError(t, err)
	require.Zero(t, info.Mode().Perm()&0o077)
	var value map[string]any
	if r.Method == http.MethodPost {
		f.creates++
		require.Equal(t, staticCredentialPath, r.URL.Path)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var fields map[string]any
		require.NoError(t, json.Unmarshal(body, &fields))
		for _, field := range []string{"secret", "managed_by", "labels", "ref"} {
			require.NotContains(t, fields, field)
		}
		routing := helpers.IdentityVaultCredentialRouting{}
		if selector, ok := fields["selector"].(string); ok {
			routing.Selector = &selector
			priority := int64(fields["priority"].(float64))
			routing.Priority = &priority
		}
		value = enrollmentRecord(routing)
		f.records = []map[string]any{maps.Clone(value)}
	} else {
		f.mints++
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, staticCredentialPath+"/"+writeFixturePrincipal, r.URL.Path)
		value = maps.Clone(f.records[0])
	}
	if f.mode == "error" {
		return secretResponse(r, 503, `{"message":"private-capability"}`), nil
	}
	value["enrollment_url"] = enrollmentFixtureURL
	switch f.mode {
	case "missing-url":
		delete(value, "enrollment_url")
	case "bad-url":
		value["enrollment_url"] = "private-capability"
	case "receipt-id":
		value["id"] = writeFixtureBasicAuth
	case "receipt-routing":
		value["selector"], value["priority"] = "false", 9
	case "receipt-time":
		delete(value, "created_at")
	case "replaced-path":
		require.NoError(t, os.Remove(f.path))
		require.NoError(t, os.WriteFile(f.path, []byte("preserve-replacement"), 0o600))
	}
	status := 200
	if r.Method == http.MethodPost {
		status = 201
	}
	return providerWriteResponse(t, r, status, value), nil
}

func TestVaultEnrollmentCreateAndExplicitIssuanceCaptureOnlyToPrivateFiles(t *testing.T) {
	for _, mode := range []string{"default-create", "rule-create", "pending-issue", "active-issue"} {
		t.Run(mode, func(t *testing.T) {
			routing := helpers.IdentityVaultCredentialRouting{}
			if mode == "rule-create" || strings.HasSuffix(mode, "issue") {
				routing.Selector, routing.Priority = new("true"), new(int64(0))
			}
			f := &enrollmentFixture{t: t, path: filepath.Join(t.TempDir(), "enrollment.json")}
			var value *helpers.IdentityVaultProviderCredential
			var err error
			if strings.HasSuffix(mode, "issue") {
				f.records = []map[string]any{enrollmentRecord(routing)}
				if mode == "active-issue" {
					f.records[0]["status"] = "active"
				}
				value, err = helpers.CaptureIdentityVaultEnrollment(
					t.Context(),
					f.api(),
					staticCredentialScope(),
					writeFixturePrincipal,
					f.path,
				)
				require.Equal(t, 1, f.mints)
				require.Zero(t, f.creates)
			} else {
				value, err = helpers.CreateAndCaptureIdentityVaultEnrollment(
					t.Context(), f.api(), staticCredentialScope(), routing, f.path,
				)
				require.Equal(t, 1, f.creates)
				require.Zero(t, f.mints)
			}
			require.NoError(t, err)
			require.Equal(t, writeFixturePrincipal, value.ID)
			data, err := json.Marshal(value)
			require.NoError(t, err)
			require.NotContains(t, string(data), "private-capability")
			require.NotContains(t, string(data), "enrollment_url")
			data, err = os.ReadFile(f.path)
			require.NoError(t, err)
			var capture map[string]any
			require.NoError(t, json.Unmarshal(data, &capture))
			require.Equal(t, enrollmentFixtureURL, capture["enrollment_url"])
			require.Equal(t, "captured", capture["status"])
			require.Equal(t, writeFixtureDirectory, capture["directory_id"])
			require.Equal(t, writeFixtureID, capture["provider_id"])
			require.Equal(t, writeFixturePrincipal, capture["credential_id"])
			require.NotContains(t, capture, "secret")
			require.Zero(t, f.deletes)
		})
	}
}

func TestVaultEnrollmentCaptureFailuresNeverRetryOrExposeCapabilities(t *testing.T) {
	for _, renew := range []bool{false, true} {
		for _, mode := range []string{
			"error", "missing-url", "bad-url", "receipt-id", "receipt-routing", "receipt-time",
			"readback-missing", "replaced-path",
		} {
			t.Run(mode+map[bool]string{true: "/issue", false: "/create"}[renew], func(t *testing.T) {
				f := &enrollmentFixture{t: t, mode: mode, path: filepath.Join(t.TempDir(), "enrollment.json")}
				var value *helpers.IdentityVaultProviderCredential
				var err error
				if renew {
					f.records = []map[string]any{enrollmentRecord(helpers.IdentityVaultCredentialRouting{})}
					value, err = helpers.CaptureIdentityVaultEnrollment(
						t.Context(),
						f.api(),
						staticCredentialScope(),
						writeFixturePrincipal,
						f.path,
					)
				} else {
					value, err = helpers.CreateAndCaptureIdentityVaultEnrollment(
						t.Context(), f.api(), staticCredentialScope(), helpers.IdentityVaultCredentialRouting{}, f.path,
					)
				}
				require.Error(t, err)
				require.Nil(t, value)
				require.NotContains(t, err.Error(), "private-capability")
				require.Equal(t, 1, f.creates+f.mints, "both enabled retry layers must leave issuance single-attempt")
				require.Zero(t, f.deletes, "capture failure never triggers automatic credential retirement")
				data, readErr := os.ReadFile(f.path)
				require.NoError(t, readErr)
				if mode == "replaced-path" {
					require.Equal(t, "preserve-replacement", string(data))
				}
			})
		}
	}
}

func TestVaultEnrollmentGuardsRejectUnexpectedOrAmbiguousScopesBeforeIssuance(t *testing.T) {
	for _, mode := range []string{
		"wrong-provider", "wrong-directory", "user-provider", "static", "duplicate", "user-record",
		"unknown-status", "later-page-error", "missing-target", "occupied", "file-exists",
	} {
		t.Run(mode, func(t *testing.T) {
			f := &enrollmentFixture{t: t, mode: mode, path: filepath.Join(t.TempDir(), "enrollment.json")}
			if mode == "duplicate" {
				f.records = []map[string]any{
					enrollmentRecord(helpers.IdentityVaultCredentialRouting{}),
					enrollmentRecord(helpers.IdentityVaultCredentialRouting{}),
				}
				f.records[1]["id"] = writeFixtureBasicAuth
			}
			if mode == "user-record" {
				value := enrollmentRecord(helpers.IdentityVaultCredentialRouting{})
				value["credential_type"] = "user"
				value["status"] = "active"
				value["principal"] = map[string]any{"id": writeFixtureBasicAuth, "display_name": "User"}
				f.records = []map[string]any{value}
			}
			if mode == "unknown-status" {
				value := enrollmentRecord(helpers.IdentityVaultCredentialRouting{})
				value["status"] = "future"
				f.records = []map[string]any{value}
			}
			if mode == "occupied" || mode == "file-exists" {
				f.records = []map[string]any{enrollmentRecord(helpers.IdentityVaultCredentialRouting{})}
			}
			if mode == "file-exists" {
				require.NoError(t, os.WriteFile(f.path, []byte("preserve-existing"), 0o600))
			}
			var err error
			if mode == "missing-target" || mode == "file-exists" {
				_, err = helpers.CaptureIdentityVaultEnrollment(
					t.Context(),
					f.api(),
					staticCredentialScope(),
					writeFixturePrincipal,
					f.path,
				)
			} else {
				_, err = helpers.CreateAndCaptureIdentityVaultEnrollment(
					t.Context(), f.api(), staticCredentialScope(), helpers.IdentityVaultCredentialRouting{}, f.path,
				)
			}
			require.Error(t, err)
			require.Zero(t, f.creates+f.mints+f.deletes)
			require.NotContains(t, err.Error(), "private-capability")
			if mode == "file-exists" {
				data, err := os.ReadFile(f.path)
				require.NoError(t, err)
				require.Equal(t, "preserve-existing", string(data))
			}
		})
	}
}

func TestSharedVaultCredentialRetirementNeverMintsURLsAndVerifiesAbsence(t *testing.T) {
	for _, mode := range []string{"deleted", "delete-retained", "error"} {
		f := &enrollmentFixture{
			t:       t,
			mode:    mode,
			records: []map[string]any{enrollmentRecord(helpers.IdentityVaultCredentialRouting{})},
		}
		err := helpers.DeleteIdentitySharedVaultCredential(
			t.Context(),
			f.api(),
			staticCredentialScope(),
			writeFixturePrincipal,
		)
		if mode == "deleted" {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
			require.NotContains(t, err.Error(), "private-capability")
		}
		require.Equal(t, 1, f.deletes)
		require.Zero(t, f.creates+f.mints)
	}
}

func TestSharedVaultCredentialRetirementRejectsUserAuthorizations(t *testing.T) {
	value := enrollmentRecord(helpers.IdentityVaultCredentialRouting{})
	value["credential_type"], value["status"] = "user", "active"
	value["principal"] = map[string]any{"id": writeFixtureBasicAuth, "display_name": "User"}
	f := &enrollmentFixture{t: t, records: []map[string]any{value}}
	err := helpers.DeleteIdentitySharedVaultCredential(
		t.Context(),
		f.api(),
		staticCredentialScope(),
		writeFixturePrincipal,
	)
	require.ErrorContains(t, err, "unexpected credential type")
	require.Zero(t, f.creates+f.mints+f.deletes)
}

func TestVaultEnrollmentLocalSelectorsAndSDKWiring(t *testing.T) {
	var empty *helpers.KonnectSDK
	require.Nil(t, empty.GetIdentityVaultEnrollmentAPI())
	require.Nil(t, (&helpers.KonnectSDK{}).GetIdentityVaultEnrollmentAPI())
	_, err := helpers.CreateAndCaptureIdentityVaultEnrollment(
		t.Context(),
		nil,
		staticCredentialScope(),
		helpers.IdentityVaultCredentialRouting{},
		"new-file",
	)
	require.ErrorContains(t, err, "not configured")
	_, err = helpers.CaptureIdentityVaultEnrollment(t.Context(), nil, staticCredentialScope(), "not-uuid", "new-file")
	require.ErrorContains(t, err, "UUID")
	err = helpers.DeleteIdentitySharedVaultCredential(t.Context(), nil, staticCredentialScope(), "not-uuid")
	require.ErrorContains(t, err, "UUID")
}
