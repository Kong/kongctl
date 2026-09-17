package declarative

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/kong/kongctl/internal/declarative/secrets"
	"github.com/kong/kongctl/internal/theme"
)

const diffSecretWriteLabel = "(secret write; no value comparison)"

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
	displayDiffChange(diffOutputFor(out).child(field), field, oldValue, newValue, hasOld, hasNew, indent, fullContent)
}

func displayDiffChange(
	output *diffOutput, field string, oldValue, newValue any,
	hasOld, hasNew bool, indent string, fullContent bool,
) {
	out := output
	// Compare before redaction, including deferred environment references.
	if hasOld == hasNew && reflect.DeepEqual(oldValue, newValue) {
		return
	}
	fieldText := output.paint(theme.ColorTextSecondary, field)
	marker := output.paint(theme.ColorDiffChanged, "~")
	sensitive := output.sensitive(oldValue) || output.sensitive(newValue)
	oldMap, oldObject := oldValue.(map[string]any)
	newMap, newObject := newValue.(map[string]any)
	if oldObject && newObject && !sensitive {
		fmt.Fprintf(out, "%s%s:\n", indent, fieldText)
		keys := maps.Clone(oldMap)
		maps.Copy(keys, newMap)
		for _, key := range slices.Sorted(maps.Keys(keys)) {
			oldChild, oldExists := oldMap[key]
			newChild, newExists := newMap[key]
			displayDiffChange(output.child(key), key, oldChild, newChild,
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
			displayDiffChange(output.child(strconv.Itoa(i)), fmt.Sprintf("[%d]", i), oldItem, newItem,
				oldExists, newExists, indent+"  ", fullContent)
		}
		return
	}
	if !hasOld {
		fmt.Fprintf(out, "%s%s %s: %s\n", indent, output.paint(theme.ColorDiffAdded, "+"), fieldText,
			output.paint(theme.ColorDiffAdded, output.formatValue(newValue, indent, fullContent)))
		return
	}
	if !hasNew {
		fmt.Fprintf(out, "%s%s %s: %s\n", indent, output.paint(theme.ColorDiffRemoved, "-"), fieldText,
			output.paint(theme.ColorDiffRemoved, output.formatValue(oldValue, indent, fullContent)))
		return
	}
	// Retain explicit null transitions, but never reveal non-null secret values
	// or their types. Describe the write without claiming a remote value comparison.
	if sensitive && oldValue != nil && newValue != nil {
		fmt.Fprintf(out, "%s%s %s: %s\n", indent, marker, fieldText,
			output.paint(theme.ColorTextMuted, diffSecretWriteLabel))
		return
	}
	typeTag := ""
	if !sensitive && reflect.TypeOf(oldValue) != reflect.TypeOf(newValue) {
		typeTag = " (type)"
	}
	oldText := output.formatValue(oldValue, indent+"  ", fullContent)
	newText := output.formatValue(newValue, indent+"  ", fullContent)
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
func (output *diffOutput) formatValue(value any, indent string, fullContent bool) string {
	if value != nil && output.sensitive(value) {
		return diffFieldRedactedValue
	}
	var parts []string
	var opening, closing string
	switch v := value.(type) {
	case map[string]any:
		opening, closing = "{", "}"
		for _, key := range slices.Sorted(maps.Keys(v)) {
			parts = append(parts, fmt.Sprintf("%q: %s", key, output.child(key).formatValue(v[key], indent+"  ", fullContent)))
		}
	case []any:
		opening, closing = "[", "]"
		for i, item := range v {
			parts = append(parts, output.child(strconv.Itoa(i)).formatValue(item, indent+"  ", fullContent))
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

func (output *diffOutput) child(segment string) *diffOutput {
	child := *output
	segment = strings.ReplaceAll(strings.ReplaceAll(segment, "~", "~0"), "/", "~1")
	child.path += "/" + segment
	return &child
}

func (output *diffOutput) sensitive(value any) bool {
	if reference, ok := value.(string); ok && secrets.IsVaultReference(reference) {
		return false
	}
	_, ok := secrets.Match(output.resourceType, output.path)
	return ok
}
