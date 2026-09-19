//go:build e2e

package harness

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// HTTPPhaseDiagnostic contains only allowlisted trace metadata. Request IDs
// are scoped to the subprocess log, not unique across execution attempts.
type HTTPPhaseDiagnostic struct {
	RequestID     string `json:"request_id"`
	LastPhase     string `json:"last_observed_phase"`
	ElapsedMS     int64  `json:"last_observed_elapsed_ms"`
	HTTPTimeoutMS int64  `json:"http_timeout_ms"`
	Outcome       string `json:"outcome"`
}

// ReadHTTPPhases reads the CLI's slog text file. A missing or truncated trace
// is evidence of an unknown outcome, never proof of a server-side timeout.
func ReadHTTPPhases(path string) ([]HTTPPhaseDiagnostic, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var requests []HTTPPhaseDiagnostic
	byID := map[string]int{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "log_type=http_phase") {
			continue
		}
		fields := slogTextFields(line)
		if fields["log_type"] != "http_phase" {
			continue
		}
		id := fields["request_id"]
		if !strings.HasPrefix(id, "khttp-") {
			continue
		}
		if _, err := strconv.ParseUint(strings.TrimPrefix(id, "khttp-"), 10, 64); err != nil {
			continue
		}
		phase := fields["phase"]
		switch phase {
		case "request_start", "connection_acquire_start", "connection_acquired", "dns_start", "dns_done",
			"connect_start", "connect_done", "tls_start", "tls_done", "request_headers_written",
			"request_written", "first_response_byte", "request_done":
		default:
			continue
		}
		i, ok := byID[id]
		if !ok {
			i = len(requests)
			byID[id] = i
			requests = append(requests, HTTPPhaseDiagnostic{RequestID: id, LastPhase: "unknown", Outcome: "unfinished"})
		}
		r := &requests[i]
		// Late DNS/connect callbacks can arrive after the request has failed.
		if r.Outcome != "unfinished" {
			continue
		}
		if phase == "request_done" {
			switch fields["outcome"] {
			case "failed", "response_headers":
				r.Outcome = fields["outcome"]
			}
		} else {
			r.LastPhase = phase
			r.ElapsedMS, _ = strconv.ParseInt(fields["elapsed_ms"], 10, 64)
		}
		r.HTTPTimeoutMS, _ = strconv.ParseInt(fields["http_timeout_ms"], 10, 64)
	}
	return requests, scanner.Err()
}

// Parse complete slog text tokens rather than matching substrings inside a
// quoted message/body. Only ReadHTTPPhases' allowlisted fields are exported.
func slogTextFields(line string) map[string]string {
	fields := map[string]string{}
	for line != "" {
		line = strings.TrimLeft(line, " \t")
		key, rest, ok := strings.Cut(line, "=")
		if !ok || strings.ContainsAny(key, " \t\"") {
			break
		}
		var value string
		if strings.HasPrefix(rest, "\"") {
			quoted, err := strconv.QuotedPrefix(rest)
			if err != nil {
				break
			}
			value, err = strconv.Unquote(quoted)
			if err != nil {
				break
			}
			line = rest[len(quoted):]
		} else {
			value, line, _ = strings.Cut(rest, " ")
		}
		fields[key] = value
	}
	return fields
}
