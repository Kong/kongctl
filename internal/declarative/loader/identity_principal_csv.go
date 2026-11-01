package loader

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/util"
	"gopkg.in/yaml.v3" //nolint:gomodguard_v2 // Custom tags require YAML nodes.
)

type principalCSVLocation struct{ line, column int }

type principalCSVResolver struct {
	files   *tags.FileTagResolver
	parents map[principalCSVLocation]string
}

// PrincipalCSVSummary contains structural counts only; it does not establish
// remote membership, management authority or runtime credential validity.
type PrincipalCSVSummary struct {
	Rows               int    `json:"rows" yaml:"rows"`
	Identities         int    `json:"identities" yaml:"identities"`
	APIKeys            int    `json:"api_keys" yaml:"api_keys"`
	BasicAuths         int    `json:"basic_auths" yaml:"basic_auths"`
	MetadataMode       string `json:"metadata_mode" yaml:"metadata_mode"`
	RemoteStateChecked bool   `json:"remote_state_checked" yaml:"remote_state_checked"`
}

// InspectPrincipalCSV validates a bounded local CSV without contacting Konnect.
func InspectPrincipalCSV(path, mode string) (*PrincipalCSVSummary, error) {
	if path == "" || mode != resources.IdentityMetadataModeMerge && mode != resources.IdentityMetadataModeSync {
		return nil, fmt.Errorf("CSV inspection requires a file and explicit metadata mode: merge or sync")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	files := tags.NewFileTagResolver(filepath.Dir(absolute), filepath.Dir(absolute))
	data, err := files.ReadLocalFile(filepath.Base(absolute))
	if err != nil {
		return nil, err
	}
	rows, err := parsePrincipalCSV(data, mode)
	if err != nil {
		return nil, err
	}
	result := &PrincipalCSVSummary{Rows: len(rows), MetadataMode: mode}
	for _, row := range rows {
		if values, ok := row["identities"].([]any); ok {
			result.Identities += len(values)
		}
		if values, ok := row["api_keys"].([]any); ok {
			result.APIKeys += len(values)
		}
		if values, ok := row["basic_auths"].([]any); ok {
			result.BasicAuths += len(values)
		}
	}
	return result, nil
}

func (r *principalCSVResolver) Tag() string { return tags.TagCSV }

func (r *principalCSVResolver) Resolve(node *yaml.Node) (any, error) {
	if node.Kind != yaml.MappingNode || len(node.Content)%2 != 0 {
		return nil, fmt.Errorf("principal !csv requires a mapping with path and metadata_mode")
	}
	options := make(map[string]string)
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if key.Kind != yaml.ScalarNode || value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
			return nil, fmt.Errorf("principal !csv options must be literal strings")
		}
		if key.Value != "path" && key.Value != "metadata_mode" {
			return nil, fmt.Errorf("principal !csv accepts only path and metadata_mode")
		}
		if _, duplicate := options[key.Value]; duplicate {
			return nil, fmt.Errorf("principal !csv options cannot be repeated")
		}
		options[key.Value] = value.Value
	}
	mode, path := options["metadata_mode"], options["path"]
	if path == "" || mode != resources.IdentityMetadataModeMerge && mode != resources.IdentityMetadataModeSync {
		return nil, fmt.Errorf("principal !csv requires path and explicit metadata_mode: merge or sync")
	}
	data, err := r.files.ReadLocalFile(path)
	if err != nil {
		return nil, err
	}
	rows, err := parsePrincipalCSV(data, mode)
	if err != nil {
		return nil, err
	}
	if parent := r.parents[principalCSVLocation{node.Line, node.Column}]; parent != "" {
		for index, row := range rows {
			if directory, _ := row["directory"].(string); directory != "" && directory != parent {
				return nil, csvRowError(index+2, "directory", "selector conflicts with the enclosing directory")
			}
		}
	}
	encoded, err := json.Marshal(rows)
	if err != nil {
		return nil, fmt.Errorf("cannot encode principal CSV declarations")
	}
	var output yaml.Node
	if err := yaml.Unmarshal(encoded, &output); err != nil {
		return nil, fmt.Errorf("cannot encode principal CSV declarations")
	}
	protectCSVLiterals(&output)
	return output.Content[0], nil
}

func protectCSVLiterals(node *yaml.Node) {
	if node.Kind == yaml.ScalarNode && node.Tag == "!!str" {
		node.Value = tags.ProtectReferenceLiteral(node.Value)
	}
	if node.Kind == yaml.MappingNode {
		for i := 1; i < len(node.Content); i += 2 {
			protectCSVLiterals(node.Content[i])
		}
		return
	}
	for _, child := range node.Content {
		protectCSVLiterals(child)
	}
}

func principalCSVLocations(contents []byte) (map[principalCSVLocation]string, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(contents, &document); err != nil {
		return nil, err
	}
	locations := make(map[principalCSVLocation]string)
	var visit func(*yaml.Node, []string, string) error
	visit = func(node *yaml.Node, path []string, directoryRef string) error {
		if len(path) >= 3 && path[len(path)-3] == "identity" && path[len(path)-2] == "directories" &&
			node.Kind == yaml.MappingNode {
			for i := 0; i < len(node.Content); i += 2 {
				if node.Content[i].Value == "ref" {
					directoryRef = node.Content[i+1].Value
				}
			}
		}
		if node.Tag == tags.TagCSV {
			direct := len(path) >= 2 && path[len(path)-2] == "identity" && path[len(path)-1] == "principals"
			nested := len(path) >= 4 && path[len(path)-4] == "identity" && path[len(path)-3] == "directories" &&
				path[len(path)-1] == "principals"
			if !direct && !nested {
				return fmt.Errorf("!csv is supported only on an identity principal collection")
			}
			if direct {
				directoryRef = ""
			}
			locations[principalCSVLocation{node.Line, node.Column}] = directoryRef
			return nil
		}
		if node.Kind == yaml.MappingNode {
			for i := 0; i < len(node.Content); i += 2 {
				if err := visit(node.Content[i+1], append(path, node.Content[i].Value), directoryRef); err != nil {
					return err
				}
			}
		} else {
			for _, child := range node.Content {
				childPath := path
				if node.Kind == yaml.SequenceNode {
					childPath = append(path, "[]")
				}
				if err := visit(child, childPath, directoryRef); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(&document, nil, ""); err != nil {
		return nil, err
	}
	return locations, nil
}

func csvRowError(row int, column, reason string) error {
	return fmt.Errorf("principal CSV row %d, column %s: %s (input values withheld)", row, column, reason)
}

func parsePrincipalCSV(contents []byte, mode string) ([]map[string]any, error) {
	contents = bytes.TrimPrefix(contents, []byte{0xef, 0xbb, 0xbf})
	if !utf8.Valid(contents) || bytes.IndexByte(contents, 0) >= 0 {
		return nil, fmt.Errorf("principal CSV must be UTF-8 without NUL characters")
	}
	reader := csv.NewReader(bytes.NewReader(contents))
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("principal CSV requires a valid header row")
	}
	allowed := map[string]bool{
		"ref": true, "id": true, "directory": true, "display_name": true, "description": true, "type": true, "active": true,
		"labels": true, "metadata": true, "kongctl": true, "identities": true, "api_keys": true, "basic_auths": true,
	}
	seen := make(map[string]bool)
	for index, header := range headers {
		if !allowed[header] || seen[header] {
			return nil, fmt.Errorf(
				"principal CSV header column %d is unknown or repeated; input values withheld",
				index+1,
			)
		}
		seen[header] = true
	}
	if !seen["ref"] {
		return nil, fmt.Errorf("principal CSV requires the ref header")
	}
	rows := []map[string]any{}
	refs, ids := make(map[string]bool), make(map[string]bool)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, csvRowError(len(rows)+2, "record", "invalid CSV syntax or field count")
		}
		line := len(rows) + 2
		row := make(map[string]any)
		for i, cell := range record {
			if cell == "" {
				continue
			}
			header := headers[i]
			value, err := principalCSVCell(header, cell)
			if err != nil {
				return nil, csvRowError(line, header, err.Error())
			}
			if err := validateCSVLiteral(value); err != nil {
				return nil, csvRowError(line, header, err.Error())
			}
			row[header] = value
		}
		ref, _ := row["ref"].(string)
		if resources.ValidateRef(ref) != nil {
			return nil, csvRowError(line, "ref", "a valid ref is required")
		}
		if refs[ref] {
			return nil, csvRowError(line, "ref", "duplicate principal ref")
		}
		refs[ref] = true
		if id, _ := row["id"].(string); id != "" {
			canonical := strings.ToLower(id)
			if !util.IsValidUUID(id) || ids[canonical] {
				return nil, csvRowError(line, "id", "UUID must be valid and unique")
			}
			ids[canonical] = true
		}
		if _, present := row["metadata"]; present {
			row["metadata_mode"] = mode
		}
		if err := validateCSVPrincipalRow(row); err != nil {
			return nil, csvRowError(line, "principal", err.Error())
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func validateCSVLiteral(value any) error {
	switch value := value.(type) {
	case string:
		if tags.IsEnvPlaceholder(value) || tags.IsSecretPlaceholder(value) {
			return fmt.Errorf("deferred environment/secret encodings are not supported in CSV cells")
		}
	case map[string]any:
		for key, entry := range value {
			if key == templatesKey || key == extendsKey {
				return fmt.Errorf("template control keys are not supported in CSV cells")
			}
			if err := validateCSVLiteral(entry); err != nil {
				return err
			}
		}
	case []any:
		for _, entry := range value {
			if err := validateCSVLiteral(entry); err != nil {
				return err
			}
		}
	}
	return nil
}

func principalCSVCell(header, cell string) (any, error) {
	switch header {
	case "active":
		if cell == "true" {
			return true, nil
		}
		if cell == "false" {
			return false, nil
		}
		return nil, fmt.Errorf("expected true or false")
	case "type":
		if cell == "null" {
			return nil, nil
		}
		if cell != "user" && cell != "agent" {
			return nil, fmt.Errorf("expected user, agent or null")
		}
		return cell, nil
	case "labels", "metadata", "kongctl", "identities", "api_keys", "basic_auths":
		decoder := json.NewDecoder(strings.NewReader(cell))
		decoder.UseNumber()
		value, err := decodeCSVJSON(decoder, 0)
		if err != nil {
			return nil, fmt.Errorf("expected JSON without duplicate keys or excessive nesting")
		}
		if decoder.Decode(new(any)) != io.EOF {
			return nil, fmt.Errorf("expected one JSON value")
		}
		if header == "identities" || header == "api_keys" || header == "basic_auths" {
			if _, ok := value.([]any); !ok {
				return nil, fmt.Errorf("expected a JSON array")
			}
		} else if _, ok := value.(map[string]any); !ok {
			return nil, fmt.Errorf("expected a JSON object")
		}
		return value, nil
	default:
		return cell, nil
	}
}

func decodeCSVJSON(decoder *json.Decoder, depth int) (any, error) {
	if depth > 32 {
		return nil, fmt.Errorf("JSON nesting limit exceeded")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delimiter, ok := token.(json.Delim); ok {
		switch delimiter {
		case '{':
			value := make(map[string]any)
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				name, ok := key.(string)
				if !ok {
					return nil, fmt.Errorf("JSON object key must be a string")
				}
				if _, duplicate := value[name]; duplicate {
					return nil, fmt.Errorf("duplicate JSON object key")
				}
				entry, err := decodeCSVJSON(decoder, depth+1)
				if err != nil {
					return nil, err
				}
				value[name] = entry
			}
			closing, err := decoder.Token()
			if err != nil || closing != json.Delim('}') {
				return nil, fmt.Errorf("invalid JSON object")
			}
			return value, nil
		case '[':
			value := []any{}
			for decoder.More() {
				entry, err := decodeCSVJSON(decoder, depth+1)
				if err != nil {
					return nil, err
				}
				value = append(value, entry)
			}
			closing, err := decoder.Token()
			if err != nil || closing != json.Delim(']') {
				return nil, fmt.Errorf("invalid JSON array")
			}
			return value, nil
		default:
			return nil, fmt.Errorf("invalid JSON delimiter")
		}
	}
	return token, nil
}

func validateCSVPrincipalRow(row map[string]any) error {
	encoded, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("invalid principal configuration")
	}
	document := map[string]any{"identity": map[string]any{"principals": []any{row}}}
	shape, err := json.Marshal(document)
	if err != nil || validateDeclarativeDocumentShape(shape, "principal CSV") != nil {
		return fmt.Errorf("unsupported principal or nested resource fields")
	}
	var principal resources.IdentityPrincipalResource
	if err := json.Unmarshal(encoded, &principal); err != nil {
		return fmt.Errorf("invalid principal field types")
	}
	if principal.Validate() != nil {
		return fmt.Errorf("invalid principal fields or metadata")
	}
	for _, value := range principal.Identities {
		if value.Principal != "" && value.Principal != principal.Ref {
			return fmt.Errorf("identity principal conflicts with its CSV row")
		}
		if value.Validate() != nil {
			return fmt.Errorf("invalid identities configuration")
		}
	}
	for _, value := range principal.APIKeys {
		if value.Principal != "" && value.Principal != principal.Ref {
			return fmt.Errorf("API-key principal conflicts with its CSV row")
		}
		if value.Validate() != nil {
			return fmt.Errorf("invalid generated api_keys configuration")
		}
	}
	for _, value := range principal.BasicAuths {
		if value.Principal != "" && value.Principal != principal.Ref {
			return fmt.Errorf("basic-auth principal conflicts with its CSV row")
		}
		if value.Validate() != nil {
			return fmt.Errorf("invalid basic_auths configuration")
		}
	}
	return nil
}
