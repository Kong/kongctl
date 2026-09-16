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

	"github.com/kong/kongctl/internal/theme"
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
	output := diffOutputFor(out)
	fieldText := output.paint(theme.ColorTextSecondary, field)
	marker := output.paint(theme.ColorDiffChanged, "~")
	sensitive := isSensitiveDiffField(field)
	oldMap, oldObject := oldValue.(map[string]any)
	newMap, newObject := newValue.(map[string]any)
	if oldObject && newObject && !sensitive {
		fmt.Fprintf(out, "%s%s:\n", indent, fieldText)
		keys := maps.Clone(oldMap)
		maps.Copy(keys, newMap)
		for _, key := range slices.Sorted(maps.Keys(keys)) {
			oldChild, oldExists := oldMap[key]
			newChild, newExists := newMap[key]
			displayNestedFieldChange(out, key, oldChild, newChild,
				oldExists, newExists, indent+"  ", fullContent)
		}
		return
	}
	oldArray, oldIsArray := oldValue.([]any)
	newArray, newIsArray := newValue.([]any)
	if oldIsArray && newIsArray && !sensitive {
		fmt.Fprintf(out, "%s%s:\n", indent, fieldText)
		for i := range max(len(oldArray), len(newArray)) {
			var oldItem, newItem any
			oldExists, newExists := i < len(oldArray), i < len(newArray)
			if oldExists {
				oldItem = oldArray[i]
			}
			if newExists {
				newItem = newArray[i]
			}
			displayNestedFieldChange(out, fmt.Sprintf("[%d]", i), oldItem, newItem,
				oldExists, newExists, indent+"  ", fullContent)
		}
		return
	}
	if !hasOld {
		fmt.Fprintf(out, "%s%s %s: %s\n", indent, output.paint(theme.ColorDiffAdded, "+"), fieldText,
			output.paint(theme.ColorDiffAdded, formatDiffValue(field, newValue, indent, fullContent)))
		return
	}
	if !hasNew {
		fmt.Fprintf(out, "%s%s %s: %s\n", indent, output.paint(theme.ColorDiffRemoved, "-"), fieldText,
			output.paint(theme.ColorDiffRemoved, formatDiffValue(field, oldValue, indent, fullContent)))
		return
	}
	// Retain explicit null transitions, but never reveal non-null secret values
	// or their types. The comparison above still detects secret-only changes.
	if sensitive && oldValue != nil && newValue != nil {
		fmt.Fprintf(out, "%s%s %s: %s\n", indent, marker, fieldText,
			output.paint(theme.ColorTextMuted, "(sensitive value changed)"))
		return
	}
	typeTag := ""
	if !sensitive && reflect.TypeOf(oldValue) != reflect.TypeOf(newValue) {
		typeTag = " (type)"
	}
	oldText := formatDiffValue(field, oldValue, indent+"  ", fullContent)
	newText := formatDiffValue(field, newValue, indent+"  ", fullContent)
	multiline := strings.Contains(oldText, "\n") || strings.Contains(newText, "\n") || len(oldText)+len(newText) > 100
	oldText = output.paint(theme.ColorDiffRemoved, oldText)
	newText = output.paint(theme.ColorDiffAdded, newText)
	typeTag = output.paint(theme.ColorTextMuted, typeTag)
	if multiline {
		fmt.Fprintf(out, "%s%s %s:%s\n%s  %s %s\n%s  %s %s\n", indent, marker, fieldText, typeTag,
			indent, output.paint(theme.ColorDiffRemoved, "-"), oldText,
			indent, output.paint(theme.ColorDiffAdded, "+"), newText)
		return
	}
	fmt.Fprintf(out, "%s%s %s: %s → %s%s\n", indent, marker, fieldText, oldText, newText, typeTag)
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
