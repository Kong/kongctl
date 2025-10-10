package kai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/kong/kongctl/internal/meta"
)

type TaskAction string

const (
	TaskActionStart TaskAction = "start"
	TaskActionStop  TaskAction = "stop"
)

type TaskDetails struct {
	ID                  string         `json:"id"`
	SessionID           string         `json:"session_id"`
	TriggerID           string         `json:"trigger_id"`
	Type                string         `json:"type"`
	Context             []ChatContext  `json:"context"`
	ToolCallMetadata    map[string]any `json:"tool_call_metadata"`
	Status              string         `json:"status"`
	CreatedAt           time.Time      `json:"created_at"`
	ExpiresAt           time.Time      `json:"expires_at"`
	ConfirmationMessage string         `json:"confirmation_message"`
}

type TaskStatusEvent struct {
	Status  string
	Message string
}

type TaskStatusStream struct {
	Events <-chan TaskStatusEvent
	err    <-chan error
}

func (s *TaskStatusStream) Err() error {
	if s == nil {
		return nil
	}
	return <-s.err
}

func ListActiveTasks(
	ctx context.Context,
	client *http.Client,
	baseURL, token, sessionID string,
) ([]TaskDetails, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, fmt.Errorf("sessionID cannot be empty")
	}

	endpoint, err := JoinSessionPath(baseURL, sessionID, "tasks", "active")
	if err != nil {
		return nil, fmt.Errorf("failed to construct active tasks endpoint: %w", err)
	}

	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build active tasks request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("User-Agent", meta.CLIName)

	logDebug(ctx, "kai list active tasks request",
		slog.String("endpoint", endpoint),
		slog.String("session_id", sessionID))

	resp, err := client.Do(req)
	if err != nil {
		logError(ctx, "kai list active tasks request failed",
			slog.String("endpoint", endpoint),
			slog.String("session_id", sessionID),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to execute active tasks request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		logDebug(ctx, "kai list active tasks empty",
			slog.String("session_id", sessionID))
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		logError(ctx, "kai list active tasks unexpected status",
			slog.String("endpoint", endpoint),
			slog.String("session_id", sessionID),
			slog.Int("status", resp.StatusCode),
			slog.String("snippet", truncateSnippet(strings.TrimSpace(string(snippet)), 2048)))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read active tasks response: %w", err)
	}

	body = bytes.TrimSpace(body)
	if len(body) == 0 || bytes.Equal(body, []byte("null")) {
		logDebug(ctx, "kai list active tasks empty body",
			slog.String("session_id", sessionID))
		return nil, nil
	}

	var single TaskDetails
	if err := json.Unmarshal(body, &single); err == nil && single.ID != "" {
		logDebug(ctx, "kai active task",
			slog.String("session_id", sessionID),
			slog.String("task_id", single.ID),
			slog.String("status", strings.ToLower(single.Status)),
			slog.Any("tool_metadata", single.ToolCallMetadata))
		logInfo(ctx, "kai list active tasks",
			slog.String("session_id", sessionID),
			slog.Int("count", 1))
		return []TaskDetails{single}, nil
	}

	var wrapper struct {
		Data []TaskDetails `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err == nil {
		for _, t := range wrapper.Data {
			logDebug(ctx, "kai active task",
				slog.String("session_id", sessionID),
				slog.String("task_id", t.ID),
				slog.String("status", strings.ToLower(t.Status)),
				slog.Any("tool_metadata", t.ToolCallMetadata))
		}
		logInfo(ctx, "kai list active tasks",
			slog.String("session_id", sessionID),
			slog.Int("count", len(wrapper.Data)))
		return wrapper.Data, nil
	}

	return nil, fmt.Errorf("failed to decode active tasks response")
}

func UpdateTask(
	ctx context.Context,
	client *http.Client,
	baseURL, token, sessionID, taskID string,
	action TaskAction,
) (*TaskDetails, error) {
	if strings.TrimSpace(sessionID) == "" || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("sessionID and taskID cannot be empty")
	}
	endpoint, err := JoinSessionPath(baseURL, sessionID, "tasks", taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to construct update task endpoint: %w", err)
	}

	if client == nil {
		client = http.DefaultClient
	}

	payload, err := json.Marshal(map[string]string{"action": string(action)})
	if err != nil {
		return nil, fmt.Errorf("failed to encode task action: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to build update task request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", meta.CLIName)

	logDebug(ctx, "kai update task request",
		slog.String("endpoint", endpoint),
		slog.String("session_id", sessionID),
		slog.String("task_id", taskID),
		slog.String("action", string(action)))

	resp, err := client.Do(req)
	if err != nil {
		logError(ctx, "kai update task request failed",
			slog.String("endpoint", endpoint),
			slog.String("session_id", sessionID),
			slog.String("task_id", taskID),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to execute update task request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		logError(ctx, "kai update task unexpected status",
			slog.String("endpoint", endpoint),
			slog.String("session_id", sessionID),
			slog.String("task_id", taskID),
			slog.Int("status", resp.StatusCode),
			slog.String("snippet", truncateSnippet(strings.TrimSpace(string(snippet)), 2048)))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	var details TaskDetails
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return nil, fmt.Errorf("failed to decode task details: %w", err)
	}

	logInfo(ctx, "kai task updated",
		slog.String("session_id", sessionID),
		slog.String("task_id", taskID),
		slog.String("action", string(action)),
		slog.String("status", strings.ToLower(details.Status)),
		slog.Any("tool_metadata", details.ToolCallMetadata))

	return &details, nil
}

func StreamTaskStatus(
	ctx context.Context,
	client *http.Client,
	baseURL, token, sessionID, taskID string,
) (*TaskStatusStream, error) {
	if strings.TrimSpace(sessionID) == "" || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("sessionID and taskID cannot be empty")
	}

	endpoint, err := JoinSessionPath(baseURL, sessionID, "tasks", taskID, "status")
	if err != nil {
		return nil, fmt.Errorf("failed to construct task status endpoint: %w", err)
	}

	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build task status request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("User-Agent", meta.CLIName)

	logDebug(ctx, "kai task status stream request",
		slog.String("endpoint", endpoint),
		slog.String("session_id", sessionID),
		slog.String("task_id", taskID))

	resp, err := client.Do(req)
	if err != nil {
		logError(ctx, "kai task status stream request failed",
			slog.String("endpoint", endpoint),
			slog.String("session_id", sessionID),
			slog.String("task_id", taskID),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to execute task status request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		logError(ctx, "kai task status stream unexpected status",
			slog.String("endpoint", endpoint),
			slog.String("session_id", sessionID),
			slog.String("task_id", taskID),
			slog.Int("status", resp.StatusCode),
			slog.String("snippet", truncateSnippet(strings.TrimSpace(string(snippet)), 2048)))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	logDebug(ctx, "kai task status stream established",
		slog.String("session_id", sessionID),
		slog.String("task_id", taskID))

	events := make(chan TaskStatusEvent)
	errCh := make(chan error, 1)

	go func() {
		defer resp.Body.Close()
		defer close(events)
		defer close(errCh)

		err := decodeSSE(ctx, resp.Body, func(evt ChatEvent) error {
			if evt.Event != "" {
				logTrace(ctx, "kai task status event",
					slog.String("session_id", sessionID),
					slog.String("task_id", taskID),
					slog.String("event", evt.Event))
			}
			switch evt.Event {
			case "task-status":
				status, message := parseTaskStatusEvent(evt.Data)
				events <- TaskStatusEvent{Status: status, Message: message}
			case "end":
				return io.EOF
			case "error":
				errMsg := fmt.Errorf("task status error: %s", extractErrorMessage(evt.Data))
				errCh <- errMsg
				logError(ctx, "kai task status event error",
					slog.String("session_id", sessionID),
					slog.String("task_id", taskID),
					slog.String("error", errMsg.Error()))
				return io.EOF
			}
			return nil
		})
		if errors.Is(err, io.EOF) {
			errCh <- nil
			return
		}
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				logError(ctx, "kai task status stream error",
					slog.String("session_id", sessionID),
					slog.String("task_id", taskID),
					slog.String("error", err.Error()))
			}
			errCh <- err
		} else {
			errCh <- nil
		}
	}()

	return &TaskStatusStream{
		Events: events,
		err:    errCh,
	}, nil
}

func parseTaskStatusEvent(data any) (string, string) {
	if data == nil {
		return "", ""
	}
	if root, ok := data.(map[string]any); ok {
		if inner, ok := root["data"].(map[string]any); ok {
			status, _ := inner["status"].(string)
			message, _ := inner["message"].(string)
			return status, message
		}
		if status, ok := root["status"].(string); ok {
			message, _ := root["message"].(string)
			return status, message
		}
	}
	if s, ok := data.(string); ok {
		return s, ""
	}
	return "", ""
}

func AnalyzeTaskStream(
	ctx context.Context,
	client *http.Client,
	baseURL, token, sessionID, taskID string,
) (*Stream, error) {
	if strings.TrimSpace(sessionID) == "" || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("sessionID and taskID cannot be empty")
	}

	endpoint, err := JoinSessionPath(baseURL, sessionID, "tasks", taskID, "analyze")
	if err != nil {
		return nil, fmt.Errorf("failed to construct analyze task endpoint: %w", err)
	}

	if client == nil {
		client = http.DefaultClient
	}

	body := bytes.NewReader([]byte("{}"))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to build analyze task request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("User-Agent", meta.CLIName)

	logDebug(ctx, "kai analyze task request",
		slog.String("endpoint", endpoint),
		slog.String("session_id", sessionID),
		slog.String("task_id", taskID))

	resp, err := client.Do(req)
	if err != nil {
		logError(ctx, "kai analyze task request failed",
			slog.String("endpoint", endpoint),
			slog.String("session_id", sessionID),
			slog.String("task_id", taskID),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to execute analyze task request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		logError(ctx, "kai analyze task unexpected status",
			slog.String("endpoint", endpoint),
			slog.String("session_id", sessionID),
			slog.String("task_id", taskID),
			slog.Int("status", resp.StatusCode),
			slog.String("snippet", truncateSnippet(strings.TrimSpace(string(snippet)), 2048)))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	logDebug(ctx, "kai analyze task stream established",
		slog.String("session_id", sessionID),
		slog.String("task_id", taskID))

	events := make(chan ChatEvent)
	errCh := make(chan error, 1)

	go func() {
		defer resp.Body.Close()
		defer close(events)
		defer close(errCh)

		err := decodeSSE(ctx, resp.Body, func(evt ChatEvent) error {
			if evt.Event != "" {
				logTrace(ctx, "kai analyze task event",
					slog.String("session_id", sessionID),
					slog.String("task_id", taskID),
					slog.String("event", evt.Event))
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case events <- evt:
				return nil
			}
		})
		if errors.Is(err, io.EOF) {
			errCh <- nil
			return
		}
		if err != nil && !errors.Is(err, context.Canceled) {
			logError(ctx, "kai analyze task stream error",
				slog.String("session_id", sessionID),
				slog.String("task_id", taskID),
				slog.String("error", err.Error()))
		}
		errCh <- err
	}()

	return &Stream{Events: events, err: errCh}, nil
}
