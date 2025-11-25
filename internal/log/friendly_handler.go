package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
)

// NewFriendlyErrorHandler returns a slog.Handler that renders error records in a
// concise, human-friendly format suitable for console output.
func NewFriendlyErrorHandler(w io.Writer) slog.Handler {
	return &friendlyHandler{
		w:       w,
		attrs:   nil,
		groups:  nil,
		enabled: true,
	}
}

type friendlyHandler struct {
	w       io.Writer
	attrs   []slog.Attr
	groups  []string
	enabled bool
}

type attrEntry struct {
	key   string
	value string
}

func (h *friendlyHandler) Enabled(_ context.Context, level slog.Level) bool {
	if !h.enabled {
		return false
	}
	return level >= slog.LevelError
}

func (h *friendlyHandler) Handle(_ context.Context, record slog.Record) error {
	summary := strings.TrimSpace(record.Message)
	entries := h.collectEntries(record)

	if summary == "" {
		for _, entry := range entries {
			if entry.key == "error" && entry.value != "" {
				summary = entry.value
				break
			}
		}
	}

	if summary == "" {
		summary = "an unknown error occurred"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Error: %s\n", summary)

	for _, entry := range entries {
		if entry.key == "suggestion" && entry.value != "" {
			fmt.Fprintf(&sb, "  suggestion: %s\n", entry.value)
		}
	}

	// Sort remaining entries to provide stable output.
	others := make([]attrEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.key == "suggestion" || entry.key == "error" || entry.value == "" {
			continue
		}
		others = append(others, entry)
	}

	sort.SliceStable(others, func(i, j int) bool {
		return others[i].key < others[j].key
	})

	for _, entry := range others {
		writeEntry(&sb, entry)
	}

	_, err := io.WriteString(h.w, sb.String())
	return err
}

func (h *friendlyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := h.clone()
	clone.attrs = append(clone.attrs, attrs...)
	return clone
}

func (h *friendlyHandler) WithGroup(name string) slog.Handler {
	clone := h.clone()
	clone.groups = append(clone.groups, name)
	return clone
}

func (h *friendlyHandler) clone() *friendlyHandler {
	clone := *h
	if len(h.attrs) > 0 {
		clone.attrs = append([]slog.Attr{}, h.attrs...)
	}
	if len(h.groups) > 0 {
		clone.groups = append([]string{}, h.groups...)
	}
	return &clone
}

func (h *friendlyHandler) collectEntries(record slog.Record) []attrEntry {
	entries := make([]attrEntry, 0, len(h.attrs)+record.NumAttrs())

	for _, attr := range h.attrs {
		entries = append(entries, attrEntry{
			key:   h.fullKey(attr.Key),
			value: h.attrValueToString(attr.Value.Resolve()),
		})
	}

	record.Attrs(func(attr slog.Attr) bool {
		entries = append(entries, attrEntry{
			key:   h.fullKey(attr.Key),
			value: h.attrValueToString(attr.Value.Resolve()),
		})
		return true
	})

	return entries
}

func (h *friendlyHandler) fullKey(key string) string {
	if len(h.groups) == 0 {
		return key
	}
	path := append([]string{}, h.groups...)
	path = append(path, key)
	return strings.Join(path, ".")
}

func (h *friendlyHandler) attrValueToString(val slog.Value) string {
	switch val.Kind() {
	case slog.KindString:
		return val.String()
	case slog.KindInt64, slog.KindUint64, slog.KindFloat64, slog.KindBool, slog.KindDuration, slog.KindTime:
		return val.String()
	case slog.KindGroup:
		groupVals := val.Group()
		parts := make([]string, 0, len(groupVals))
		for _, attr := range groupVals {
			parts = append(parts, fmt.Sprintf("%s=%s", attr.Key, h.attrValueToString(attr.Value)))
		}
		return strings.Join(parts, ", ")
	case slog.KindLogValuer:
		return h.attrValueToString(val.Resolve())
	case slog.KindAny:
		raw := val.Any()
		if err, ok := raw.(error); ok {
			return err.Error()
		}
		return fmt.Sprint(raw)
	default:
		return val.String()
	}
}

func writeEntry(sb *strings.Builder, entry attrEntry) {
	val := strings.TrimSpace(entry.value)
	if strings.Contains(val, "\n") {
		lines := strings.Split(val, "\n")
		fmt.Fprintf(sb, "  %s: %s\n", entry.key, strings.TrimSpace(lines[0]))
		for _, line := range lines[1:] {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			fmt.Fprintf(sb, "    %s\n", trimmed)
		}
		return
	}
	fmt.Fprintf(sb, "  %s: %s\n", entry.key, val)
}
