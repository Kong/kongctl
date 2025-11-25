package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Kong/sdk-konnect-go/models/sdkerrors"
)

// APIErrorDetails captures common fields returned by Konnect error payloads.
type APIErrorDetails struct {
	Detail            string                `json:"detail"`
	Status            int                   `json:"status"`
	Title             string                `json:"title"`
	Instance          string                `json:"instance"`
	InvalidParameters []APIInvalidParameter `json:"invalid_parameters"`
}

// APIInvalidParameter describes a field-level validation failure.
type APIInvalidParameter struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

// ParseAPIErrorDetails attempts to extract a structured payload from common Konnect API error responses.
func ParseAPIErrorDetails(err error) *APIErrorDetails {
	if err == nil {
		return nil
	}

	var apiErr *sdkerrors.SDKError
	if errors.As(err, &apiErr) {
		if details := decodeAPIErrorBody(apiErr.Body); details != nil {
			if details.Status == 0 {
				details.Status = apiErr.StatusCode
			}
			return details
		}
	}

	return decodeAPIErrorBody(extractJSONBody(err.Error()))
}

func decodeAPIErrorBody(raw string) *APIErrorDetails {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var payload APIErrorDetails
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	return &payload
}

func extractJSONBody(msg string) string {
	start := strings.Index(msg, "{")
	if start == -1 {
		return ""
	}
	return msg[start:]
}

// AppendAPIErrorAttrs adds commonly useful error fields to slog attributes, skipping duplicates.
func AppendAPIErrorAttrs(attrs []any, details *APIErrorDetails) []any {
	if details == nil {
		return attrs
	}

	if details.Status > 0 {
		attrs = AppendIfMissingAttr(attrs, "status", details.Status)
	}
	if title := strings.TrimSpace(details.Title); title != "" {
		attrs = AppendIfMissingAttr(attrs, "title", title)
	}
	if detail := strings.TrimSpace(details.Detail); detail != "" {
		attrs = AppendIfMissingAttr(attrs, "detail", detail)
	}
	if instance := strings.TrimSpace(details.Instance); instance != "" {
		attrs = AppendIfMissingAttr(attrs, "instance", instance)
	}
	if formatted := formatInvalidParameters(details.InvalidParameters); formatted != "" {
		attrs = AppendIfMissingAttr(attrs, "invalid_parameters", formatted)
	}

	return attrs
}

// AppendIfMissingAttr appends a key/value pair if the key is not already present.
func AppendIfMissingAttr(attrs []any, key string, value any) []any {
	if key == "" {
		return attrs
	}
	for i := 0; i+1 < len(attrs); i += 2 {
		if existingKey, ok := attrs[i].(string); ok && existingKey == key {
			return attrs
		}
	}
	return append(attrs, key, value)
}

func formatInvalidParameters(params []APIInvalidParameter) string {
	lines := make([]string, 0, len(params))
	for _, param := range params {
		reason := strings.TrimSpace(strings.ReplaceAll(param.Reason, "\n", " "))
		if reason == "" {
			continue
		}
		if field := strings.TrimSpace(param.Field); field != "" {
			lines = append(lines, fmt.Sprintf("%s: %s", field, reason))
			continue
		}
		lines = append(lines, reason)
	}
	return strings.Join(lines, "\n")
}

// BuildDetailedMessage favors structured detail fields when available for a friendlier summary.
func BuildDetailedMessage(base string, attrs []any, err error) string {
	if detail := ExtractDetailFromAttrs(attrs); detail != "" {
		return fmt.Sprintf("%s: %s", base, detail)
	}
	if detail := ExtractDetailFromError(err); detail != "" {
		return fmt.Sprintf("%s: %s", base, detail)
	}
	if err != nil {
		return fmt.Sprintf("%s: %s", base, SanitizeErrorMessage(err.Error()))
	}
	return base
}

// ExtractDetailFromError returns a human-friendly detail from a Konnect API error.
func ExtractDetailFromError(err error) string {
	if err == nil {
		return ""
	}

	if details := ParseAPIErrorDetails(err); details != nil {
		if detail := strings.TrimSpace(details.Detail); detail != "" {
			return detail
		}
		if title := strings.TrimSpace(details.Title); title != "" {
			return title
		}
	}

	return ""
}

// ExtractDetailFromAttrs scans slog attrs for a detail-rich field.
func ExtractDetailFromAttrs(attrs []any) string {
	for i := 0; i+1 < len(attrs); i += 2 {
		key, ok := attrs[i].(string)
		if !ok {
			continue
		}
		if strings.EqualFold(key, "detail") {
			if value, ok := attrs[i+1].(string); ok {
				return value
			}
		}
		if strings.EqualFold(key, "error") {
			if value, ok := attrs[i+1].(string); ok {
				if detail := parseDetailFromJSON(value); detail != "" {
					return detail
				}
			}
		}
	}
	return ""
}

func parseDetailFromJSON(errStr string) string {
	payload := decodeAPIErrorBody(errStr)
	if payload == nil {
		return ""
	}
	var parts []string
	if payload.Status != 0 {
		parts = append(parts, fmt.Sprintf("status: %d", payload.Status))
	}
	if title := strings.TrimSpace(payload.Title); title != "" {
		parts = append(parts, fmt.Sprintf("title: %s", title))
	}
	if instance := strings.TrimSpace(payload.Instance); instance != "" {
		parts = append(parts, fmt.Sprintf("instance: %s", instance))
	}
	if detail := strings.TrimSpace(payload.Detail); detail != "" {
		parts = append(parts, fmt.Sprintf("detail: %s", detail))
	}
	for _, p := range payload.InvalidParameters {
		reason := strings.TrimSpace(p.Reason)
		if reason == "" {
			continue
		}
		if field := strings.TrimSpace(p.Field); field != "" {
			reason = fmt.Sprintf("%s (field=%s)", reason, field)
		}
		parts = append(parts, reason)
	}
	return strings.Join(parts, "; ")
}

// SanitizeErrorMessage removes noisy JSON payloads and collapses whitespace for friendlier display.
func SanitizeErrorMessage(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}

	if idx := strings.Index(msg, "\n{"); idx != -1 {
		msg = msg[:idx]
	}

	msg = strings.ReplaceAll(msg, "\n", " ")
	msg = strings.Join(strings.Fields(msg), " ")
	return msg
}
