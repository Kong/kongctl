package patch

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"

	"github.com/kong/go-apiops/filebasics"
	"github.com/kong/go-apiops/jsonbasics"
	"github.com/kong/go-apiops/patch"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/i18n"
	"github.com/kong/kongctl/internal/util/normalizers"
)

var (
	fileUse = "file <input-file> [patch-files...]"

	fileShort = i18n.T("root.verbs.patch.file.fileShort",
		"Apply patches to a YAML or JSON file")

	fileLong = normalizers.LongDesc(i18n.T("root.verbs.patch.file.fileLong",
		`Apply patches to a YAML or JSON file using JSONPath selectors to target
specific nodes. Patches can be specified either inline via --selector and
--value flags, or loaded from one or more patch files passed as positional
arguments after the input file.

Values use the format 'key:json-value' to set fields, 'key:' (empty value)
to remove fields, or '[val1,val2]' to append to arrays. String values must
be double-quoted within the JSON value portion.

Patch files optionally support a '_format_version' field for compatibility
with existing deck patch files, but it is not required.`))

	fileExamples = normalizers.Examples(i18n.T("root.verbs.patch.file.fileExamples",
		fmt.Sprintf(`
        # Set timeouts on all services
        %[1]s patch file kong.yaml -s '$..services[*]' -v 'read_timeout:30000'

        # Set multiple values
        %[1]s patch file kong.yaml -s '$..services[*]' -v 'read_timeout:30000' -v 'write_timeout:30000'

        # Remove a key from the root object
        %[1]s patch file config.yaml -s '$' -v 'debug:'

        # Append to an array
        %[1]s patch file kong.yaml -s '$..routes[*].methods' -v '["OPTIONS"]'

        # Apply a patch file
        %[1]s patch file kong.yaml patches.yaml

        # Apply multiple patch files in order
        %[1]s patch file kong.yaml base.yaml env.yaml team.yaml

        # Read from stdin, write to a file
        cat kong.yaml | %[1]s patch file - -s '$' -v 'version:"2.0"' --output-file output.yaml

        # Output as JSON
        %[1]s patch file kong.yaml patches.yaml --format json --output-file output.json
        `, meta.CLIName)))
)

func newFileCmd() *cobra.Command {
	var (
		selectors  []string
		values     []string
		outputFile string
		format     string
	)

	fileCmd := &cobra.Command{
		Use:     fileUse,
		Short:   fileShort,
		Long:    fileLong,
		Example: fileExamples,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runFilePatch(args, selectors, values, outputFile, format)
		},
	}

	fileCmd.Flags().StringArrayVarP(&selectors, "selector", "s", nil,
		"JSONPath expression to select target nodes (repeatable)")
	fileCmd.Flags().StringArrayVarP(&values, "value", "v", nil,
		`Value to set: "key:json-value", "key:" (remove), or "[values]" (append). Repeatable.`)
	fileCmd.Flags().StringVar(&outputFile, "output-file", "-",
		"Output file path (default: stdout)")
	fileCmd.Flags().StringVar(&format, "format", "yaml",
		"Output format: yaml or json")

	fileCmd.MarkFlagsRequiredTogether("selector", "value")

	return fileCmd
}

func runFilePatch(
	args []string,
	selectors []string,
	values []string,
	outputFile string,
	format string,
) error {
	inputFile := args[0]
	patchFiles := args[1:]

	// Validate mutual exclusivity: inline flags vs patch files
	if len(selectors) > 0 && len(patchFiles) > 0 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("cannot combine --selector/--value flags with patch file arguments"),
		}
	}
	if len(selectors) == 0 && len(patchFiles) == 0 {
		return &cmd.ConfigurationError{
			Err: fmt.Errorf("provide either patch file arguments or --selector/--value flags"),
		}
	}

	// Validate output format
	outputFmt, err := parseOutputFormat(format)
	if err != nil {
		return &cmd.ConfigurationError{Err: err}
	}

	// Read and deserialize input file
	data, err := filebasics.DeserializeFile(inputFile)
	if err != nil {
		return &cmd.ExecutionError{
			Err: fmt.Errorf("failed to read input file: %w", err),
		}
	}

	// Convert to YAML node tree for JSONPath operations.
	// ConvertToYamlNode panics on error, so we recover.
	yamlNode, err := safeConvertToYamlNode(data)
	if err != nil {
		return &cmd.ExecutionError{
			Err: fmt.Errorf("failed to convert input to YAML node: %w", err),
		}
	}

	// Apply patches
	if len(patchFiles) > 0 {
		err = applyPatchFiles(patchFiles, yamlNode)
	} else {
		err = applyInlinePatches(selectors, values, yamlNode)
	}
	if err != nil {
		return &cmd.ExecutionError{Err: err}
	}

	// Convert back to map and write output.
	// ConvertToJSONobject panics on error, so we recover.
	result, err := safeConvertToJSONObject(yamlNode)
	if err != nil {
		return &cmd.ExecutionError{
			Err: fmt.Errorf("failed to convert patched result: %w", err),
		}
	}

	if err := filebasics.WriteSerializedFile(outputFile, result, outputFmt); err != nil {
		return &cmd.ExecutionError{
			Err: fmt.Errorf("failed to write output: %w", err),
		}
	}

	return nil
}

func applyPatchFiles(patchFiles []string, yamlNode *yaml.Node) error {
	for _, filename := range patchFiles {
		var pf patch.DeckPatchFile
		if err := pf.ParseFile(filename); err != nil {
			return fmt.Errorf("failed to parse patch file %q: %w", filename, err)
		}
		if err := pf.Apply(yamlNode); err != nil {
			return fmt.Errorf("failed to apply patch file %q: %w", filename, err)
		}
	}
	return nil
}

func applyInlinePatches(selectors, values []string, yamlNode *yaml.Node) error {
	objValues, removeArr, appendArr, err := patch.ValidateValuesFlags(values)
	if err != nil {
		return fmt.Errorf("invalid --value flag: %w", err)
	}

	p := patch.DeckPatch{
		SelectorSources: selectors,
		ObjValues:       objValues,
		ArrValues:       appendArr,
		Remove:          removeArr,
	}

	if err := p.ApplyToNodes(yamlNode); err != nil {
		return fmt.Errorf("failed to apply patch: %w", err)
	}

	return nil
}

func parseOutputFormat(format string) (filebasics.OutputFormat, error) {
	switch strings.ToLower(format) {
	case "yaml":
		return filebasics.OutputFormatYaml, nil
	case "json":
		return filebasics.OutputFormatJSON, nil
	default:
		return "", fmt.Errorf("unsupported output format %q: must be 'yaml' or 'json'", format)
	}
}

// safeConvertToYamlNode wraps jsonbasics.ConvertToYamlNode with panic recovery
// since the upstream function panics on errors instead of returning them.
func safeConvertToYamlNode(data interface{}) (result *yaml.Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	return jsonbasics.ConvertToYamlNode(data), nil
}

// safeConvertToJSONObject wraps jsonbasics.ConvertToJSONobject with panic recovery
// since the upstream function panics on errors instead of returning them.
func safeConvertToJSONObject(data *yaml.Node) (result map[string]interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	return jsonbasics.ConvertToJSONobject(data), nil
}
