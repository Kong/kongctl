package declarative

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"reflect"
	"slices"
	"strings"
)

// Use the plan's JSON representation to handle typed maps, slices, and pointers
// identically before and after saving a plan. Keep numbers lossless.
func normalizeDiffValue(value any) any {
	data, err := json.Marshal(value)
	if err != nil {
		// Unsupported values cannot occur in saved plans. Do not fall back to a
		// Go representation that could expose nested secrets.
		return "<unsupported value>"
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var normalized any
	if err := decoder.Decode(&normalized); err != nil {
		return "<unsupported value>"
	}
	return normalized
}

func displayNestedFieldChange(
	out io.Writer, field string, oldValue, newValue any,
	hasOld, hasNew bool, indent string, fullContent bool,
) {
	// Compare before redaction, including deferred environment references.
	if hasOld == hasNew && reflect.DeepEqual(oldValue, newValue) {
		return
	}
	oldMap, oldObject := oldValue.(map[string]any)
	newMap, newObject := newValue.(map[string]any)
	if oldObject && newObject && !isSensitiveDiffField(field) {
		var children strings.Builder
		keys := maps.Clone(oldMap)
		maps.Copy(keys, newMap)
		for _, key := range slices.Sorted(maps.Keys(keys)) {
			oldChild, oldExists := oldMap[key]
			newChild, newExists := newMap[key]
			displayNestedFieldChange(&children, key, oldChild, newChild,
				oldExists, newExists, indent+"  ", fullContent)
		}
		if children.Len() > 0 {
			fmt.Fprintf(out, "%s%s:\n%s", indent, field, children.String())
		}
		return
	}
	if !hasOld {
		fmt.Fprintf(out, "%s+ %s: %s\n", indent, field, formatDiffValue(field, newValue, indent, fullContent))
		return
	}
	if !hasNew {
		fmt.Fprintf(out, "%s- %s: %s\n", indent, field, formatDiffValue(field, oldValue, indent, fullContent))
		return
	}
	oldText := formatDiffValue(field, oldValue, indent+"  ", fullContent)
	newText := formatDiffValue(field, newValue, indent+"  ", fullContent)
	if strings.Contains(oldText, "\n") || strings.Contains(newText, "\n") || len(oldText)+len(newText) > 100 {
		fmt.Fprintf(out, "%s~ %s:\n%s  - %s\n%s  + %s\n", indent, field, indent, oldText, indent, newText)
		return
	}
	fmt.Fprintf(out, "%s~ %s: %s → %s\n", indent, field, oldText, newText)
}

// Format containers only after recursively formatting their children. Both the
// inline and multiline forms therefore share the same redaction rules.
func formatDiffValue(field string, value any, indent string, fullContent bool) string {
	if isSensitiveDiffField(field) {
		return formatFieldValueForField(field, value, fullContent)
	}
	var parts []string
	var opening, closing string
	switch v := value.(type) {
	case map[string]any:
		opening, closing = "{", "}"
		for _, key := range slices.Sorted(maps.Keys(v)) {
			parts = append(parts, fmt.Sprintf("%q: %s", key, formatDiffValue(key, v[key], indent+"  ", fullContent)))
		}
	case []any:
		opening, closing = "[", "]"
		for _, item := range v {
			parts = append(parts, formatDiffValue("", item, indent+"  ", fullContent))
		}
	default:
		return formatFieldValue(value, fullContent)
	}
	inline := opening + strings.Join(parts, ", ") + closing
	if len(inline) <= 80 && !strings.Contains(inline, "\n") {
		return inline
	}
	return opening + "\n" + indent + "  " + strings.Join(parts, ",\n"+indent+"  ") + "\n" + indent + closing
}
