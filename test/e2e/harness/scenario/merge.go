//go:build e2e

package scenario

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	sprig "github.com/Masterminds/sprig/v3"
	"gopkg.in/yaml.v3"
)

// renderTemplate applies Go template + sprig to the input bytes.
func renderTemplate(data []byte, ctx any) ([]byte, error) {
	t, err := template.New("overlay").Funcs(sprig.FuncMap()).Option("missingkey=error").Parse(string(data))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// mergeYAMLNode merges src into dst (in place) using JSON Merge semantics:
// - Mapping: deep-merge keys
// - Sequence: replace entirely
// - Scalar/others: replace
func mergeYAMLNode(dst, src *yaml.Node) *yaml.Node {
	if dst == nil || src == nil {
		return src
	}
	// If either is not a mapping, sequence or scalar, default to replace.
	switch src.Kind {
	case yaml.MappingNode:
		if dst.Kind != yaml.MappingNode {
			return src
		}
		// Merge by keys (mapping nodes have Content as [k1,v1,k2,v2,...])
		for i := 0; i+1 < len(src.Content); i += 2 {
			sk := src.Content[i]
			sv := src.Content[i+1]
			// find key in dst
			found := false
			for j := 0; j+1 < len(dst.Content); j += 2 {
				dk := dst.Content[j]
				dv := dst.Content[j+1]
				if dk.Value == sk.Value {
					dst.Content[j+1] = mergeYAMLNode(dv, sv)
					found = true
					break
				}
			}
			if !found {
				// append new key/value
				dst.Content = append(dst.Content, cloneNode(sk), cloneNode(sv))
			}
		}
		return dst
	case yaml.SequenceNode:
		// replace entirely
		return src
	default:
		// scalar or others: replace
		return src
	}
}

func cloneNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	c := *n
	c.Content = make([]*yaml.Node, len(n.Content))
	for i := range n.Content {
		c.Content[i] = cloneNode(n.Content[i])
	}
	return &c
}

// applyOverlayFile merges a single overlay file into a destination file (in place).
// If the destination file does not exist, it writes the overlay as-is.
// Templating is applied to the overlay contents before merge.
func applyOverlayFile(dstPath, overlayPath string, tmplCtx any) error {
	ob, err := os.ReadFile(overlayPath)
	if err != nil {
		return err
	}
	ob, err = renderTemplate(ob, tmplCtx)
	if err != nil {
		return fmt.Errorf("template overlay %s: %w", overlayPath, err)
	}
	// If non-YAML extension, just overwrite/copy
	if !isYAML(overlayPath) {
		return os.WriteFile(dstPath, ob, 0o644)
	}
	// Parse overlay YAML
	var on yaml.Node
	if err := yaml.Unmarshal(ob, &on); err != nil {
		return fmt.Errorf("parse overlay yaml %s: %w", overlayPath, err)
	}
	// If dst does not exist, write overlay
	if _, err := os.Stat(dstPath); err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(dstPath, ob, 0o644)
		}
		return err
	}
	// Parse destination YAML and merge
	db, err := os.ReadFile(dstPath)
	if err != nil {
		return err
	}
	var dn yaml.Node
	if err := yaml.Unmarshal(db, &dn); err != nil {
		return fmt.Errorf("parse dst yaml %s: %w", dstPath, err)
	}
	merged := mergeYAMLNode(&dn, &on)
	out, err := yaml.Marshal(merged)
	if err != nil {
		return err
	}
	return os.WriteFile(dstPath, out, 0o644)
}

func isYAML(path string) bool {
	low := strings.ToLower(path)
	return strings.HasSuffix(low, ".yaml") || strings.HasSuffix(low, ".yml")
}

// Overlay Ops support (targeted edits without array replacement)

type opFile struct {
	Ops []op `yaml:"ops"`
}

type op struct {
	File   string         `yaml:"file"`
	Match  string         `yaml:"match"`  // JMESPath-like match (limited subset)
	Path   []pathStep     `yaml:"path"`   // DEPRECATED: ordered path steps
	Set    map[string]any `yaml:"set"`    // merge into matched mapping node
	Remove []string       `yaml:"remove"` // optional future use
}

type pathStep struct {
	Key   string            `yaml:"key"`   // descend into mapping by key
	Where map[string]string `yaml:"where"` // filter sequence of mappings by key=value
}

// ApplyOverlayOps reads an ops file and applies targeted edits to files under dstRoot.
func ApplyOverlayOps(dstRoot, opsPath string, tmplCtx any) error {
	b, err := os.ReadFile(opsPath)
	if err != nil {
		return err
	}
	b, err = renderTemplate(b, tmplCtx)
	if err != nil {
		return fmt.Errorf("template ops %s: %w", opsPath, err)
	}
	var of opFile
	if err := yaml.Unmarshal(b, &of); err != nil {
		return fmt.Errorf("parse ops yaml %s: %w", opsPath, err)
	}
	for _, o := range of.Ops {
		if o.File == "" {
			return fmt.Errorf("ops entry missing file")
		}
		target := filepath.Join(dstRoot, o.File)
		db, err := os.ReadFile(target)
		if err != nil {
			return fmt.Errorf("read target %s: %w", target, err)
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(db, &doc); err != nil {
			return fmt.Errorf("parse target %s: %w", target, err)
		}
		// find matched mapping nodes and apply set
		steps := o.Path
		if o.Match != "" {
			ps, perr := parseJMESMatch(o.Match)
			if perr != nil {
				return fmt.Errorf("parse match '%s': %w", o.Match, perr)
			}
			steps = ps
		}
		matched := matchNodes(&doc, steps)
		if len(matched) == 0 {
			return fmt.Errorf("ops match found no targets in %s for %s", target, opsPath)
		}
		for _, m := range matched {
			if m.Kind != yaml.MappingNode {
				continue
			}
			// apply set keys
			for k, v := range o.Set {
				setMappingKey(m, k, v)
			}
			// TODO: handle remove keys in future
		}
		out, err := yaml.Marshal(&doc)
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, out, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// matchNodes traverses doc by path steps and returns matching mapping nodes.
func matchNodes(doc *yaml.Node, path []pathStep) []*yaml.Node {
	// start at document mapping root
	var cur []*yaml.Node
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		cur = []*yaml.Node{doc.Content[0]}
	} else {
		cur = []*yaml.Node{doc}
	}
	for _, step := range path {
		next := []*yaml.Node{}
		for _, n := range cur {
			if step.Key != "" {
				if n.Kind == yaml.MappingNode {
					if v := findMapKey(n, step.Key); v != nil {
						// If where is also present and v is sequence, filter
						if len(step.Where) > 0 && v.Kind == yaml.SequenceNode {
							next = append(next, filterSeqWhere(v, step.Where)...)
						} else {
							next = append(next, v)
						}
					}
				}
			} else if len(step.Where) > 0 && n.Kind == yaml.SequenceNode {
				next = append(next, filterSeqWhere(n, step.Where)...)
			}
		}
		if len(next) == 0 {
			return nil
		}
		cur = next
	}
	// return only mapping nodes
	out := []*yaml.Node{}
	for _, n := range cur {
		if n.Kind == yaml.MappingNode {
			out = append(out, n)
		}
	}
	return out
}

// findMapKey returns the value node for a given key within a mapping node.
func findMapKey(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i]
		v := m.Content[i+1]
		if k.Value == key {
			return v
		}
	}
	return nil
}

func filterSeqWhere(seq *yaml.Node, where map[string]string) []*yaml.Node {
	out := []*yaml.Node{}
	for _, it := range seq.Content {
		if it.Kind != yaml.MappingNode {
			continue
		}
		if mappingMatches(it, where) {
			out = append(out, it)
		}
	}
	return out
}

func mappingMatches(m *yaml.Node, where map[string]string) bool {
	for wk, wv := range where {
		vn := findMapKey(m, wk)
		if vn == nil {
			return false
		}
		if vn.Kind != yaml.ScalarNode || vn.Value != wv {
			return false
		}
	}
	return true
}

func setMappingKey(m *yaml.Node, key string, val any) {
	// find existing
	for i := 0; i+1 < len(m.Content); i += 2 {
		k := m.Content[i]
		// v := m.Content[i+1]
		if k.Value == key {
			m.Content[i+1] = toYAMLNode(val)
			return
		}
	}
	// append
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, toYAMLNode(val))
}

func toYAMLNode(val any) *yaml.Node {
	switch x := val.(type) {
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: x}
	case bool:
		if x {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"}
	case int, int64, float64, float32:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: fmt.Sprintf("%v", x)}
	case map[string]any:
		n := &yaml.Node{Kind: yaml.MappingNode}
		for k, v := range x {
			n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}, toYAMLNode(v))
		}
		return n
	case []any:
		n := &yaml.Node{Kind: yaml.SequenceNode}
		for _, v := range x {
			n.Content = append(n.Content, toYAMLNode(v))
		}
		return n
	default:
		// fallback via yaml marshal
		var n yaml.Node
		_ = n.Encode(x)
		if n.Kind == 0 {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: fmt.Sprintf("%v", x)}
		}
		return &n
	}
}

// parseJMESMatch parses a limited JMESPath-like expression into path steps.
// Supported patterns (sequential):
//
//	key
//	key[?field=='value']
//
// Chained with dots, optional trailing "| [0]" is ignored.
func parseJMESMatch(expr string) ([]pathStep, error) {
	s := strings.TrimSpace(expr)
	// drop optional pipe first selection
	if i := strings.Index(s, "|"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	parts := strings.Split(s, ".")
	var steps []pathStep
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// look for filter [?...]
		if lb := strings.Index(p, "["); lb >= 0 {
			key := strings.TrimSpace(p[:lb])
			if key != "" {
				steps = append(steps, pathStep{Key: key})
			}
			// parse filter content between [ and ]
			rb := strings.LastIndex(p, "]")
			if rb <= lb {
				return nil, fmt.Errorf("unbalanced filter in %q", p)
			}
			filt := p[lb+1 : rb]
			filt = strings.TrimSpace(filt)
			// expect ?field=='value'
			if !strings.HasPrefix(filt, "?") {
				return nil, fmt.Errorf("unsupported filter %q", filt)
			}
			cond := strings.TrimSpace(filt[1:])
			// split on ==
			idx := strings.Index(cond, "==")
			if idx < 0 {
				return nil, fmt.Errorf("unsupported condition %q", cond)
			}
			lk := strings.TrimSpace(cond[:idx])
			rv := strings.TrimSpace(cond[idx+2:])
			// strip quotes around rv
			if strings.HasPrefix(rv, "'") && strings.HasSuffix(rv, "'") {
				rv = strings.Trim(rv, "'")
			} else if strings.HasPrefix(rv, "\"") && strings.HasSuffix(rv, "\"") {
				rv = strings.Trim(rv, "\"")
			}
			steps = append(steps, pathStep{Where: map[string]string{lk: rv}})
		} else {
			// plain key
			steps = append(steps, pathStep{Key: p})
		}
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("empty match expression")
	}
	return steps, nil
}

// ApplyOverlayOpsInline applies a slice of inline ops (from scenario.yaml) directly.
func ApplyOverlayOpsInline(dstRoot string, entries []InlineOp, tmplCtx any) error {
	for _, e := range entries {
		if e.File == "" {
			return fmt.Errorf("inline op missing file")
		}
		target := filepath.Join(dstRoot, e.File)
		db, err := os.ReadFile(target)
		if err != nil {
			return fmt.Errorf("read target %s: %w", target, err)
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(db, &doc); err != nil {
			return fmt.Errorf("parse target %s: %w", target, err)
		}
		// template the match string in case it uses vars
		match := e.Match
		if match != "" {
			if rb, rerr := renderTemplate([]byte(match), tmplCtx); rerr == nil {
				match = string(rb)
			}
		}
		steps, perr := parseJMESMatch(match)
		if perr != nil {
			return fmt.Errorf("parse match '%s': %w", match, perr)
		}
		matched := matchNodes(&doc, steps)
		if len(matched) == 0 {
			return fmt.Errorf("inline op match found no targets in %s for %s", target, match)
		}
		for _, m := range matched {
			if m.Kind != yaml.MappingNode {
				continue
			}
			for k, v := range e.Set {
				setMappingKey(m, k, v)
			}
		}
		out, err := yaml.Marshal(&doc)
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, out, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// ApplyOverlayDir merges all files from overlayDir into dstRoot preserving structure.
// For YAML files, perform in-place merge; for others, overwrite.
func ApplyOverlayDir(dstRoot, overlayDir string, tmplCtx any) error {
	return filepath.WalkDir(overlayDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(overlayDir, p)
		out := filepath.Join(dstRoot, rel)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if isYAML(p) {
			return applyOverlayFile(out, p, tmplCtx)
		}
		// Non-YAML: copy/overwrite
		ob, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		tb, err := renderTemplate(ob, tmplCtx)
		if err != nil {
			return err
		}
		return os.WriteFile(out, tb, 0o644)
	})
}
