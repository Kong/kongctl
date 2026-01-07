package api

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	"github.com/kong/kongctl/internal/cmd/root/verbs"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
	"github.com/segmentio/cli"
	"github.com/spf13/cobra"
)

const (
	attributesCommandName = "attributes"
)

type apiAttributeRecord struct {
	Key        string
	Values     string
	ValueCount int
}

var (
	attributesUse = attributesCommandName

	attributesShort = i18n.T("root.products.konnect.api.attributesShort",
		"Inspect API attributes for a Konnect API")
	attributesLong = normalizers.LongDesc(i18n.T("root.products.konnect.api.attributesLong",
		`Use the attributes command to list API attributes for a specific Konnect API.`))
	attributesExample = normalizers.Examples(
		i18n.T("root.products.konnect.api.attributesExamples",
			fmt.Sprintf(`
# List attributes for an API by ID
%[1]s get api attributes --api-id <api-id>
# List attributes for an API by name
%[1]s get api attributes --api-name my-api
# Get a specific attribute key
%[1]s get api attributes --api-id <api-id> category
`, meta.CLIName)))
)

func newGetAPIAttributesCmd(
	verb verbs.VerbValue,
	addParentFlags func(verbs.VerbValue, *cobra.Command),
	parentPreRun func(*cobra.Command, []string) error,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     attributesUse,
		Short:   attributesShort,
		Long:    attributesLong,
		Example: attributesExample,
		Aliases: []string{"attribute", "attrs", "attr"},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if parentPreRun != nil {
				if err := parentPreRun(cmd, args); err != nil {
					return err
				}
			}
			return bindAPIChildFlags(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			handler := apiAttributesHandler{cmd: cmd}
			return handler.run(args)
		},
	}

	addAPIChildFlags(cmd)

	if addParentFlags != nil {
		addParentFlags(verb, cmd)
	}

	return cmd
}

type apiAttributesHandler struct {
	cmd *cobra.Command
}

func (h apiAttributesHandler) run(args []string) error {
	helper := cmd.BuildHelper(h.cmd, args)

	if len(args) > 1 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("too many arguments. Listing API attributes requires 0 or 1 arguments (attribute key)"),
		}
	}

	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	logger, err := helper.GetLogger()
	if err != nil {
		return err
	}

	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	sdk, err := helper.GetKonnectSDK(cfg, logger)
	if err != nil {
		return err
	}

	apiID, apiName := getAPIIdentifiers(cfg)
	if apiID != "" && apiName != "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("only one of --%s or --%s can be provided", apiIDFlagName, apiNameFlagName),
		}
	}

	if apiID == "" && apiName == "" {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("an API identifier is required. Provide --%s or --%s", apiIDFlagName, apiNameFlagName),
		}
	}

	apiClient := sdk.GetAPIAPI()
	if apiClient == nil {
		return &cmd.ExecutionError{
			Msg: "API client is not available",
			Err: fmt.Errorf("api client not configured"),
		}
	}

	if apiID == "" {
		apiID, err = resolveAPIIDByName(apiName, apiClient, helper, cfg)
		if err != nil {
			return err
		}
	}

	api, err := runGet(apiID, apiClient, helper)
	if err != nil {
		return err
	}

	normalized, err := normalizeAttributes(api.GetAttributes())
	if err != nil {
		return &cmd.ExecutionError{
			Msg: "Failed to parse API attributes",
			Err: err,
		}
	}

	if len(args) == 1 {
		key := strings.TrimSpace(args[0])
		values, ok := normalized[key]
		if !ok {
			return &cmd.ConfigurationError{
				Err: fmt.Errorf("attribute %q not found", key),
			}
		}
		normalized = map[string][]string{key: values}
	}

	keys := make([]string, 0, len(normalized))
	for key := range normalized {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	records := make([]apiAttributeRecord, 0, len(keys))
	rows := make([]table.Row, 0, len(keys))
	valueLookup := make(map[string][]string, len(keys))
	for _, key := range keys {
		values := append([]string(nil), normalized[key]...)
		sort.Strings(values)
		record := apiAttributeRecord{
			Key:        key,
			Values:     strings.Join(values, ", "),
			ValueCount: len(values),
		}
		records = append(records, record)
		rows = append(rows, table.Row{key, fmt.Sprintf("%d", record.ValueCount)})
		valueLookup[key] = values
	}

	detailFn := func(index int) string {
		if index < 0 || index >= len(keys) {
			return ""
		}
		key := keys[index]
		values := append([]string(nil), valueLookup[key]...)
		sort.Strings(values)
		return attributeDetailView(key, values)
	}

	return tableview.RenderForFormat(
		false,
		outType,
		printer,
		helper.GetStreams(),
		records,
		normalized,
		"",
		tableview.WithCustomTable([]string{"KEY", "VALUE COUNT"}, rows),
		tableview.WithDetailRenderer(detailFn),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func normalizeAttributes(attributes any) (map[string][]string, error) {
	result := make(map[string][]string)
	if attributes == nil {
		return result, nil
	}

	switch value := attributes.(type) {
	case map[string]any:
		for key, raw := range value {
			result[key] = coerceToStrings(raw)
		}
	case map[string][]any:
		for key, raw := range value {
			converted := make([]string, 0, len(raw))
			for _, item := range raw {
				converted = append(converted, fmt.Sprint(item))
			}
			result[key] = converted
		}
	case map[string][]string:
		for key, raw := range value {
			result[key] = append([]string(nil), raw...)
		}
	case map[string]string:
		for key, raw := range value {
			result[key] = []string{raw}
		}
	default:
		// Try to marshal into a generic map for robustness.
		bytes, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("unsupported attributes type %T", attributes)
		}
		var generic map[string]any
		if err := json.Unmarshal(bytes, &generic); err != nil {
			return nil, fmt.Errorf("unsupported attributes structure: %w", err)
		}
		for key, raw := range generic {
			result[key] = coerceToStrings(raw)
		}
	}

	return result, nil
}

func attributeDetailView(key string, values []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Key: %s\n", key)
	fmt.Fprintf(&b, "Value Count: %d\n", len(values))

	if len(values) > 0 {
		fmt.Fprintf(&b, "\nValues:\n")
		for _, v := range values {
			fmt.Fprintf(&b, "  - %s\n", v)
		}
	}

	return b.String()
}

func coerceToStrings(value any) []string {
	switch v := value.(type) {
	case nil:
		return nil
	case []string:
		return append([]string(nil), v...)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, fmt.Sprint(item))
		}
		return out
	case string:
		return []string{v}
	case fmt.Stringer:
		return []string{v.String()}
	default:
		return []string{fmt.Sprint(v)}
	}
}
