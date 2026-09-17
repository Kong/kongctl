package patch

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v4"

	"github.com/kong/go-apiops/filebasics"
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
with existing deck patch files, but it is not required.

YAML output preserves custom tags without resolving them. Updating children
of a tagged mapping preserves its tag; replacing a field replaces its tag
with the replacement value's type. JSON output is rejected if custom tags
remain after patching. Patch values must not contain custom YAML tags.
Comments, ordering, anchors, and byte-for-byte formatting are not guaranteed.`))

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

func validatePatchInputs(selectors, patchFiles []string) error {
	hasInlineFlags := len(selectors) > 0
	hasPatchFiles := len(patchFiles) > 0

	if hasInlineFlags && hasPatchFiles {
		return fmt.Errorf("cannot combine --selector/--value flags with patch file arguments")
	}
	if !hasInlineFlags && !hasPatchFiles {
		return fmt.Errorf("provide either patch file arguments or --selector/--value flags")
	}
	return nil
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
	if err := validatePatchInputs(selectors, patchFiles); err != nil {
		return &cmd.ConfigurationError{Err: err}
	}

	// Validate output format
	outputFmt, err := parseOutputFormat(format)
	if err != nil {
		return &cmd.ConfigurationError{Err: err}
	}

	yamlNode, err := readPatchInput(inputFile)
	if err != nil {
		return &cmd.ExecutionError{
			Err: fmt.Errorf("failed to read input file: %w", err),
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

	var output []byte
	if outputFmt == filebasics.OutputFormatJSON {
		if err := rejectCustomTags(yamlNode); err != nil {
			return &cmd.ExecutionError{Err: fmt.Errorf("cannot output JSON: %w; use --format yaml", err)}
		}
		var result map[string]any
		if err := yamlNode.Decode(&result); err != nil {
			return &cmd.ExecutionError{Err: fmt.Errorf("failed to convert patched result: %w", err)}
		}
		output, err = filebasics.Serialize(result, outputFmt)
	} else {
		if err := checkPatchAliases(yamlNode, make(map[string]*yaml.Node)); err != nil {
			return &cmd.ExecutionError{Err: fmt.Errorf("cannot output YAML: %w", err)}
		}
		normalizePatchStyles(yamlNode)
		output, err = yaml.Marshal(yamlNode)
	}
	if err != nil {
		return &cmd.ExecutionError{
			Err: fmt.Errorf("failed to serialize patched result: %w", err),
		}
	}

	if err := filebasics.WriteFile(outputFile, output); err != nil {
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

// readPatchInput keeps tags as syntax; it does not use the declarative loader.
func readPatchInput(filename string) (*yaml.Node, error) {
	data, err := filebasics.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected the data to be an object")
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("expected a single YAML document")
	}
	return document.Content[0], nil
}

func rejectCustomTags(node *yaml.Node) error {
	return checkPatchTags(node, make(map[*yaml.Node]bool))
}

// Check anchors in serialization order. A removed or replaced anchor must not
// leave an alias undefined or referring to a different node with the same name.
func checkPatchAliases(node *yaml.Node, anchors map[string]*yaml.Node) error {
	if node.Kind == yaml.AliasNode {
		if target := anchors[node.Value]; target == nil || target != node.Alias {
			return fmt.Errorf("alias %q refers to an anchor that is no longer available", node.Value)
		}
		return nil
	}
	if node.Anchor != "" {
		anchors[node.Anchor] = node
	}
	for _, child := range node.Content {
		if err := checkPatchAliases(child, anchors); err != nil {
			return err
		}
	}
	return nil
}

func checkPatchTags(node *yaml.Node, seen map[*yaml.Node]bool) error {
	if node == nil || seen[node] {
		return nil
	}
	seen[node] = true
	if node.Tag != "" && !strings.HasPrefix(node.ShortTag(), "!!") {
		return fmt.Errorf("custom YAML tag %q cannot be represented", node.Tag)
	}
	for _, child := range node.Content {
		if err := checkPatchTags(child, seen); err != nil {
			return err
		}
	}
	return checkPatchTags(node.Alias, seen)
}

// Patch values come from JSON with flow and quoted styles. Emit ordinary YAML
// while retaining explicit tags; formatting is not part of the patch contract.
func normalizePatchStyles(node *yaml.Node) {
	node.Style &= yaml.TaggedStyle
	for _, child := range node.Content {
		normalizePatchStyles(child)
	}
}
