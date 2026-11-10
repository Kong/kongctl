package adopt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	adoptCommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/adopt/common"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/internal/declarative/secrets"
	"github.com/kong/kongctl/internal/declarative/tags"
	"github.com/kong/kongctl/internal/util"
	"github.com/spf13/cobra"
	//nolint:gomodguard_v2 // yaml.v3 is required to emit !ref nodes without altering public strings.
	yamlv3 "gopkg.in/yaml.v3"
	"sigs.k8s.io/yaml"
)

const (
	declarativeOutputFlag = "declarative-output"
	parentRefFlag         = "parent-ref"
)

type declarativeExportKey struct{}

type adoptionParent struct {
	kind        resources.ResourceType
	id          string
	directoryID string
}

type exportedDeclaration struct {
	collection  []string
	fields      map[string]any
	parentField string
}

type adoptionExport struct {
	file            *os.File
	path, parentRef string
	dryRun          bool
	prepared        map[string]exportedDeclaration
	parents         []exportedDeclaration
	parentIDs       map[string]string
	refs            map[string]string
	completed       []exportedDeclaration
	written         bool
	identity        os.FileInfo
}

func addDeclarativeExportFlags(cmd *cobra.Command) {
	cmd.Flags().String(declarativeOutputFlag, "", "Write selected adopted children as declarative YAML to a new file")
	cmd.Flags().String(parentRefFlag, "", "Existing parent ref for exported !ref links; requires --declarative-output")
}

func setupChildAdoptRun(cmd *cobra.Command, args []string) (adoptCommon.AdoptRunSetup, func(), error) {
	setup, err := adoptCommon.SetupAdoptRun(cmd, args)
	if err != nil {
		return setup, func() {}, err
	}
	export, err := openAdoptionExport(cmd, setup.AdoptFlags.DryRun)
	if err != nil {
		return setup, func() {}, err
	}
	if export == nil {
		return setup, func() {}, nil
	}
	cmd.SetContext(context.WithValue(adoptCommon.EnsureContext(cmd.Context()), declarativeExportKey{}, export))
	return setup, export.close, nil
}

func openAdoptionExport(cmd *cobra.Command, dryRun bool) (*adoptionExport, error) {
	output, _ := cmd.Flags().GetString(declarativeOutputFlag)
	parentRef, _ := cmd.Flags().GetString(parentRefFlag)
	if cmd.Flags().Changed(parentRefFlag) {
		if err := resources.ValidateRef(parentRef); err != nil {
			return nil, fmt.Errorf("invalid --parent-ref: %w", err)
		}
		if output == "" {
			return nil, fmt.Errorf("--parent-ref requires --declarative-output")
		}
	}
	if !cmd.Flags().Changed(declarativeOutputFlag) {
		return nil, nil
	}
	if strings.TrimSpace(output) == "" || output == "-" {
		return nil, fmt.Errorf("--declarative-output requires a file path; stdout remains reserved for adoption results")
	}
	absolute, err := filepath.Abs(output)
	if err != nil {
		return nil, fmt.Errorf("resolve declarative output: %w", err)
	}
	// Reserve the destination before any remote resource write. O_EXCL also
	// rejects existing symlinks; an export never truncates an existing input.
	file, err := os.OpenFile(absolute, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create declarative output %q before adoption: %w", absolute, err)
	}
	identity, err := file.Stat()
	if err != nil {
		_ = file.Close()
		_ = os.Remove(absolute)
		return nil, fmt.Errorf("inspect declarative output: %w", err)
	}
	export := &adoptionExport{
		file: file, identity: identity, path: absolute, parentRef: parentRef, dryRun: dryRun,
		prepared: make(map[string]exportedDeclaration), parentIDs: make(map[string]string), refs: make(map[string]string),
	}
	if parentRef != "" {
		export.refs[parentRef] = "existing-parent"
	}
	return export, nil
}

func (e *adoptionExport) close() {
	_ = e.file.Close()
	if !e.written {
		if current, err := os.Lstat(e.path); err == nil && os.SameFile(e.identity, current) {
			_ = os.Remove(e.path)
		}
	}
}

func adoptionExportFromContext(ctx context.Context) *adoptionExport {
	if ctx == nil {
		return nil
	}
	export, _ := ctx.Value(declarativeExportKey{}).(*adoptionExport)
	return export
}

func prepareDeclarativeChild(ctx context.Context, kind resources.ResourceType, id, ref, namespace string,
	protected bool, payload any, parent adoptionParent,
) error {
	export := adoptionExportFromContext(ctx)
	if export == nil {
		return nil
	}
	shape, err := adoptionExportShape(kind)
	if err != nil {
		return err
	}
	parentRef := export.parentRef
	if parentRef == "" {
		parentRef, err = export.externalParent(parent)
		if err != nil {
			return err
		}
	}
	fields, err := adoptionPublicFields(payload)
	if err != nil {
		return err
	}
	for _, key := range []string{
		resources.SchemaFieldID, resources.SchemaFieldCreatedAt,
		resources.SchemaFieldUpdatedAt, "managed_by", "konnect_managed",
	} {
		delete(fields, key)
	}
	omitExportSecrets(kind, fields, "")
	if kind == resources.ResourceTypeAIGatewayModel {
		resources.OmitAIGatewayModelDatastoreConnections(fields)
	}
	fields[resources.SchemaFieldRef] = ref
	fields[shape.parentField] = tags.RefPlaceholderPrefix + parentRef + "#id"
	fields["kongctl"] = map[string]any{"namespace": namespace, "protected": protected}
	if kind == resources.ResourceTypeIdentityAuthServerClient {
		values := make(map[string]string)
		if raw, ok := fields["labels"].(map[string]any); ok {
			for key, value := range raw {
				if text, ok := value.(string); ok {
					values[key] = text
				}
			}
		}
		fields["labels"] = labels.GetUserLabels(values)
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return fmt.Errorf("encode declarative child")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(shape.resource); err != nil {
		return fmt.Errorf("cannot map selected child to declarative configuration")
	}
	if err := shape.resource.Validate(); err != nil {
		return fmt.Errorf("selected child cannot be exported: %w", err)
	}
	// Typed declarations omit read-only fields and independently owned descendants.
	fields, err = adoptionPublicFields(shape.resource)
	if err != nil {
		return err
	}
	omitExportSecrets(kind, fields, "")
	if existing, exists := export.refs[ref]; exists && existing != id {
		return fmt.Errorf("exported ref %q collides with another selected resource or parent; "+
			"choose another --parent-ref", ref)
	}
	export.refs[ref] = id
	declaration := exportedDeclaration{collection: shape.collection, fields: fields, parentField: shape.parentField}
	export.prepared[id] = declaration
	// Exercise YAML serialization during preflight rather than after mutation.
	_, err = export.render([]exportedDeclaration{declaration})
	return err
}

func finishDeclarativeChild(ctx context.Context, result *adoptCommon.AdoptResult) (*adoptCommon.AdoptResult, error) {
	export := adoptionExportFromContext(ctx)
	if export == nil {
		return result, nil
	}
	declaration, ok := export.prepared[result.ID]
	if !ok {
		return result, fmt.Errorf("resource %s was adopted but no declarative snapshot was prepared", result.ID)
	}
	export.completed = append(export.completed, declaration)
	data, err := export.render(export.completed)
	if err == nil {
		err = export.persist(data)
	}
	if err != nil {
		action := "adopted"
		if export.dryRun {
			action = "previewed"
		}
		return result, fmt.Errorf("resource %q (%s) was %s but declarative output %q could not be written: %w",
			result.Name, result.ID, action, export.path, err)
	}
	return result, nil
}

// Replace only our reserved destination, keeping the last complete snapshot
// intact if a later export fails. Existing user inputs are never opened here.
func (e *adoptionExport) persist(data []byte) error {
	staged, err := os.CreateTemp(filepath.Dir(e.path), ".adopt-export-*.yaml")
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = staged.Close()
			_ = os.Remove(staged.Name())
		}
	}()
	if _, err := staged.Write(data); err != nil {
		return err
	}
	if err := staged.Sync(); err != nil {
		return err
	}
	expected, err := e.file.Stat()
	if err != nil {
		return err
	}
	current, err := os.Lstat(e.path)
	if err != nil {
		return err
	}
	if !os.SameFile(expected, current) {
		return fmt.Errorf("declarative destination changed during adoption")
	}
	identity, err := staged.Stat()
	if err != nil {
		return err
	}
	if err := os.Rename(staged.Name(), e.path); err != nil {
		return err
	}
	_ = e.file.Close()
	e.file = staged
	e.identity = identity
	e.written = true
	committed = true
	return nil
}

func adoptionPublicFields(value any) (map[string]any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("cannot encode declarative resource snapshot")
	}
	var fields map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&fields); err != nil {
		return nil, fmt.Errorf("cannot decode declarative resource snapshot")
	}
	return fields, nil
}

// The reviewed write-only catalog is shared with loading and execution. Remove
// values even if a backend happens to return them; retain public array siblings.
func omitExportSecrets(kind resources.ResourceType, value any, path string) {
	switch node := value.(type) {
	case map[string]any:
		for key, field := range node {
			pointer := path + "/" + strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
			if _, secret := secrets.Match(kind, pointer); secret {
				delete(node, key)
				continue
			}
			omitExportSecrets(kind, field, pointer)
		}
	case []any:
		for i, field := range node {
			omitExportSecrets(kind, field, path+"/"+strconv.Itoa(i))
		}
	}
}

func (e *adoptionExport) externalParent(parent adoptionParent) (string, error) {
	if !util.IsValidUUID(parent.id) {
		return "", fmt.Errorf("declarative parent ID must be a UUID")
	}
	key := string(parent.kind) + "/" + parent.id
	if ref, ok := e.parentIDs[key]; ok {
		return ref, nil
	}
	var collection []string
	var prefix, parentField, directoryRef string
	//exhaustive:ignore
	switch parent.kind {
	case resources.ResourceTypeAIGateway:
		collection = []string{"ai_gateways"}
		prefix = "adopt-gateway-"
	case resources.ResourceTypeIdentityDirectory:
		collection = []string{"identity", "directories"}
		prefix = "adopt-directory-"
	case resources.ResourceTypeIdentityAuthServer:
		collection = []string{"identity", "auth_servers"}
		prefix = "adopt-auth-server-"
	case resources.ResourceTypeIdentityPrincipal:
		collection = []string{"identity", "principals"}
		prefix = "adopt-principal-"
		parentField = "directory"
		var err error
		directoryRef,
			err = e.externalParent(adoptionParent{
			kind: resources.ResourceTypeIdentityDirectory,
			id:   parent.directoryID,
		})
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported declarative adoption parent %s", parent.kind)
	}
	ref := prefix + parent.id
	if err := resources.ValidateRef(ref); err != nil {
		return "", fmt.Errorf("cannot export parent ref: %w", err)
	}
	if _, exists := e.refs[ref]; exists {
		return "", fmt.Errorf("generated parent ref %q collides with selected resource", ref)
	}
	fields := map[string]any{"ref": ref, "_external": map[string]any{"id": parent.id}}
	if parentField != "" {
		fields[parentField] = tags.RefPlaceholderPrefix + directoryRef + "#id"
	}
	e.parentIDs[key] = ref
	e.refs[ref] = key
	e.parents = append(e.parents, exportedDeclaration{collection: collection, fields: fields, parentField: parentField})
	return ref, nil
}

func (e *adoptionExport) render(selected []exportedDeclaration) ([]byte, error) {
	root := &yamlv3.Node{Kind: yamlv3.MappingNode, Tag: "!!map"}
	for _, declaration := range append(append([]exportedDeclaration(nil), e.parents...), selected...) {
		data, err := yaml.Marshal(declaration.fields)
		if err != nil {
			return nil, fmt.Errorf("cannot marshal declarative adoption output")
		}
		var document yamlv3.Node
		if err := yamlv3.Unmarshal(data, &document); err != nil {
			return nil, fmt.Errorf("cannot encode declarative adoption output")
		}
		if len(document.Content) != 1 || document.Content[0].Kind != yamlv3.MappingNode {
			return nil, fmt.Errorf("declarative adoption output must contain a resource mapping")
		}
		node := document.Content[0]
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == declaration.parentField {
				link := node.Content[i+1]
				link.Tag = tags.TagRef
				link.Value = strings.TrimPrefix(link.Value, tags.RefPlaceholderPrefix)
			}
		}
		container := root
		for i, key := range declaration.collection {
			var next *yamlv3.Node
			for j := 0; j+1 < len(container.Content); j += 2 {
				if container.Content[j].Value == key {
					next = container.Content[j+1]
					break
				}
			}
			if next == nil {
				next = &yamlv3.Node{Kind: yamlv3.MappingNode, Tag: "!!map"}
				if i == len(declaration.collection)-1 {
					next.Kind = yamlv3.SequenceNode
					next.Tag = "!!seq"
				}
				container.Content = append(container.Content, &yamlv3.Node{Kind: yamlv3.ScalarNode, Tag: "!!str", Value: key}, next)
			}
			container = next
		}
		container.Content = append(container.Content, node)
	}
	var out bytes.Buffer
	if e.dryRun {
		out.WriteString("# DRY RUN: proposed configuration; these resources have not been adopted by this command.\n")
	} else {
		out.WriteString("# Declarative configuration for successfully adopted resources.\n")
	}
	encoder := yamlv3.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(root); err != nil {
		return nil, fmt.Errorf("cannot serialize declarative adoption output")
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
