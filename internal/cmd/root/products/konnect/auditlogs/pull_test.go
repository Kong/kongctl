package auditlogs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/iostreams"
	"github.com/kong/kongctl/internal/konnect/apiutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestAuditLogPullClientEncodesFiltersAndCursor(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.August, 23, 1, 2, 3, 0, time.UTC)
	end := start.Add(time.Hour)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, auditLogsPullPath, r.URL.Path)
		require.Equal(t, "Bearer secret", r.Header.Get("Authorization"))
		require.Equal(t, "1000", r.URL.Query().Get("page[size]"))
		require.Equal(t, "cursor-value", r.URL.Query().Get("page[after]"))
		require.Equal(t, "authorization", r.URL.Query().Get("filter[type]"))
		require.Equal(t, start.Format(time.RFC3339Nano), r.URL.Query().Get("filter[ts][gte]"))
		require.Equal(t, end.Format(time.RFC3339Nano), r.URL.Query().Get("filter[ts][lte]"))
		_, _ = w.Write([]byte(`{"data":[{"signature":"sig"}],` +
			`"meta":{"page":{"next":"/audit-logs?page%5Bafter%5D=next-token"}}}`))
	}))
	defer server.Close()

	client := auditLogPullClient{
		baseURL:     server.URL,
		tokenSource: apiutil.NewStaticTokenSource("secret"),
		httpClient:  server.Client(),
	}
	page, err := client.fetchPage(t.Context(), auditLogRequest{
		Window:   auditLogWindow{Start: &start, End: &end},
		Type:     "authorization",
		PageSize: 1000,
		After:    "cursor-value",
	})
	require.NoError(t, err)
	require.Len(t, page.Data, 1)
	require.Equal(t, "next-token", page.NextCursor)
}

func TestFetchAllAuditLogPagesExhaustsCursorChain(t *testing.T) {
	t.Parallel()

	requested := make([]string, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("page[after]")
		requested = append(requested, cursor)
		switch cursor {
		case "":
			_, _ = w.Write([]byte(`{"data":[{"id":1}],"meta":{"page":{"next":"one"}}}`))
		case "one":
			_, _ = w.Write([]byte(`{"data":[{"id":2}],"meta":{"next":"two"}}`))
		case "two":
			_, _ = w.Write([]byte(`{"data":[{"id":3}],"meta":{"page":{}}}`))
		default:
			http.Error(w, "unexpected cursor", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	client := testAuditLogClient(server)
	var records []json.RawMessage
	count, truncated, err := fetchAllAuditLogPages(t.Context(), &client, auditLogRequest{PageSize: 10}, 0,
		func(page []json.RawMessage) error {
			records = append(records, page...)
			return nil
		})
	require.NoError(t, err)
	require.Equal(t, 3, count)
	require.False(t, truncated)
	require.Len(t, records, 3)
	require.Equal(t, []string{"", "one", "two"}, requested)
}

func TestFetchAllAuditLogPagesHonorsTotalLimit(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page[after]") == "" {
			_, _ = w.Write([]byte(`{"data":[{"id":1},{"id":2}],"meta":{"page":{"next":"two"}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":3},{"id":4}],"meta":{"page":{"next":"three"}}}`))
	}))
	defer server.Close()

	client := testAuditLogClient(server)
	var records []json.RawMessage
	count, truncated, err := fetchAllAuditLogPages(t.Context(), &client, auditLogRequest{PageSize: 2}, 3,
		func(page []json.RawMessage) error {
			records = append(records, page...)
			return nil
		})
	require.NoError(t, err)
	require.Equal(t, 3, count)
	require.True(t, truncated)
	require.Len(t, records, 3)
}

func TestFetchAllAuditLogPagesRejectsRepeatedCursor(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[],"meta":{"page":{"next":"repeat"}}}`))
	}))
	defer server.Close()

	client := testAuditLogClient(server)
	_, _, err := fetchAllAuditLogPages(t.Context(), &client, auditLogRequest{PageSize: 10}, 0,
		func([]json.RawMessage) error { return nil })
	require.ErrorContains(t, err, "repeated cursor")
}

func TestDecodeAuditLogPageRejectsMissingPaginationMetadata(t *testing.T) {
	t.Parallel()

	_, err := decodeAuditLogPage([]byte(`{"data":[]}`))
	require.ErrorContains(t, err, "missing pagination metadata")
}

func TestResolveAuditLogWindow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC)
	window, err := resolveAuditLogWindow(pullAuditLogsOptions{Since: 24 * time.Hour}, now)
	require.NoError(t, err)
	require.Equal(t, now.Add(-24*time.Hour), *window.Start)
	require.Equal(t, now, *window.End)

	window, err = resolveAuditLogWindow(pullAuditLogsOptions{Follow: true}, now)
	require.NoError(t, err)
	require.Equal(t, now.Add(-defaultFollowLookback), *window.Start)
	require.Equal(t, now, *window.End)

	_, err = resolveAuditLogWindow(pullAuditLogsOptions{
		Follow:    true,
		StartTime: now.Add(time.Hour).Format(time.RFC3339),
	}, now)
	require.ErrorContains(t, err, "--start-time must not be later")
}

func TestResolveAuditLogLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		options    pullAuditLogsOptions
		limitValue string
		want       int
	}{
		{name: "bare command", want: defaultAuditLogLimit},
		{name: "type filter only", options: pullAuditLogsOptions{EventType: "authorization"}, want: defaultAuditLogLimit},
		{name: "explicit limit", limitValue: "100", want: 100},
		{name: "explicit unlimited", limitValue: "0", want: 0},
		{name: "relative time window", options: pullAuditLogsOptions{Since: 24 * time.Hour}, want: 0},
		{name: "start time", options: pullAuditLogsOptions{StartTime: "2026-08-23T00:00:00Z"}, want: 0},
		{name: "end time", options: pullAuditLogsOptions{EndTime: "2026-08-24T00:00:00Z"}, want: 0},
		{name: "follow", options: pullAuditLogsOptions{Follow: true}, want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := test.options
			command := &cobra.Command{}
			addPullAuditLogsFlags(command, &options)
			if test.limitValue != "" {
				require.NoError(t, command.Flags().Set("limit", test.limitValue))
			}

			require.Equal(t, test.want, resolveAuditLogLimit(command, options))
		})
	}
}

func TestValidatePullAuditLogsOptions(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{}
	tests := []struct {
		name    string
		options pullAuditLogsOptions
		output  string
		page    int
		want    string
	}{
		{
			name: "page too large", options: pullAuditLogsOptions{PollInterval: time.Second}, output: "json", page: 1001,
			want: "between 1 and 1000",
		},
		{name: "relative and absolute", options: pullAuditLogsOptions{
			Since: time.Hour, StartTime: "x",
			PollInterval: time.Second,
		}, output: "json", page: 10, want: "cannot be combined"},
		{
			name: "invalid type", options: pullAuditLogsOptions{EventType: "access", PollInterval: time.Second},
			output: "json", page: 10, want: "invalid --type",
		},
		{
			name: "bounded follow", options: pullAuditLogsOptions{Follow: true, EndTime: "x", PollInterval: time.Second},
			output: "text", page: 10, want: "cannot be used with --follow",
		},
		{
			name: "document follow", options: pullAuditLogsOptions{Follow: true, PollInterval: time.Second},
			output: "json", page: 10, want: "row-oriented",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validatePullAuditLogsOptions(command, test.options, test.output, test.page)
			require.ErrorContains(t, err, test.want)
		})
	}
}

func TestValidateAuditLogOutputOptions(t *testing.T) {
	t.Parallel()

	t.Run("jsonl permits per-record jq", func(t *testing.T) {
		t.Parallel()
		command := &cobra.Command{}
		addPullAuditLogsFlags(command, &pullAuditLogsOptions{})
		require.NoError(t, command.Flags().Set("jq", ".signature"))
		require.NoError(t, validateAuditLogOutputOptions(command, newPullTestConfig(t), jsonLinesOutput))
	})

	t.Run("jsonl raw output requires jq", func(t *testing.T) {
		t.Parallel()
		command := &cobra.Command{}
		addPullAuditLogsFlags(command, &pullAuditLogsOptions{})
		require.NoError(t, command.Flags().Set("jq-raw-output", "true"))
		cfg := newPullTestConfig(t)
		cfg.Set("jq.raw-output", true)
		err := validateAuditLogOutputOptions(command, cfg, jsonLinesOutput)
		require.ErrorContains(t, err, "requires --jq")
	})

	t.Run("text rejects jq", func(t *testing.T) {
		t.Parallel()
		command := &cobra.Command{}
		addPullAuditLogsFlags(command, &pullAuditLogsOptions{})
		require.NoError(t, command.Flags().Set("jq", ".signature"))
		err := validateAuditLogOutputOptions(command, newPullTestConfig(t), "text")
		require.ErrorContains(t, err, "only supported with --output json or --output yaml")
	})
}

func TestRenderAuditLogJSONLinesPreservesSignatureAndFiltersPerRecord(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{Use: "audit-logs"}
	addPullAuditLogsFlags(command, &pullAuditLogsOptions{})
	require.NoError(t, command.Flags().Set("jq", ".signature"))
	cfg := newPullTestConfig(t)
	streams := &iostreams.IOStreams{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}
	helper := cmd.NewMockHelper(t)
	helper.EXPECT().GetConfig().Return(cfg, nil)
	helper.EXPECT().GetCmd().Return(command)
	helper.EXPECT().GetStreams().Return(streams)

	err := renderAuditLogJSONLines(helper, []json.RawMessage{
		json.RawMessage(`{"signature":"sig-one","type":"authorization"}`),
		json.RawMessage(`{"signature":"sig-two","type":"authentication"}`),
	})
	require.NoError(t, err)
	require.Equal(t, "\"sig-one\"\n\"sig-two\"\n", streams.Out.(*bytes.Buffer).String())
}

func TestRunFiniteAuditLogPullJSONLinesLeavesPartialOutputOnLaterFailure(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page[after]") == "" {
			_, _ = w.Write([]byte(`{"data":[{"signature":"first"}],"meta":{"page":{"next":"second"}}}`))
			return
		}
		http.Error(w, "temporary failure", http.StatusInternalServerError)
	}))
	defer server.Close()

	command := &cobra.Command{Use: "audit-logs"}
	addPullAuditLogsFlags(command, &pullAuditLogsOptions{})
	cfg := newPullTestConfig(t)
	streams := &iostreams.IOStreams{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}
	helper := cmd.NewMockHelper(t)
	helper.EXPECT().GetContext().Return(context.Background())
	helper.EXPECT().GetConfig().Return(cfg, nil)
	helper.EXPECT().GetCmd().Return(command).Times(2)
	helper.EXPECT().GetStreams().Return(streams)
	client := testAuditLogClient(server)

	err := runFiniteAuditLogPull(helper, &client, pullAuditLogsOptions{}, jsonLinesOutput, 10, auditLogWindow{})
	require.Error(t, err)
	require.Contains(t, streams.Out.(*bytes.Buffer).String(), `"signature":"first"`)
}

func TestRenderFiniteAuditLogsStructuredOutputPreservesCompleteRecords(t *testing.T) {
	t.Parallel()

	for _, output := range []string{"json", "yaml"} {
		t.Run(output, func(t *testing.T) {
			t.Parallel()
			command := &cobra.Command{Use: "audit-logs"}
			addPullAuditLogsFlags(command, &pullAuditLogsOptions{})
			cfg := newPullTestConfig(t)
			cfg.Set("output", output)
			streams := &iostreams.IOStreams{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}
			helper := cmd.NewMockHelper(t)
			helper.EXPECT().GetStreams().Return(streams)
			helper.EXPECT().GetConfig().Return(cfg, nil)
			helper.EXPECT().GetCmd().Return(command)

			err := renderFiniteAuditLogs(helper, output, auditLogEnvelope{
				Metadata: auditLogOutputMetadata{Count: 1, Truncated: true},
				Data: []json.RawMessage{
					json.RawMessage(`{"type":"authentication","signature":"unchanged","future_field":{"a":1}}`),
				},
			})
			require.NoError(t, err)
			rendered := streams.Out.(*bytes.Buffer).String()
			require.Contains(t, rendered, "unchanged")
			require.Contains(t, rendered, "future_field")
			require.Contains(t, rendered, "truncated")
		})
	}
}

func TestDefaultAuditLogRowsCoverEveryEventType(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{"2026-08-24T10:00:00Z", "authentication", "user-1", "sso", "success", "trace-1"},
		defaultAuditLogRow(json.RawMessage(
			`{"type":"authentication","principal_id":"user-1","authentication_type":"sso","outcome":"success",`+
				`"ts":"2026-08-24T10:00:00Z","trace_id":"trace-1"}`,
		)))
	require.Equal(t, []string{"2026-08-24T10:00:01Z", "authorization", "user-2", "gateway.read", "denied", "trace-2"},
		defaultAuditLogRow(json.RawMessage(
			`{"type":"authorization","principal_id":"user-2","action":"gateway.read","granted":false,`+
				`"ts":"2026-08-24T10:00:01Z","trace_id":"trace-2"}`,
		)))
	require.Equal(t, []string{"2026-08-24T10:00:02Z", "gateway_access", "user-3", "POST /v2/services", "201", "trace-3"},
		defaultAuditLogRow(json.RawMessage(
			`{"type":"gateway_access","principal_id":"user-3","method":"POST","uri":"/v2/services",`+
				`"status_code":201,"ts":"2026-08-24T10:00:02Z","trace_id":"trace-3"}`,
		)))
}

func TestDeduplicateAuditLogRecordsUsesSignature(t *testing.T) {
	t.Parallel()

	records := []json.RawMessage{
		json.RawMessage(`{"signature":"same","ts":"2026-08-24T10:00:00Z","value":1}`),
		json.RawMessage(`{"signature":"same","ts":"2026-08-24T10:00:01Z","value":2}`),
		json.RawMessage(`{"ts":"2026-08-24T10:00:02Z","value":3,"nested":{"a":1}}`),
		json.RawMessage(`{ "nested": { "a": 1 }, "value": 3, "ts": "2026-08-24T10:00:02Z" }`),
	}
	unique := deduplicateAuditLogRecords(records, map[string]time.Time{}, time.Time{})
	require.Len(t, unique, 2)
}

func TestRunAuditLogFollowDeduplicatesOverlappingCyclesAndCancelsCleanly(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	requestCount := 0
	firstTimestamp := time.Now().UTC().Add(-time.Second).Format(time.RFC3339Nano)
	secondTimestamp := time.Now().UTC().Format(time.RFC3339Nano)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		switch requestCount {
		case 1:
			_, _ = fmt.Fprintf(w, `{"data":[{"signature":"one","ts":%q}],"meta":{"page":{}}}`,
				firstTimestamp)
		case 2:
			_, _ = fmt.Fprintf(w, `{"data":[{"signature":"two","ts":%q},{"signature":"one","ts":%q}],`+
				`"meta":{"page":{}}}`, secondTimestamp, firstTimestamp)
		default:
			cancel()
		}
	}))
	defer server.Close()

	command := &cobra.Command{Use: "audit-logs"}
	addPullAuditLogsFlags(command, &pullAuditLogsOptions{})
	cfg := newPullTestConfig(t)
	streams := &iostreams.IOStreams{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}
	helper := cmd.NewMockHelper(t)
	helper.EXPECT().GetContext().Return(ctx).Maybe()
	helper.EXPECT().GetConfig().Return(cfg, nil).Maybe()
	helper.EXPECT().GetCmd().Return(command).Maybe()
	helper.EXPECT().GetStreams().Return(streams).Maybe()
	client := testAuditLogClient(server)
	start := time.Now().Add(-time.Minute)
	end := time.Now()

	err := runAuditLogFollow(helper, &client, pullAuditLogsOptions{
		Follow: true, PollInterval: time.Millisecond,
	}, jsonLinesOutput, 10, auditLogWindow{Start: &start, End: &end})
	require.NoError(t, err)
	output := streams.Out.(*bytes.Buffer).String()
	require.Equal(t, 1, strings.Count(output, `"signature":"one"`))
	require.Equal(t, 1, strings.Count(output, `"signature":"two"`))
}

func TestRenderFollowTextWritesHeaderOnce(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{Use: "audit-logs"}
	addPullAuditLogsFlags(command, &pullAuditLogsOptions{})
	streams := &iostreams.IOStreams{Out: &bytes.Buffer{}, ErrOut: &bytes.Buffer{}}
	helper := cmd.NewMockHelper(t)
	helper.EXPECT().GetCmd().Return(command).Times(2)
	helper.EXPECT().GetStreams().Return(streams).Times(3)
	headerWritten := false
	record := []json.RawMessage{json.RawMessage(
		`{"type":"authentication","ts":"2026-08-24T10:00:00Z","signature":"one"}`,
	)}
	require.NoError(t, renderFollowText(helper, record, &headerWritten))
	require.NoError(t, renderFollowText(helper, record, &headerWritten))
	require.Equal(t, 1, strings.Count(streams.Out.(*bytes.Buffer).String(), "TIMESTAMP"))
}

func TestAuditLogRetryClassification(t *testing.T) {
	t.Parallel()

	require.True(t, isRetryableAuditLogError(context.DeadlineExceeded))
	require.True(t, isRetryableAuditLogError(&auditLogResponseError{status: http.StatusTooManyRequests}))
	require.True(t, isRetryableAuditLogError(&auditLogResponseError{status: http.StatusBadGateway}))
	require.False(t, isRetryableAuditLogError(&auditLogResponseError{status: http.StatusForbidden}))
}

func testAuditLogClient(server *httptest.Server) auditLogPullClient {
	return auditLogPullClient{
		baseURL:     server.URL,
		tokenSource: apiutil.NewStaticTokenSource("token"),
		httpClient:  server.Client(),
	}
}

func newPullTestConfig(t *testing.T) config.Hook {
	t.Helper()
	cfg := newTestConfigHook(t)
	cfg.Set("jq.default-expression", "")
	return cfg
}

func TestNormalizeAuditLogCursor(t *testing.T) {
	t.Parallel()

	cursor, err := normalizeAuditLogCursor("/audit-logs?page%5Bafter%5D=token")
	require.NoError(t, err)
	require.Equal(t, "token", cursor)
	cursor, err = normalizeAuditLogCursor("token")
	require.NoError(t, err)
	require.Equal(t, "token", cursor)
	cursor, err = normalizeAuditLogCursor("")
	require.NoError(t, err)
	require.Empty(t, cursor)
	_, err = normalizeAuditLogCursor("/audit-logs?page%5Bbefore%5D=token")
	require.ErrorContains(t, err, "does not contain page[after]")
}

func Example_auditLogQueryEncoding() {
	values := url.Values{}
	values.Set("filter[ts][gte]", "2026-08-23T00:00:00Z")
	fmt.Println(values.Encode())
	// Output: filter%5Bts%5D%5Bgte%5D=2026-08-23T00%3A00%3A00Z
}
