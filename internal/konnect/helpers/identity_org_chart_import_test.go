package helpers_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	kk "github.com/Kong/sdk-konnect-go"
	"github.com/Kong/sdk-konnect-go/retry"
	"github.com/kong/kongctl/internal/konnect/helpers"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	konglog "github.com/kong/kongctl/internal/log"
	"github.com/stretchr/testify/require"
)

const (
	orgImportDirectory = "7a5e1b58-48a4-4586-b02d-0d91eccca152"
	orgImportCSV       = "external_id,email,given_name,family_name,manager_external_id\n" +
		"private-employee,employee-private@example.test,Private,Employee,\n"
)

type orgImportFixture struct {
	t          *testing.T
	path, mode string
	dryRun     bool
	calls      int
	logs       bytes.Buffer
}

func (f *orgImportFixture) api() helpers.IdentityOrgChartImportAPI {
	logger := slog.New(slog.NewJSONHandler(&f.logs, &slog.HandlerOptions{Level: konglog.LevelTrace}))
	logging := httpclient.NewLoggingHTTPClientWithClient(&http.Client{Transport: principalTransport(f.do)}, logger)
	transport := httpclient.NewRetryingHTTPClient(logging, httpclient.RetryConfig{
		Strategy: "backoff", MaxAttempts: 3, InitialIntervalMS: 1, MaxIntervalMS: 2, BackoffFactor: 1,
	}, nil)
	sdk := kk.New(kk.WithClient(transport), kk.WithRetryConfig(retry.Config{
		Strategy: "backoff",
		Backoff:  &retry.BackoffStrategy{InitialInterval: 1, MaxInterval: 2, Exponent: 1, MaxElapsedTime: 10},
	}))
	return (&helpers.KonnectSDK{SDK: sdk}).GetIdentityOrgChartImportAPI()
}

func (f *orgImportFixture) do(request *http.Request) (*http.Response, error) {
	f.calls++
	require.Equal(f.t, http.MethodPost, request.Method)
	require.Equal(f.t, "/v2/directories/"+orgImportDirectory+"/org-chart/import", request.URL.Path)
	data, err := os.ReadFile(f.path)
	require.NoError(f.t, err)
	require.Contains(f.t, string(data), "request_not_confirmed")
	require.NotContains(f.t, string(data), "employee-private")
	info, err := os.Stat(f.path)
	require.NoError(f.t, err)
	require.Equal(f.t, os.FileMode(0o600), info.Mode().Perm())
	_, parameters, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	require.NoError(f.t, err)
	reader := multipart.NewReader(request.Body, parameters["boundary"])
	parts := make(map[string]string)
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(f.t, err)
		body, err := io.ReadAll(part)
		require.NoError(f.t, err)
		parts[part.FormName()] = string(body)
		if part.FormName() == "file" {
			require.Equal(f.t, "org-chart.csv", part.FileName())
		}
	}
	require.Equal(f.t, orgImportCSV, parts["file"])
	require.Equal(f.t, strconv.FormatBool(f.dryRun), parts["dry_run"])
	if f.mode == "transport" {
		return nil, fmt.Errorf("employee-private@example.test transport diagnostic")
	}
	status, contentType := 200, "application/json"
	value := map[string]any{
		"counts": map[string]any{
			"users":       map[string]any{"created": 0, "updated": 0, "unchanged": 0, "inactive": 0, "adopted": 1},
			"teams":       map[string]any{"created": 0, "deleted": 0},
			"memberships": map[string]any{"added": 0, "removed": 0},
		},
		"warnings": []any{map[string]any{
			"type": "principal_adopted", "row": 1,
			"detail": "employee-private@example.test adopted",
		}},
		"errors": []any{}, "future_private_field": "employee-private@example.test",
	}
	users := value["counts"].(map[string]any)["users"].(map[string]any)
	switch f.mode {
	case "rows":
		value["errors"] = []any{map[string]any{"row": 1, "reason": "employee-private@example.test invalid"}}
	case "unsafe-notices":
		value["warnings"] = []any{map[string]any{"row": 2000000, "type": "employee-private@example.test"}}
		value["errors"] = []any{map[string]any{"row": 0, "field": "employee-private@example.test", "reason": "private"}}
	case "missing-count":
		delete(users, "inactive")
	case "negative":
		users["created"] = -1
	case "fractional":
		users["created"] = 1.5
	case "warnings-null":
		value["warnings"] = nil
	case "no-adopted":
		delete(users, "adopted")
	case "http-400", "http-503":
		status, contentType = 400, "application/problem+json"
		if f.mode == "http-503" {
			status = 503
		}
		value = map[string]any{
			"status": status, "title": "Request failed", "type": "https://example.test/problem",
			"instance": "probe", "detail": "employee-private@example.test rejected",
		}
	case "replaced":
		require.NoError(f.t, os.Remove(f.path))
		require.NoError(f.t, os.WriteFile(f.path, []byte("preserve-replacement"), 0o600))
	}
	body, err := json.Marshal(value)
	require.NoError(f.t, err)
	if f.mode == "non-json" {
		body, contentType = []byte("employee-private@example.test"), "text/plain"
	}
	return &http.Response{
		StatusCode: status, Header: http.Header{"Content-Type": {contentType}},
		Body: io.NopCloser(bytes.NewReader(body)), Request: request,
	}, nil
}

func TestOrgChartImportMultipartIntentPrivateReportsAndSafeCounts(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		for _, mode := range []string{"", "rows", "no-adopted", "unsafe-notices"} {
			f := &orgImportFixture{t: t, dryRun: dryRun, mode: mode, path: filepath.Join(t.TempDir(), "report.json")}
			value, err := helpers.ImportAndCaptureIdentityOrgChart(
				t.Context(), f.api(), orgImportDirectory, []byte(orgImportCSV), dryRun, f.path,
			)
			if mode == "rows" || mode == "unsafe-notices" {
				require.ErrorContains(t, err, "row errors")
				require.Equal(t, "reported_errors", value.Status)
			} else {
				require.NoError(t, err)
				if dryRun {
					require.Equal(t, "validated", value.Status)
				} else {
					require.Equal(t, "completed", value.Status)
				}
			}
			require.Equal(t, 1, f.calls)
			require.Equal(t, dryRun, value.DryRun)
			require.Equal(t, 1, value.Rows)
			require.Equal(t, 1, value.WarningCount)
			if mode == "unsafe-notices" {
				require.Equal(t, "other", value.Warnings[0].Type)
				require.Nil(t, value.Warnings[0].Row)
				require.Equal(t, "other", value.Errors[0].Field)
				require.Nil(t, value.Errors[0].Row)
			}
			if mode == "no-adopted" {
				require.Nil(t, value.Counts.Users.Adopted)
			} else {
				require.EqualValues(t, 1, *value.Counts.Users.Adopted)
			}
			public, err := json.Marshal(value)
			require.NoError(t, err)
			require.NotContains(t, string(public), "employee-private")
			require.NotContains(t, string(public), "future_private_field")
			require.NotContains(t, f.logs.String(), "employee-private")
			data, err := os.ReadFile(f.path)
			require.NoError(t, err)
			require.Contains(t, string(data), "employee-private@example.test")
			require.Contains(t, string(data), "future_private_field")
			var captured struct {
				DirectoryID string `json:"directory_id"`
				DryRun      bool   `json:"dry_run"`
				Hash        string `json:"request_sha256"`
			}
			require.NoError(t, json.Unmarshal(data, &captured))
			digest := sha256.Sum256([]byte(orgImportCSV))
			require.Equal(t, hex.EncodeToString(digest[:]), captured.Hash)
			require.Equal(t, orgImportDirectory, captured.DirectoryID)
			require.Equal(t, dryRun, captured.DryRun)
		}
	}
}

func TestOrgChartImportUncertainFailuresAreSingleAttemptAndValueFree(t *testing.T) {
	for _, mode := range []string{
		"transport", "http-400", "http-503", "missing-count", "negative", "fractional",
		"warnings-null", "non-json", "replaced",
	} {
		t.Run(mode, func(t *testing.T) {
			f := &orgImportFixture{t: t, mode: mode, path: filepath.Join(t.TempDir(), "report.json")}
			value, err := helpers.ImportAndCaptureIdentityOrgChart(
				t.Context(), f.api(), orgImportDirectory, []byte(orgImportCSV), false, f.path,
			)
			require.Error(t, err)
			require.Nil(t, value)
			require.NotContains(t, err.Error(), "employee-private")
			require.NotContains(t, f.logs.String(), "employee-private")
			require.Equal(t, 1, f.calls, "SDK and transport retry layers must not replay even this RPC preview")
			data, err := os.ReadFile(f.path)
			require.NoError(t, err)
			if mode == "replaced" {
				require.Equal(t, "preserve-replacement", string(data))
			}
			if mode == "http-400" || mode == "http-503" || mode == "transport" || mode == "non-json" {
				var captured struct {
					Report       map[string]any `json:"report"`
					ResponseBody []byte         `json:"response_body_base64"`
				}
				require.NoError(t, json.Unmarshal(data, &captured))
				switch mode {
				case "http-400":
					require.EqualValues(t, 400, captured.Report["status"])
				case "http-503":
					require.EqualValues(t, 503, captured.Report["status"])
				case "transport":
					require.Contains(t, captured.Report["request_error"], "employee-private")
				case "non-json":
					require.Equal(t, []byte("employee-private@example.test"), captured.ResponseBody)
				}
			}
		})
	}
}

func TestOrgChartImportLocalPreflightRejectsInputWithoutUpload(t *testing.T) {
	for _, input := range []string{
		"", "header\n", "a,b\nwrong,field,count\n", "a\n\xff\n", "a\n\"unclosed\n",
		"a\n" + strings.Repeat("row\n", 50001),
	} {
		f := &orgImportFixture{t: t, path: filepath.Join(t.TempDir(), "report.json")}
		_, err := helpers.ImportAndCaptureIdentityOrgChart(
			t.Context(), f.api(), orgImportDirectory, []byte(input), true, f.path,
		)
		require.Error(t, err)
		require.Zero(t, f.calls)
		_, err = os.Stat(f.path)
		require.True(t, os.IsNotExist(err))
	}
	f := &orgImportFixture{t: t, path: filepath.Join(t.TempDir(), "report.json")}
	require.NoError(t, os.WriteFile(f.path, []byte("preserve-existing"), 0o600))
	f.path = filepath.Join(filepath.Dir(f.path), "report.1.json")
	summary, err := helpers.ImportAndCaptureIdentityOrgChart(
		t.Context(), f.api(), orgImportDirectory, []byte(orgImportCSV), false,
		filepath.Join(filepath.Dir(f.path), "report.json"),
	)
	require.NoError(t, err)
	require.Equal(t, f.path, summary.ReportOutputFile)
	require.Equal(t, 1, f.calls)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(f.path), "report.json"))
	require.NoError(t, err)
	require.Equal(t, "preserve-existing", string(data))
	var empty *helpers.KonnectSDK
	require.Nil(t, empty.GetIdentityOrgChartImportAPI())
	require.Nil(t, (&helpers.KonnectSDK{}).GetIdentityOrgChartImportAPI())
	_, err = helpers.ImportAndCaptureIdentityOrgChart(t.Context(), nil, orgImportDirectory, nil, true, f.path)
	require.ErrorContains(t, err, "not configured")
	_, err = helpers.ImportAndCaptureIdentityOrgChart(t.Context(), f.api(), "not-uuid", nil, true, f.path)
	require.ErrorContains(t, err, "UUID")
}
