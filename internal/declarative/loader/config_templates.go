package loader

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/kong/kongctl/internal/declarative/tags"
	"gopkg.in/yaml.v3" //nolint:gomodguard_v2 // yaml.v3 is required to preserve source locations while expanding templates
)

const (
	templatesKey = "_templates"
	extendsKey   = "_extends"
)

type configTemplate struct {
	sourcePath string
	node       *yaml.Node
}

type configTemplateRegistry struct {
	definitions  map[string]configTemplate
	resolved     map[string]*yaml.Node
	resolvedUses map[string]map[string]configTemplate
	resolving    map[string]int
	stack        []string
	currentUses  map[string]configTemplate
}

type configTemplateDocument struct {
	content       []byte
	sourcePath    string
	document      yaml.Node
	usedTemplates map[string]configTemplate
}

func newConfigTemplateRegistry() *configTemplateRegistry {
	return &configTemplateRegistry{
		definitions:  make(map[string]configTemplate),
		resolved:     make(map[string]*yaml.Node),
		resolvedUses: make(map[string]map[string]configTemplate),
		resolving:    make(map[string]int),
	}
}

func expandConfigTemplateDocuments(documents []*configTemplateDocument) error {
	registry := newConfigTemplateRegistry()
	for _, document := range documents {
		if err := yaml.Unmarshal(document.content, &document.document); err != nil {
			return fmt.Errorf("failed to parse YAML for template expansion in %s: %w", document.sourcePath, err)
		}
		if err := registry.collect(&document.document, document.sourcePath); err != nil {
			return err
		}
		document.document = yaml.Node{}
	}
	if err := registry.resolveDefinitions(); err != nil {
		return err
	}

	for _, document := range documents {
		if err := yaml.Unmarshal(document.content, &document.document); err != nil {
			return fmt.Errorf("failed to parse YAML for template expansion in %s: %w", document.sourcePath, err)
		}
		if err := stripTemplateDefinitions(&document.document, document.sourcePath); err != nil {
			return err
		}
		if isEmptyYAMLDocument(&document.document) {
			document.document = yaml.Node{}
			continue
		}
		document.usedTemplates = make(map[string]configTemplate)
		registry.currentUses = document.usedTemplates
		if err := registry.expandDocument(&document.document, document.sourcePath); err != nil {
			return err
		}
		registry.currentUses = nil

		var result bytes.Buffer
		encoder := yaml.NewEncoder(&result)
		encoder.SetIndent(2)
		if err := encoder.Encode(&document.document); err != nil {
			return fmt.Errorf("failed to encode expanded YAML in %s: %w", document.sourcePath, err)
		}
		document.content = result.Bytes()
		document.document = yaml.Node{}
	}
	return nil
}

func (r *configTemplateRegistry) resolveDefinitions() error {
	names := slices.Sorted(maps.Keys(r.definitions))

	for _, name := range names {
		definition := r.definitions[name]
		if _, err := r.resolve(
			name,
			definition.sourcePath,
			[]string{templatesKey, name},
			definition.node,
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *configTemplateRegistry) collect(document *yaml.Node, sourcePath string) error {
	root, err := documentMapping(document, sourcePath)
	if err != nil {
		return err
	}
	if root == nil {
		return nil
	}

	var registryKey *yaml.Node
	for i := 0; i+1 < len(root.Content); {
		if root.Content[i].Value != templatesKey {
			i += 2
			continue
		}
		if registryKey != nil {
			return nodeError(
				root.Content[i],
				sourcePath,
				"duplicate %s key; first declared at line %d, column %d",
				templatesKey,
				registryKey.Line,
				registryKey.Column,
			)
		}
		registryKey = root.Content[i]

		templates := root.Content[i+1]
		if templates.Kind != yaml.MappingNode {
			return nodeError(templates, sourcePath, "%s must be a configuration block", templatesKey)
		}
		for j := 0; j+1 < len(templates.Content); j += 2 {
			nameNode := templates.Content[j]
			valueNode := templates.Content[j+1]
			name := strings.TrimSpace(nameNode.Value)
			if nameNode.Kind != yaml.ScalarNode || nameNode.Tag != "!!str" || name == "" {
				return nodeError(nameNode, sourcePath, "template names must be non-empty strings")
			}
			if valueNode.Kind != yaml.MappingNode {
				return nodeError(valueNode, sourcePath, "template %q must be a configuration block", name)
			}
			if previous, ok := r.definitions[name]; ok {
				return nodeError(
					nameNode,
					sourcePath,
					"duplicate template %q; first defined in %s at line %d, column %d",
					name,
					previous.sourcePath,
					previous.node.Line,
					previous.node.Column,
				)
			}
			r.definitions[name] = configTemplate{sourcePath: sourcePath, node: cloneYAMLNode(valueNode)}
		}

		root.Content = append(root.Content[:i], root.Content[i+2:]...)
	}
	return nil
}

func stripTemplateDefinitions(document *yaml.Node, sourcePath string) error {
	root, err := documentMapping(document, sourcePath)
	if err != nil || root == nil {
		return err
	}
	if index, _, ok := mappingValue(root, templatesKey); ok {
		root.Content = append(root.Content[:index], root.Content[index+2:]...)
	}
	return nil
}

func (r *configTemplateRegistry) expandDocument(document *yaml.Node, sourcePath string) error {
	root, err := documentMapping(document, sourcePath)
	if err != nil {
		return err
	}
	if root == nil {
		return nil
	}
	if _, _, ok := mappingValue(root, extendsKey); ok {
		return nodeError(root, sourcePath, "%s is not supported at the document root", extendsKey)
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i].Value
		if key == "_defaults" {
			if extends, path, ok := findNestedMappingValue(root.Content[i+1], extendsKey, []string{key}); ok {
				return nodeError(
					extends,
					sourcePath,
					"%s is not supported inside _defaults at %s",
					extendsKey,
					declarativeTemplatePath(path),
				)
			}
			continue
		}
		if err := r.expandNode(root.Content[i+1], sourcePath, []string{key}); err != nil {
			return err
		}
	}
	return nil
}

func (r *configTemplateRegistry) expandNode(node *yaml.Node, sourcePath string, path []string) error {
	if node == nil {
		return nil
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if err := r.expandNode(child, sourcePath, path); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			if err := r.expandNode(child, sourcePath, append(path, fmt.Sprintf("[%d]", i))); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		if _, nestedTemplates, ok := mappingValue(node, templatesKey); ok {
			return nodeError(
				nestedTemplates,
				sourcePath,
				"%s is only supported at the document root, not at %s",
				templatesKey,
				declarativeTemplatePath(path),
			)
		}
		if keyIndex, extends, ok := mappingValue(node, extendsKey); ok {
			if extends.Kind != yaml.ScalarNode || extends.Tag != "!!str" ||
				strings.TrimSpace(extends.Value) == "" || tags.IsEnvPlaceholder(extends.Value) {
				return nodeError(
					extends,
					sourcePath,
					"%s at %s must name one template",
					extendsKey,
					declarativeTemplatePath(path),
				)
			}
			base, err := r.resolve(strings.TrimSpace(extends.Value), sourcePath, path, extends)
			if err != nil {
				return err
			}
			local := cloneYAMLNode(node)
			local.Content = append(local.Content[:keyIndex], local.Content[keyIndex+2:]...)
			merged, err := mergeTemplateMappings(base, local)
			if err != nil {
				return nodeError(node, sourcePath, "failed to merge template at %s: %v", declarativeTemplatePath(path), err)
			}
			*node = *merged
		}

		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			if err := r.expandNode(node.Content[i+1], sourcePath, append(path, key)); err != nil {
				return err
			}
		}
	case yaml.ScalarNode, yaml.AliasNode:
		return nil
	}
	return nil
}

func findNestedMappingValue(node *yaml.Node, key string, path []string) (*yaml.Node, []string, bool) {
	if node == nil {
		return nil, nil, false
	}
	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if value, valuePath, ok := findNestedMappingValue(child, key, path); ok {
				return value, valuePath, true
			}
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			if value, valuePath, ok := findNestedMappingValue(
				child,
				key,
				append(path, fmt.Sprintf("[%d]", i)),
			); ok {
				return value, valuePath, true
			}
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			childKey := node.Content[i].Value
			childPath := append(path, childKey)
			if childKey == key {
				return node.Content[i+1], childPath, true
			}
			if value, valuePath, ok := findNestedMappingValue(node.Content[i+1], key, childPath); ok {
				return value, valuePath, true
			}
		}
	case yaml.ScalarNode, yaml.AliasNode:
		return nil, nil, false
	}
	return nil, nil, false
}

func (r *configTemplateRegistry) resolve(
	name string,
	consumerSource string,
	consumerPath []string,
	reference *yaml.Node,
) (*yaml.Node, error) {
	if resolved, ok := r.resolved[name]; ok {
		r.recordTemplateUses(name)
		return cloneYAMLNode(resolved), nil
	}
	definition, ok := r.definitions[name]
	if !ok {
		return nil, nodeError(
			reference,
			consumerSource,
			"unknown template %q referenced at %s",
			name,
			declarativeTemplatePath(consumerPath),
		)
	}
	if start, ok := r.resolving[name]; ok {
		cycle := append(append([]string(nil), r.stack[start:]...), name)
		return nil, nodeError(
			reference,
			consumerSource,
			"template inheritance cycle detected: %s",
			strings.Join(cycle, " -> "),
		)
	}

	r.resolving[name] = len(r.stack)
	r.stack = append(r.stack, name)
	consumerUses := r.currentUses
	definitionUses := make(map[string]configTemplate)
	r.currentUses = definitionUses
	resolved := cloneYAMLNode(definition.node)
	err := r.expandNode(resolved, definition.sourcePath, []string{templatesKey, name})
	r.currentUses = consumerUses
	r.stack = r.stack[:len(r.stack)-1]
	delete(r.resolving, name)
	if err != nil {
		return nil, err
	}
	r.resolved[name] = resolved
	r.resolvedUses[name] = definitionUses
	r.recordTemplateUses(name)
	return cloneYAMLNode(resolved), nil
}

func (r *configTemplateRegistry) recordTemplateUses(name string) {
	if r.currentUses == nil {
		return
	}
	if definition, ok := r.definitions[name]; ok {
		r.currentUses[name] = definition
	}
	maps.Copy(r.currentUses, r.resolvedUses[name])
}

func mergeTemplateMappings(base, local *yaml.Node) (*yaml.Node, error) {
	if base.Kind != yaml.MappingNode || local.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("template and consumer values must be configuration blocks")
	}

	result := cloneYAMLNode(base)
	for i := 0; i+1 < len(local.Content); i += 2 {
		localKey := local.Content[i]
		localValue := local.Content[i+1]
		baseIndex, baseValue, ok := mappingValue(result, localKey.Value)
		if ok && baseValue.Kind == yaml.MappingNode && localValue.Kind == yaml.MappingNode {
			merged, err := mergeTemplateMappings(baseValue, localValue)
			if err != nil {
				return nil, err
			}
			result.Content[baseIndex] = cloneYAMLNode(localKey)
			result.Content[baseIndex+1] = merged
			continue
		}
		if ok {
			result.Content[baseIndex] = cloneYAMLNode(localKey)
			result.Content[baseIndex+1] = cloneYAMLNode(localValue)
			continue
		}
		result.Content = append(result.Content, cloneYAMLNode(localKey), cloneYAMLNode(localValue))
	}
	return result, nil
}

func documentMapping(document *yaml.Node, sourcePath string) (*yaml.Node, error) {
	if isEmptyYAMLDocument(document) {
		return nil, nil
	}
	if document == nil || document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return nil, fmt.Errorf("declarative document in %s must contain one YAML document", sourcePath)
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, nodeError(root, sourcePath, "declarative document must be a mapping")
	}
	return root, nil
}

func isEmptyYAMLDocument(document *yaml.Node) bool {
	if document == nil || document.Kind == 0 || len(document.Content) == 0 {
		return true
	}
	return document.Kind == yaml.DocumentNode && len(document.Content) == 1 &&
		document.Content[0].Kind == yaml.ScalarNode && document.Content[0].Tag == "!!null"
}

func mappingValue(mapping *yaml.Node, key string) (int, *yaml.Node, bool) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return 0, nil, false
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return i, mapping.Content[i+1], true
		}
	}
	return 0, nil, false
}

func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	clone := *node
	clone.Content = make([]*yaml.Node, len(node.Content))
	for i, child := range node.Content {
		clone.Content[i] = cloneYAMLNode(child)
	}
	return &clone
}

func nodeError(node *yaml.Node, sourcePath, format string, args ...any) error {
	message := fmt.Sprintf(format, args...)
	if node != nil && node.Line > 0 {
		return fmt.Errorf("%s in %s at line %d, column %d", message, sourcePath, node.Line, node.Column)
	}
	return fmt.Errorf("%s in %s", message, sourcePath)
}

func declarativeTemplatePath(path []string) string {
	if len(path) == 0 {
		return "<document>"
	}
	var result strings.Builder
	for i, part := range path {
		if strings.HasPrefix(part, "[") {
			result.WriteString(part)
			continue
		}
		if i > 0 {
			result.WriteByte('.')
		}
		result.WriteString(part)
	}
	return result.String()
}

func templateDefinitionContext(definitions map[string]configTemplate) string {
	if len(definitions) == 0 {
		return ""
	}
	names := slices.Sorted(maps.Keys(definitions))

	parts := make([]string, 0, len(names))
	for _, name := range names {
		definition := definitions[name]
		parts = append(parts, fmt.Sprintf(
			"template %q defined in %s at line %d, column %d",
			name,
			definition.sourcePath,
			definition.node.Line,
			definition.node.Column,
		))
	}
	return strings.Join(parts, "; ")
}
