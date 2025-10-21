package kai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kong/kongctl/internal/meta"
)

// ChatEvent represents a single server-sent event emitted by the Kai agent API.
type ChatEvent struct {
	Event string `json:"event"          yaml:"event"`
	Data  any    `json:"data,omitempty" yaml:"data,omitempty"`
}

// Result captures the aggregated chat response and all underlying events.
type Result struct {
	Prompt   string      `json:"prompt"   yaml:"prompt"`
	Response string      `json:"response" yaml:"response"`
	Events   []ChatEvent `json:"events"   yaml:"events"`
}

// ContextType enumerates the supported Kai context entity types.
type ContextType string

const (
	ContextTypeControlPlane         ContextType = "control_plane"
	ContextTypeDataPlane            ContextType = "data_plane"
	ContextTypeRoute                ContextType = "route"
	ContextTypePlugin               ContextType = "plugin"
	ContextTypeService              ContextType = "service"
	ContextTypeConsumer             ContextType = "consumer"
	ContextTypeActiveTracingSession ContextType = "active_tracing_session"
)

// ChatContext represents an entity reference shared with the Kai agent.
type ChatContext struct {
	Type ContextType `json:"type"`
	ID   string      `json:"id"`
}

const (
	defaultScannerCapacity = 1024 * 1024 // 1 MiB buffer for large SSE payloads
	chatPathSegment        = "v1/chat"
	sessionsPathSegment    = "v1/sessions"
)

type SessionMetadata struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateSessionPayload struct {
	Name string `json:"name"`
}

type SessionList []SessionMetadata

type SessionHistory struct {
	ID        string               `json:"id"`
	Name      string               `json:"name"`
	CreatedAt time.Time            `json:"created_at"`
	History   []SessionHistoryItem `json:"history"`
}

type SessionHistoryItem struct {
	ID        string                 `json:"id"`
	TriggerID string                 `json:"trigger_id"`
	Role      string                 `json:"role"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	Context   []map[string]any       `json:"context"`
	Tool      *SessionHistoryToolRef `json:"tool_details"`
}

type SessionHistoryToolRef struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// SessionLimitError indicates the server refused to create a session because the maximum was reached.
type SessionLimitError struct {
	Detail string
	Raw    string
}

func (e *SessionLimitError) Error() string {
	if e == nil {
		return "session limit reached"
	}
	detail := strings.TrimSpace(e.Detail)
	if detail != "" {
		return detail
	}
	return "maximum allowed sessions reached"
}

// Stream represents an active chat stream returning events incrementally.
type Stream struct {
	Events <-chan ChatEvent
	err    <-chan error
}

// Err blocks until the underlying stream completes and returns the terminal error, if any.
func (s *Stream) Err() error {
	if s == nil {
		return nil
	}
	return <-s.err
}

// Chat executes a stateless prompt against the Kai agent.
func Chat(ctx context.Context, client *http.Client, baseURL, token, prompt string) (*Result, error) {
	stream, err := ChatStream(ctx, client, baseURL, token, prompt)
	if err != nil {
		return nil, err
	}

	var (
		events   []ChatEvent
		response strings.Builder
		errEvent error
	)

	logDebug(ctx, "kai chat stream opened",
		slog.Int("prompt_length", len(prompt)))

	for evt := range stream.Events {
		events = append(events, evt)
		if errEvent != nil {
			continue
		}

		switch evt.Event {
		case "llm-response":
			if text, ok := extractResponseText(evt.Data); ok {
				response.WriteString(text)
			}
		case "error":
			errEvent = fmt.Errorf("kai agent error: %s", extractErrorMessage(evt.Data))
		}
	}

	if streamErr := stream.Err(); streamErr != nil && errEvent == nil {
		errEvent = streamErr
	}

	if errEvent != nil {
		logError(ctx, "kai chat stream failed",
			slog.String("error", errEvent.Error()))
		return nil, errEvent
	}

	logInfo(ctx, "kai chat stream completed",
		slog.Int("event_count", len(events)),
		slog.Int("response_length", response.Len()))

	return &Result{
		Prompt:   prompt,
		Response: response.String(),
		Events:   events,
	}, nil
}

// ChatStream executes the chat request and streams events to the caller.
func ChatStream(ctx context.Context, client *http.Client, baseURL, token, prompt string) (*Stream, error) {
	return chatStream(ctx, client, baseURL, token, prompt, "", nil)
}

// ChatStreamSession streams chat tied to an existing session.
func ChatStreamSession(
	ctx context.Context,
	client *http.Client,
	baseURL, token, sessionID, prompt string,
	contextEntities []ChatContext,
) (*Stream, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, errors.New("sessionID cannot be empty")
	}
	return chatStream(ctx, client, baseURL, token, prompt, sessionID, contextEntities)
}

// JoinSessionPath constructs /sessions/{id} paths safely.
func JoinSessionPath(baseURL, sessionID string, segments ...string) (string, error) {
	parts := append([]string{sessionsPathSegment, sessionID}, segments...)
	return url.JoinPath(baseURL, parts...)
}

func chatStream(
	ctx context.Context,
	client *http.Client,
	baseURL, token, prompt, sessionID string,
	contextEntities []ChatContext,
) (*Stream, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("prompt cannot be empty")
	}

	if client == nil {
		client = http.DefaultClient
	}

	endpoint, err := url.JoinPath(baseURL, chatPathSegment)
	if sessionID != "" {
		endpoint, err = url.JoinPath(baseURL, sessionsPathSegment, sessionID, "chat")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to construct chat endpoint: %w", err)
	}

	body, err := json.Marshal(struct {
		Prompt  string        `json:"prompt"`
		Context []ChatContext `json:"context,omitempty"`
	}{
		Prompt:  prompt,
		Context: contextEntities,
	})
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

	logDebug(ctx, "kai chat request",
		slog.String("endpoint", endpoint),
		slog.Int("payload_bytes", len(body)),
		slog.String("session_id", sessionID))

	resp, err := client.Do(req)
	if err != nil {
		err = wrapIfTransient(err)
		logError(ctx, "kai chat request failed",
			slog.String("endpoint", endpoint),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to execute chat request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		logError(ctx, "kai chat unexpected status",
			slog.String("endpoint", endpoint),
			slog.Int("status", resp.StatusCode),
			slog.String("snippet", truncateSnippet(strings.TrimSpace(string(snippet)), 512)))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	logDebug(ctx, "kai chat stream established",
		slog.String("endpoint", endpoint),
		slog.Int("status", resp.StatusCode))

	events := make(chan ChatEvent)
	errCh := make(chan error, 1)

	go func() {
		defer resp.Body.Close()
		defer close(events)
		defer close(errCh)

		err := decodeSSE(ctx, resp.Body, func(evt ChatEvent) error {
			logTrace(ctx, "kai chat event",
				slog.String("event", evt.Event))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case events <- evt:
				return nil
			}
		})
		if err != nil {
			err = wrapIfTransient(err)
			if !errors.Is(err, context.Canceled) {
				logError(ctx, "kai chat stream error",
					slog.String("endpoint", endpoint),
					slog.String("error", err.Error()))
			}
			errCh <- err
			return
		}
		errCh <- nil
	}()

	return &Stream{
		Events: events,
		err:    errCh,
	}, nil
}

func decodeSSE(ctx context.Context, r io.Reader, onEvent func(ChatEvent) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), defaultScannerCapacity)

	var (
		currentEvent string
		dataLines    []string
	)

	flushEvent := func() (bool, error) {
		if currentEvent == "" && len(dataLines) == 0 {
			return false, nil
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

		if err := onEvent(evt); err != nil {
			return true, err
		}

		currentEvent = ""
		dataLines = dataLines[:0]

		return evt.Event == "end", nil
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		switch {
		case line == "":
			stop, err := flushEvent()
			if err != nil {
				return err
			}
			if stop {
				return nil
			}
		case strings.HasPrefix(line, ":"):
			continue
		case strings.HasPrefix(line, "event:"):
			currentEvent = strings.TrimSpace(line[len("event:"):])
		case strings.HasPrefix(line, "data:"):
			data := strings.TrimPrefix(line[len("data:"):], " ")
			dataLines = append(dataLines, data)
		default:
			dataLines = append(dataLines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read chat response: %w", err)
	}

	_, err := flushEvent()
	return err
}

// CreateSession creates a new chat session and returns metadata.
func CreateSession(ctx context.Context, client *http.Client, baseURL, token, name string) (*SessionMetadata, error) {
	if client == nil {
		client = http.DefaultClient
	}

	endpoint, err := url.JoinPath(baseURL, sessionsPathSegment)
	if err != nil {
		return nil, fmt.Errorf("failed to construct sessions endpoint: %w", err)
	}

	body, err := json.Marshal(CreateSessionPayload{Name: name})
	if err != nil {
		return nil, fmt.Errorf("failed to encode session payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to build session request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", meta.CLIName)

	logDebug(ctx, "kai create session request",
		slog.String("endpoint", endpoint),
		slog.Int("payload_bytes", len(body)))

	resp, err := client.Do(req)
	if err != nil {
		logError(ctx, "kai create session request failed",
			slog.String("endpoint", endpoint),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to execute session request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		logError(ctx, "kai create session unexpected status",
			slog.String("endpoint", endpoint),
			slog.Int("status", resp.StatusCode),
			slog.String("snippet", truncateSnippet(strings.TrimSpace(string(snippet)), 512)))
		if resp.StatusCode == http.StatusConflict {
			detail := strings.TrimSpace(string(snippet))
			var payload struct {
				Detail string `json:"detail"`
				Title  string `json:"title"`
			}
			if err := json.Unmarshal(snippet, &payload); err == nil {
				if d := strings.TrimSpace(payload.Detail); d != "" {
					detail = d
				} else if t := strings.TrimSpace(payload.Title); t != "" {
					detail = t
				}
			}
			return nil, &SessionLimitError{
				Detail: detail,
				Raw:    strings.TrimSpace(string(snippet)),
			}
		}
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	var meta SessionMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("failed to decode session metadata: %w", err)
	}

	logInfo(ctx, "kai session created",
		slog.String("session_id", meta.ID),
		slog.String("name", meta.Name))

	return &meta, nil
}

// ListSessions retrieves available sessions for the current user.
func ListSessions(ctx context.Context, client *http.Client, baseURL, token string) (SessionList, error) {
	if client == nil {
		client = http.DefaultClient
	}

	endpoint, err := url.JoinPath(baseURL, sessionsPathSegment)
	if err != nil {
		return nil, fmt.Errorf("failed to construct sessions endpoint: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build sessions request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("User-Agent", meta.CLIName)

	logDebug(ctx, "kai list sessions request",
		slog.String("endpoint", endpoint))

	resp, err := client.Do(req)
	if err != nil {
		logError(ctx, "kai list sessions request failed",
			slog.String("endpoint", endpoint),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to execute sessions request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		logError(ctx, "kai list sessions unexpected status",
			slog.String("endpoint", endpoint),
			slog.Int("status", resp.StatusCode),
			slog.String("snippet", truncateSnippet(strings.TrimSpace(string(snippet)), 512)))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	var list SessionList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("failed to decode sessions: %w", err)
	}

	logInfo(ctx, "kai sessions listed",
		slog.Int("count", len(list)))

	return list, nil
}

// GetSessionHistory retrieves full chat history for a session.
func GetSessionHistory(
	ctx context.Context,
	client *http.Client,
	baseURL, token, sessionID string,
) (*SessionHistory, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, errors.New("sessionID cannot be empty")
	}

	if client == nil {
		client = http.DefaultClient
	}

	endpoint, err := url.JoinPath(baseURL, sessionsPathSegment, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to construct session history endpoint: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build session history request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("User-Agent", meta.CLIName)

	logDebug(ctx, "kai get session history request",
		slog.String("endpoint", endpoint),
		slog.String("session_id", sessionID))

	resp, err := client.Do(req)
	if err != nil {
		err = wrapIfTransient(err)
		logError(ctx, "kai get session history request failed",
			slog.String("endpoint", endpoint),
			slog.String("session_id", sessionID),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to execute session history request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		logError(ctx, "kai get session history unexpected status",
			slog.String("endpoint", endpoint),
			slog.String("session_id", sessionID),
			slog.Int("status", resp.StatusCode),
			slog.String("snippet", truncateSnippet(strings.TrimSpace(string(snippet)), 512)))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	var history SessionHistory
	if err := json.NewDecoder(resp.Body).Decode(&history); err != nil {
		return nil, fmt.Errorf("failed to decode session history: %w", err)
	}

	logInfo(ctx, "kai session history retrieved",
		slog.String("session_id", sessionID),
		slog.Int("message_count", len(history.History)))

	return &history, nil
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

func truncateSnippet(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "…"
}
