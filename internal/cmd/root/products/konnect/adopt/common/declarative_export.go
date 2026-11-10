package common

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	kkComps "github.com/Kong/sdk-konnect-go/models/components"

	declexport "github.com/kong/kongctl/internal/declarative/export"
	"github.com/kong/kongctl/internal/declarative/labels"
	"github.com/kong/kongctl/internal/declarative/resources"
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

type DeclarativeExportKey struct{}

type AdoptionParent struct {
	Kind        resources.ResourceType
	ID          string
	DirectoryID string
}

type exportedDeclaration struct {
	collection  []string
	fields      map[string]any
	parentField string
}

type AdoptionExport struct {
	File            *os.File
	Path, parentRef string
	dryRun          bool
	prepared        map[string]exportedDeclaration
	parents         []exportedDeclaration
	parentIDs       map[string]string
	refs            map[string]string
	Completed       []exportedDeclaration
	written         bool
	identity        os.FileInfo
}

func AddDeclarativeExportFlags(cmd *cobra.Command) {
	cmd.Flags().String(declarativeOutputFlag, "", "Write selected adopted resources as declarative YAML to a new file")
	cmd.Flags().String(parentRefFlag, "", "Existing parent ref for exported !ref links; requires --declarative-output")
}

func SetupExportRun(cmd *cobra.Command, args []string) (AdoptRunSetup, func(), error) {
	setup, err := SetupAdoptRun(cmd, args)
	if err != nil {
		return setup, func() {}, err
	}
	export, err := OpenAdoptionExport(cmd, setup.AdoptFlags.DryRun)
	if err != nil {
		return setup, func() {}, err
	}
	if export == nil {
		return setup, func() {}, nil
	}
	cmd.SetContext(context.WithValue(EnsureContext(cmd.Context()), DeclarativeExportKey{}, export))
	return setup, export.Close, nil
}

func OpenAdoptionExport(cmd *cobra.Command, dryRun bool) (*AdoptionExport, error) {
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
	export := &AdoptionExport{
		File: file, identity: identity, Path: absolute, parentRef: parentRef, dryRun: dryRun,
		prepared: make(map[string]exportedDeclaration), parentIDs: make(map[string]string), refs: make(map[string]string),
	}
	if parentRef != "" {
		export.refs[parentRef] = "existing-parent"
	}
	return export, nil
}

func (e *AdoptionExport) Close() {
	_ = e.File.Close()
	if !e.written {
		if current, err := os.Lstat(e.Path); err == nil && os.SameFile(e.identity, current) {
			_ = os.Remove(e.Path)
		}
	}
}

func AdoptionExportFromContext(ctx context.Context) *AdoptionExport {
	if ctx == nil {
		return nil
	}
	export, _ := ctx.Value(DeclarativeExportKey{}).(*AdoptionExport)
	return export
}

func PrepareDeclarativeChild(ctx context.Context, kind resources.ResourceType, id, ref, namespace string,
	protected bool, payload any, parent AdoptionParent,
) error {
	export := AdoptionExportFromContext(ctx)
	if export == nil {
		return nil
	}
	if client, ok := payload.(kkComps.Client); ok {
		payload = declexport.MapIdentityClient(client, client.AllowScopes)
	}
	fields, err := declexport.PublicFields(payload)
	if err != nil {
		return err
	}
	shape, err := declexport.DeclarationShape(kind)
	if err != nil {
		return err
	}
	fields[resources.SchemaFieldRef] = ref
	if parent.Kind != "" {
		parentRef := export.parentRef
		if parentRef == "" {
			parentRef, err = export.externalParent(parent)
			if err != nil {
				return err
			}
		}
		fields[shape.ParentField] = tags.RefPlaceholderPrefix + parentRef + "#id"
	} else if export.parentRef != "" {
		return fmt.Errorf("--parent-ref is only supported for child resources")
	}
	fields["kongctl"] = map[string]any{"namespace": namespace, "protected": protected}
	if raw, ok := fields["labels"].(map[string]any); ok &&
		(parent.Kind == "" || kind == resources.ResourceTypeIdentityAuthServerClient) {
		for key := range raw {
			if labels.IsKongctlLabel(key) {
				delete(raw, key)
			}
		}
	}
	shape, err = declexport.Project(kind, fields)
	if err != nil {
		return err
	}
	if err := shape.Resource.Validate(); err != nil {
		return fmt.Errorf("selected resource cannot be exported: %w", err)
	}
	fields, err = declexport.PublicFields(shape.Resource)
	if err != nil {
		return err
	}
	declexport.OmitSecrets(kind, fields, "")
	if existing, exists := export.refs[ref]; exists && existing != id {
		return fmt.Errorf("exported ref %q collides with another selected resource or parent; "+
			"choose another --parent-ref", ref)
	}
	export.refs[ref] = id
	declaration := exportedDeclaration{collection: shape.Collection, fields: fields, parentField: shape.ParentField}
	export.prepared[id] = declaration
	// Exercise YAML serialization during preflight rather than after mutation.
	_, err = export.render([]exportedDeclaration{declaration})
	return err
}

func FinishDeclarativeChild(ctx context.Context, result *AdoptResult) (*AdoptResult, error) {
	export := AdoptionExportFromContext(ctx)
	if export == nil {
		return result, nil
	}
	declaration, ok := export.prepared[result.ID]
	if !ok {
		return result, fmt.Errorf("resource %s was adopted but no declarative snapshot was prepared", result.ID)
	}
	for _, completed := range export.Completed {
		if completed.fields[resources.SchemaFieldRef] == declaration.fields[resources.SchemaFieldRef] {
			return result, nil
		}
	}
	export.Completed = append(export.Completed, declaration)
	data, err := export.render(export.Completed)
	if err == nil {
		err = export.persist(data)
	}
	if err != nil {
		action := "adopted"
		if export.dryRun {
			action = "previewed"
		}
		return result, fmt.Errorf("resource %q (%s) was %s but declarative output %q could not be written: %w",
			result.Name, result.ID, action, export.Path, err)
	}
	return result, nil
}

// Replace only our reserved destination, keeping the last complete snapshot
// intact if a later export fails. Existing user inputs are never opened here.
func (e *AdoptionExport) persist(data []byte) error {
	staged, err := os.CreateTemp(filepath.Dir(e.Path), ".adopt-export-*.yaml")
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
	expected, err := e.File.Stat()
	if err != nil {
		return err
	}
	current, err := os.Lstat(e.Path)
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
	if err := os.Rename(staged.Name(), e.Path); err != nil {
		return err
	}
	_ = e.File.Close()
	e.File = staged
	e.identity = identity
	e.written = true
	committed = true
	return nil
}

func (e *AdoptionExport) externalParent(parent AdoptionParent) (string, error) {
	if !util.IsValidUUID(parent.ID) {
		return "", fmt.Errorf("declarative parent ID must be a UUID")
	}
	key := string(parent.Kind) + "/" + parent.ID
	if ref, ok := e.parentIDs[key]; ok {
		return ref, nil
	}
	var collection []string
	var prefix, parentField, directoryRef string
	//exhaustive:ignore
	switch parent.Kind {
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
			err = e.externalParent(AdoptionParent{
			Kind: resources.ResourceTypeIdentityDirectory,
			ID:   parent.DirectoryID,
		})
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported declarative adoption parent %s", parent.Kind)
	}
	ref := prefix + parent.ID
	if err := resources.ValidateRef(ref); err != nil {
		return "", fmt.Errorf("cannot export parent ref: %w", err)
	}
	if _, exists := e.refs[ref]; exists {
		return "", fmt.Errorf("generated parent ref %q collides with selected resource", ref)
	}
	fields := map[string]any{"ref": ref, "_external": map[string]any{"id": parent.ID}}
	if parentField != "" {
		fields[parentField] = tags.RefPlaceholderPrefix + directoryRef + "#id"
	}
	e.parentIDs[key] = ref
	e.refs[ref] = key
	e.parents = append(e.parents, exportedDeclaration{collection: collection, fields: fields, parentField: parentField})
	return ref, nil
}

func (e *AdoptionExport) render(selected []exportedDeclaration) ([]byte, error) {
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

// PrepareDeclarativeRoot validates the selected parent's declaration before the
// ownership write. Nothing is fetched or projected when export is disabled.
func PrepareDeclarativeRoot(ctx context.Context, kind resources.ResourceType, id, namespace string,
	payload any, protected bool,
) error {
	if AdoptionExportFromContext(ctx) == nil {
		return nil
	}
	resource, err := declexport.Root(payload)
	if err != nil {
		return err
	}
	return PrepareDeclarativeChild(ctx, kind, id, id, namespace, protected, resource, AdoptionParent{})
}

func AddDeclarativeRootExportFlag(cmd *cobra.Command) {
	cmd.Flags().String(declarativeOutputFlag, "", "Write the adopted resource as declarative YAML to a new file")
}
