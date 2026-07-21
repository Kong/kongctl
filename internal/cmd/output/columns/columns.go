package columns

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	cmdpkg "github.com/kong/kongctl/internal/cmd"
	cmdcommon "github.com/kong/kongctl/internal/cmd/common"
	"github.com/kong/kongctl/internal/cmd/output/jq"
	"github.com/kong/kongctl/internal/config"
	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
)

const (
	FlagName       = "columns"
	MaxColumnWidth = 40
)

// Column is a user-selected text column and its compiled field path.
type Column struct {
	Header string
	Path   string
	steps  []pathStep
}

type pathStep struct {
	key   *string
	index *int
	slice *stringSlice
}

type stringSlice struct {
	start *int
	end   *int
}

// AddFlags adds custom text-column selection to a command flag set.
func AddFlags(flags *pflag.FlagSet) {
	if flags == nil || flags.Lookup(FlagName) != nil {
		return
	}
	flags.StringArray(
		FlagName,
		nil,
		"Select text columns as HEADER=.field (repeatable or comma-separated). "+
			"Supports nested fields, quoted keys, array indexes, and string slices.",
	)
}

func ValidateColumnFlags(helper cmdpkg.Helper, cfg config.Hook) error {
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}
	selected, err := Resolve(helper.GetCmd(), outType)
	if err != nil {
		return &cmdpkg.ConfigurationError{Err: err}
	}
	settings, err := jq.ResolveSettings(helper.GetCmd(), cfg)
	if err != nil {
		return err
	}
	if len(selected) > 0 && jq.HasFilter(settings) {
		return &cmdpkg.ConfigurationError{
			Err: fmt.Errorf("--%s cannot be combined with --%s", FlagName, jq.FlagName),
		}
	}
	return nil
}

// Resolve reads and validates custom columns from cmd. An empty result means
// the command should use its built-in text columns.
func Resolve(cmd *cobra.Command, outType cmdcommon.OutputFormat) ([]Column, error) {
	if cmd == nil || cmd.Flags().Lookup(FlagName) == nil {
		return nil, nil
	}

	values, err := cmd.Flags().GetStringArray(FlagName)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, nil
	}
	if outType != cmdcommon.TEXT {
		return nil, fmt.Errorf("--%s is only supported with --output text", FlagName)
	}
	return Parse(values)
}

// Parse compiles repeated or comma-separated HEADER=.field specifications.
func Parse(values []string) ([]Column, error) {
	var specs []string
	for _, value := range values {
		parts, err := splitSpecifications(value)
		if err != nil {
			return nil, err
		}
		specs = append(specs, parts...)
	}
	if len(specs) == 0 {
		return nil, fmt.Errorf("--%s requires at least one column", FlagName)
	}

	columns := make([]Column, 0, len(specs))
	seen := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		header, path, ok := strings.Cut(spec, "=")
		header = strings.TrimSpace(header)
		path = strings.TrimSpace(path)
		if !ok || header == "" || path == "" {
			return nil, fmt.Errorf("invalid --%s value %q: expected HEADER=.field", FlagName, spec)
		}
		key := strings.ToLower(header)
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate --%s header %q", FlagName, header)
		}
		steps, err := parsePath(path)
		if err != nil {
			return nil, fmt.Errorf("invalid path for column %q: %w", header, err)
		}
		seen[key] = struct{}{}
		columns = append(columns, Column{Header: header, Path: path, steps: steps})
	}
	return columns, nil
}

func splitSpecifications(value string) ([]string, error) {
	var parts []string
	start := 0
	depth := 0
	inString := false
	escaped := false
	for i, r := range value {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch r {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch r {
		case '"':
			inString = true
		case '[':
			depth++
		case ']':
			depth--
			if depth < 0 {
				return nil, errors.New("invalid --columns value: unexpected closing bracket")
			}
		case ',':
			if depth == 0 {
				part := strings.TrimSpace(value[start:i])
				if part == "" {
					return nil, errors.New("invalid --columns value: empty column")
				}
				parts = append(parts, part)
				start = i + 1
			}
		}
	}
	if inString || depth != 0 {
		return nil, errors.New("invalid --columns value: unterminated quoted key or bracket")
	}
	part := strings.TrimSpace(value[start:])
	if part == "" {
		return nil, errors.New("invalid --columns value: empty column")
	}
	return append(parts, part), nil
}

func parsePath(path string) ([]pathStep, error) {
	if path == "." {
		return nil, nil
	}
	if !strings.HasPrefix(path, ".") {
		return nil, errors.New("path must start with '.'")
	}

	var steps []pathStep
	for i := 1; i < len(path); {
		switch path[i] {
		case '.':
			i++
			if i == len(path) {
				return nil, errors.New("field name cannot be empty")
			}
		case '[':
			step, next, err := parseBracket(path, i)
			if err != nil {
				return nil, err
			}
			steps = append(steps, step)
			i = next
			continue
		}

		start := i
		for i < len(path) && path[i] != '.' && path[i] != '[' {
			i++
		}
		if start == i {
			return nil, fmt.Errorf("unexpected character %q", path[i])
		}
		field := path[start:i]
		if strings.IndexFunc(field, unicode.IsSpace) >= 0 {
			return nil, fmt.Errorf("field %q contains whitespace; use a quoted key", field)
		}
		steps = append(steps, pathStep{key: &field})
	}
	return steps, nil
}

func parseBracket(path string, start int) (pathStep, int, error) {
	end := -1
	inString := false
	escaped := false
	for i := start + 1; i < len(path); i++ {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch path[i] {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch path[i] {
		case '"':
			inString = true
		case ']':
			end = i
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 || inString {
		return pathStep{}, 0, errors.New("unterminated bracket")
	}
	content := strings.TrimSpace(path[start+1 : end])
	if content == "" {
		return pathStep{}, 0, errors.New("empty bracket")
	}
	if strings.HasPrefix(content, "\"") {
		var key string
		if err := json.Unmarshal([]byte(content), &key); err != nil {
			return pathStep{}, 0, fmt.Errorf("invalid quoted key: %w", err)
		}
		return pathStep{key: &key}, end + 1, nil
	}
	if strings.Contains(content, ":") {
		slice, err := parseStringSlice(content)
		if err != nil {
			return pathStep{}, 0, err
		}
		return pathStep{slice: &slice}, end + 1, nil
	}
	index, err := strconv.Atoi(content)
	if err != nil || index < 0 {
		return pathStep{}, 0, fmt.Errorf("invalid array index %q", content)
	}
	return pathStep{index: &index}, end + 1, nil
}

func parseStringSlice(content string) (stringSlice, error) {
	startValue, endValue, ok := strings.Cut(content, ":")
	if !ok || strings.Contains(endValue, ":") || (startValue == "" && endValue == "") {
		return stringSlice{}, fmt.Errorf("invalid string slice %q: expected [start:end]", content)
	}

	start, err := parseSliceBound(startValue)
	if err != nil {
		return stringSlice{}, fmt.Errorf("invalid string slice start %q: %w", startValue, err)
	}
	end, err := parseSliceBound(endValue)
	if err != nil {
		return stringSlice{}, fmt.Errorf("invalid string slice end %q: %w", endValue, err)
	}
	if start != nil && end != nil && *start > *end {
		return stringSlice{}, fmt.Errorf("invalid string slice %q: start exceeds end", content)
	}
	return stringSlice{start: start, end: end}, nil
}

func parseSliceBound(value string) (*int, error) {
	if value == "" {
		return nil, nil
	}
	bound, err := strconv.Atoi(value)
	if err != nil || bound < 0 {
		return nil, errors.New("slice bound must be a non-negative integer")
	}
	return &bound, nil
}

// Project converts raw structured output into headers and text rows.
func Project(raw any, columns []Column) ([]string, [][]string, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("encode column data: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, nil, fmt.Errorf("decode column data: %w", err)
	}

	items, ok := decoded.([]any)
	if !ok {
		items = []any{decoded}
	}
	headers := make([]string, len(columns))
	rows := make([][]string, len(items))
	for i, column := range columns {
		headers[i] = column.Header
	}
	for i, item := range items {
		rows[i] = make([]string, len(columns))
		for j, column := range columns {
			rows[i][j] = formatValue(evaluate(item, column.steps))
		}
	}
	return headers, rows, nil
}

func evaluate(value any, steps []pathStep) any {
	for _, step := range steps {
		if step.key != nil {
			object, ok := value.(map[string]any)
			if !ok {
				return nil
			}
			value = object[*step.key]
			continue
		}
		if step.slice != nil {
			text, ok := value.(string)
			if !ok {
				return nil
			}
			value = sliceString(text, *step.slice)
			continue
		}
		array, ok := value.([]any)
		if !ok || *step.index >= len(array) {
			return nil
		}
		value = array[*step.index]
	}
	return value
}

func sliceString(value string, bounds stringSlice) string {
	runes := []rune(value)
	start := 0
	if bounds.start != nil {
		start = min(*bounds.start, len(runes))
	}
	end := len(runes)
	if bounds.end != nil {
		end = min(*bounds.end, len(runes))
	}
	return string(runes[start:end])
}

func formatValue(value any) string {
	if value == nil {
		return ""
	}
	switch value := value.(type) {
	case string:
		return singleLine(value)
	case bool:
		return strconv.FormatBool(value)
	case json.Number:
		return value.String()
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return string(data)
	}
}

func singleLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

// RenderAutoWidth detects the output terminal width and falls back to 120
// columns when the output does not expose a terminal file descriptor.
func RenderAutoWidth(out io.Writer, headers []string, rows [][]string) error {
	return Render(out, headers, rows, terminalWidth(out))
}

func terminalWidth(out io.Writer) int {
	const defaultWidth = 120

	fdWriter, ok := out.(interface{ Fd() uintptr })
	if !ok {
		return defaultWidth
	}
	fd := fdWriter.Fd()
	if fd == ^uintptr(0) {
		return defaultWidth
	}
	width, _, err := term.GetSize(int(fd))
	if err != nil || width <= 0 {
		return defaultWidth
	}
	return width
}

// Render writes a plain, aligned table with display-width-aware truncation.
func Render(out io.Writer, headers []string, rows [][]string, availableWidth int) error {
	if out == nil {
		return errors.New("column output stream is unavailable")
	}
	if len(headers) == 0 {
		return nil
	}
	if availableWidth <= 0 {
		availableWidth = 120
	}
	widths := calculateWidths(headers, rows, availableWidth)
	if err := writeRow(out, headers, widths); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writeRow(out, row, widths); err != nil {
			return err
		}
	}
	return nil
}

func calculateWidths(headers []string, rows [][]string, available int) []int {
	widths := make([]int, len(headers))
	minimums := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = min(MaxColumnWidth, max(1, runewidth.StringWidth(singleLine(header))))
		minimums[i] = min(widths[i], 3)
	}
	for _, row := range rows {
		for i := 0; i < len(headers) && i < len(row); i++ {
			widths[i] = min(MaxColumnWidth, max(widths[i], runewidth.StringWidth(singleLine(row[i]))))
		}
	}
	for tableWidth(widths) > available {
		widest := -1
		for i := range widths {
			if widths[i] > minimums[i] && (widest < 0 || widths[i] > widths[widest]) {
				widest = i
			}
		}
		if widest < 0 {
			break
		}
		widths[widest]--
	}
	return widths
}

func tableWidth(widths []int) int {
	return 2*max(0, len(widths)-1) + sum(widths)
}

func sum(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}

func writeRow(out io.Writer, values []string, widths []int) error {
	var line strings.Builder
	for i, width := range widths {
		if i > 0 {
			line.WriteString("  ")
		}
		value := ""
		if i < len(values) {
			value = singleLine(values[i])
		}
		value = truncate(value, width)
		line.WriteString(value)
		if i < len(widths)-1 {
			line.WriteString(strings.Repeat(" ", max(0, width-runewidth.StringWidth(value))))
		}
	}
	line.WriteByte('\n')
	_, err := io.WriteString(out, line.String())
	return err
}

func truncate(value string, width int) string {
	if width <= 0 || runewidth.StringWidth(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	return runewidth.Truncate(value, width-1, "") + "…"
}
