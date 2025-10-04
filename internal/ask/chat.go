package ask

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/kong/kongctl/internal/meta"
)

// ChatEvent represents a single server-sent event emitted by the Doctor Who agent API.
type ChatEvent struct {
	Event string `json:"event" yaml:"event"`
	Data  any    `json:"data,omitempty" yaml:"data,omitempty"`
}

// Result captures the aggregated chat response and all underlying events.
type Result struct {
	Prompt   string      `json:"prompt" yaml:"prompt"`
	Response string      `json:"response" yaml:"response"`
	Events   []ChatEvent `json:"events" yaml:"events"`
}

const (
	defaultScannerCapacity = 1024 * 1024 // 1 MiB buffer for large SSE payloads
	chatPathSegment        = "v1/chat"
)

// Chat executes a stateless prompt against the Doctor Who agent.
func Chat(ctx context.Context, client *http.Client, baseURL, token, prompt string) (*Result, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	if client == nil {
		client = http.DefaultClient
	}

	endpoint, err := url.JoinPath(baseURL, chatPathSegment)
	if err != nil {
		return nil, fmt.Errorf("failed to construct chat endpoint: %w", err)
	}

	body, err := json.Marshal(map[string]string{"prompt": prompt})
	if err != nil {
		return nil, fmt.Errorf("failed to encode chat payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to build chat request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("User-Agent", meta.CLIName)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute chat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	events, text, err := parseSSE(resp.Body)
	if err != nil {
		return nil, err
	}

	for _, evt := range events {
		if evt.Event == "error" {
			return nil, fmt.Errorf("doctor who agent error: %s", extractErrorMessage(evt.Data))
		}
	}

	return &Result{
		Prompt:   prompt,
		Response: text,
		Events:   events,
	}, nil
}

func parseSSE(r io.Reader) ([]ChatEvent, string, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), defaultScannerCapacity)

	var (
		currentEvent string
		dataLines    []string
		events       []ChatEvent
		response     strings.Builder
		streamEnded  bool
	)

	flushEvent := func() {
		if currentEvent == "" && len(dataLines) == 0 {
			return
		}
		raw := strings.Join(dataLines, "\n")
		var parsed any
		if raw != "" {
			if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
				parsed = raw
			}
		}

		evt := ChatEvent{
			Event: currentEvent,
			Data:  parsed,
		}
		if evt.Event == "" {
			evt.Event = "message"
		}

		events = append(events, evt)

		if evt.Event == "llm-response" {
			if text, ok := extractResponseText(evt.Data); ok {
				response.WriteString(text)
			}
		}

		if evt.Event == "end" {
			streamEnded = true
		}

		currentEvent = ""
		dataLines = dataLines[:0]
	}

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			flushEvent()
			if streamEnded {
				return events, response.String(), nil
			}
		case strings.HasPrefix(line, ":"):
			// Comment/keepalive line, ignore.
			continue
		case strings.HasPrefix(line, "event:"):
			currentEvent = strings.TrimSpace(line[len("event:"):])
		case strings.HasPrefix(line, "data:"):
			data := strings.TrimPrefix(line[len("data:"):], " ")
			dataLines = append(dataLines, data)
		default:
			// Unexpected line, treat as data continuation.
			dataLines = append(dataLines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, "", fmt.Errorf("failed to read chat response: %w", err)
	}

	flushEvent()

	return events, response.String(), nil
}

func extractResponseText(data any) (string, bool) {
	switch v := data.(type) {
	case map[string]any:
		if inner, ok := v["data"]; ok {
			if str, ok := inner.(string); ok {
				return str, true
			}
		}
	case string:
		return v, true
	case nil:
		return "", false
	}
	return "", false
}

func extractErrorMessage(data any) string {
	switch v := data.(type) {
	case map[string]any:
		if msg, ok := v["error"]; ok {
			return fmt.Sprint(msg)
		}
		if msg, ok := v["message"]; ok {
			return fmt.Sprint(msg)
		}
	case string:
		return v
	case nil:
		return "unknown error"
	}
	return fmt.Sprint(data)
}
