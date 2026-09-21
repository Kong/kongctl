//go:build e2e

package harness

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"time"
)

const teamCreateMarker = "e2e-create-id"

// CreateOutcomeUnknownError prevents replaying a mutation whose effect is unknown.
type CreateOutcomeUnknownError struct{ Cause error }

func (e *CreateOutcomeUnknownError) Error() string {
	return fmt.Sprintf("create outcome unknown; POST will not be repeated: %v", e.Cause)
}
func (e *CreateOutcomeUnknownError) Unwrap() error { return e.Cause }
func IsCreateOutcomeUnknown(err error) bool {
	var unknown *CreateOutcomeUnknownError
	return errors.As(err, &unknown)
}

func markTeamCreate(body []byte) ([]byte, map[string]any, string, error) {
	var desired map[string]any
	if err := json.Unmarshal(body, &desired); err != nil || desired == nil {
		return nil, nil, "", fmt.Errorf("team create requires a JSON object")
	}
	labels, ok := desired["labels"].(map[string]any)
	if desired["labels"] != nil && !ok {
		return nil, nil, "", fmt.Errorf("team labels must be an object")
	}
	if labels == nil {
		labels = map[string]any{}
	}
	if _, exists := labels[teamCreateMarker]; exists {
		return nil, nil, "", fmt.Errorf("team label %s is reserved for create recovery", teamCreateMarker)
	}
	marker := rand.Text()
	labels[teamCreateMarker] = marker
	desired["labels"] = labels
	encoded, err := json.Marshal(desired)
	return encoded, desired, marker, err
}

func (s *Step) recoverTeamCreate(
	desired map[string]any, marker string, budget time.Duration,
) (resourceRequestResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	dir, err := s.cli.allocateCommandDir("recover-team-create")
	if err != nil {
		return resourceRequestResult{}, err
	}
	for {
		var matches []map[string]any
		var last resourceRequestResult
		// Inspect all pages before accepting uniqueness. Never follow a server-supplied URL.
		for page := 1; ; page++ {
			if page > 100 {
				return last, fmt.Errorf("team recovery inventory exceeds page limit")
			}
			endpoints := map[string]resourceEndpoint{"recovery": {
				Method: http.MethodGet, Path: fmt.Sprintf("/v3/teams?page[number]=%d&page[size]=100", page), UseGlobal: true,
			}}
			result, err := s.requestResource("recovery", nil, endpoints, resourceRequestOptions{
				Context: ctx, DiagnosticsOnly: true, ArtifactDir: dir, ExpectStatus: http.StatusOK,
			})
			last = result
			if err != nil {
				if ctx.Err() != nil {
					return last, fmt.Errorf("team recovery deadline exceeded: %w", ctx.Err())
				}
				if result.Status < 500 && result.Status != http.StatusTooManyRequests &&
					!ShouldRetryResetHTTPAttempt(err, err.Error()) {
					return last, fmt.Errorf("team recovery read failed: %w", err)
				}
				matches = nil
				break
			}
			var envelope struct {
				Data []map[string]any `json:"data"`
			}
			if err := json.Unmarshal(result.Body, &envelope); err != nil || envelope.Data == nil {
				return last, fmt.Errorf("invalid team recovery response")
			}
			for _, team := range envelope.Data {
				labels, _ := team["labels"].(map[string]any)
				if labels[teamCreateMarker] == marker {
					matches = append(matches, team)
				}
			}
			if len(envelope.Data) < 100 {
				break
			}
		}
		if len(matches) > 1 {
			return last, fmt.Errorf("multiple teams match the create recovery marker")
		}
		if len(matches) == 1 {
			team := matches[0]
			id, _ := team["id"].(string)
			if id == "" || !containsCreateFields(team, desired) {
				return last, fmt.Errorf("recovered team conflicts with requested fields")
			}
			last.Parsed = team
			last.Body, _ = json.Marshal(team)
			evidence, _ := json.Marshal(map[string]any{"outcome": "recovered", "id": id, "marker": marker})
			if dir != "" {
				if err := os.WriteFile(filepath.Join(dir, "create-recovery.json"), evidence, 0o600); err != nil {
					return last, fmt.Errorf("record team recovery: %w", err)
				}
			}
			return last, nil
		}
		if err := sleepWithContext(ctx, time.Second); err != nil {
			return last, fmt.Errorf("no matching team observed before recovery deadline: %w", err)
		}
	}
}

func containsCreateFields(actual, desired map[string]any) bool {
	for key, value := range desired {
		if nested, ok := value.(map[string]any); ok {
			other, ok := actual[key].(map[string]any)
			if !ok || !containsCreateFields(other, nested) {
				return false
			}
		} else if !reflect.DeepEqual(actual[key], value) {
			return false
		}
	}
	return true
}
