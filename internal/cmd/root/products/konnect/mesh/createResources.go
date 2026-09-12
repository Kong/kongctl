package mesh

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/table"
	"github.com/kong/kongctl/internal/cmd"
	"github.com/kong/kongctl/internal/cmd/output/tableview"
	meshcommon "github.com/kong/kongctl/internal/cmd/root/products/konnect/mesh/common"
	"github.com/kong/kongctl/internal/declarative/loader"
	"github.com/kong/kongctl/internal/konnect/apiutil"
	"github.com/kong/kongctl/internal/konnect/httpclient"
	"github.com/segmentio/cli"
	"go.yaml.in/yaml/v4"
)

// meshResource is one document read from the input, carrying only the fields
// needed to address it. The document is sent to the control plane in full.
type meshResource struct {
	Type string
	Name string
	Mesh string
	Body []byte
	// Origin names where the document came from, for error messages.
	Origin string
}

// applyResult records what happened to one document.
type applyResult struct {
	Resource meshResource
	Created  bool
	Err      error
}

// runCreateResources serves `create mesh -f <source>`, applying every document
// in the input to the control plane.
//
// Kuma addresses a resource by type and name and creates or replaces it with a
// PUT, which is what kumactl apply does. The verb here is create rather than
// apply because kongctl's apply carries plan-and-diff semantics this does not.
func runCreateResources(helper cmd.Helper, filenames []string) error {
	cfg, err := helper.GetConfig()
	if err != nil {
		return err
	}

	descriptors, err := Discover(helper)
	if err != nil {
		return cmd.PrepareExecutionError("failed to retrieve mesh resource types", err, helper.GetCmd())
	}

	sources, err := loader.ParseSources(filenames)
	if err != nil {
		return &cmd.ConfigurationError{Err: err}
	}

	resources, err := readResources(helper, sources, meshcommon.ResolveMesh(cfg))
	if err != nil {
		return err
	}
	if len(resources) == 0 {
		return &cmd.ConfigurationError{
			Err: errors.New("no mesh resources found in the given input"),
		}
	}

	results := make([]applyResult, 0, len(resources))
	var failed bool
	for _, resource := range resources {
		created, err := applyResource(helper, descriptors, resource)
		if err != nil {
			failed = true
		}
		results = append(results, applyResult{Resource: resource, Created: created, Err: err})
	}

	if err := reportApplyResults(helper, results); err != nil {
		return err
	}
	if failed {
		return cmd.PrepareExecutionError(
			"one or more mesh resources could not be applied", errApplyFailed, helper.GetCmd())
	}
	return nil
}

// errApplyFailed marks a partial failure. Per-resource detail is already
// reported, so this only sets the exit status.
var errApplyFailed = errors.New("see the reported resources above")

// applyResource sends one document, reporting whether the control plane created
// it rather than replaced an existing one.
func applyResource(helper cmd.Helper, descriptors []ResourceDescriptor, resource meshResource) (bool, error) {
	descriptor, err := ResolveType(descriptors, resource.Type)
	if err != nil {
		return false, err
	}

	// The control plane also refuses a write to a read-only type with a 405,
	// but saying so before sending names the type rather than the status.
	if descriptor.ReadOnly {
		return false, fmt.Errorf(
			"%s is read only on this control plane and cannot be created or updated", descriptor.Singular())
	}

	mesh := resource.Mesh
	if !descriptor.IsMeshScoped() {
		mesh = ""
	}

	status, err := sendForStatus(helper, http.MethodPut, descriptor.ItemPath(mesh, resource.Name), resource.Body)
	if err != nil {
		return false, err
	}
	return status == http.StatusCreated, nil
}

// readResources collects every document from the given sources.
func readResources(helper cmd.Helper, sources []loader.Source, defaultMesh string) ([]meshResource, error) {
	var resources []meshResource

	for _, source := range sources {
		switch source.Type {
		case loader.SourceTypeSTDIN:
			docs, err := decodeResources(helper.GetStreams().In, "stdin", defaultMesh)
			if err != nil {
				return nil, err
			}
			resources = append(resources, docs...)

		case loader.SourceTypeFile:
			docs, err := readResourceFile(source.Path, defaultMesh)
			if err != nil {
				return nil, err
			}
			resources = append(resources, docs...)

		case loader.SourceTypeDirectory:
			paths, err := yamlFilesIn(source.Path)
			if err != nil {
				return nil, err
			}
			for _, path := range paths {
				docs, err := readResourceFile(path, defaultMesh)
				if err != nil {
					return nil, err
				}
				resources = append(resources, docs...)
			}

		case loader.SourceTypeURL:
			docs, err := readResourceURL(helper, source.Path, defaultMesh)
			if err != nil {
				return nil, err
			}
			resources = append(resources, docs...)
		}
	}

	return resources, nil
}

func readResourceFile(path, defaultMesh string) ([]meshResource, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	defer file.Close()

	return decodeResources(bufio.NewReader(file), path, defaultMesh)
}

// readResourceURL fetches documents from an HTTP source. It deliberately does
// not carry the control plane credential, since the URL is not the control
// plane.
func readResourceURL(helper cmd.Helper, rawURL, defaultMesh string) ([]meshResource, error) {
	logger, err := helper.GetLogger()
	if err != nil {
		return nil, err
	}

	ctx := helper.GetContext()
	result, err := apiutil.Request(
		ctx, httpclient.NewLoggingHTTPClient(logger), http.MethodGet, "", rawURL, "", nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", rawURL, err)
	}
	if result.StatusCode < http.StatusOK || result.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("failed to fetch %s: status %d", rawURL, result.StatusCode)
	}

	return decodeResources(strings.NewReader(string(result.Body)), rawURL, defaultMesh)
}

// yamlFilesIn lists the YAML files directly inside a directory, sorted so that
// applying a directory twice sends the same order.
func yamlFilesIn(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	var paths []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".yaml", ".yml", ".json":
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
	}
	return paths, nil
}

// decodeResources reads every document from one input. YAML and JSON are both
// accepted, since JSON is valid YAML, and a multi document YAML stream is read
// document by document the way kumactl reads one.
func decodeResources(in io.Reader, origin, defaultMesh string) ([]meshResource, error) {
	decoder := yaml.NewDecoder(in)

	var resources []meshResource
	for index := 0; ; index++ {
		var document map[string]any
		err := decoder.Decode(&document)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", describeDocument(origin, index), err)
		}
		if len(document) == 0 {
			continue
		}

		resource, err := newMeshResource(document, origin, index, defaultMesh)
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}

	return resources, nil
}

// newMeshResource validates that a document can be addressed and renders it as
// the JSON the control plane expects.
func newMeshResource(document map[string]any, origin string, index int, defaultMesh string) (meshResource, error) {
	where := describeDocument(origin, index)

	resourceType := stringField(document, "type")
	if resourceType == "" {
		return meshResource{}, &cmd.ConfigurationError{
			Err: fmt.Errorf("%s has no 'type' field, so the resource type cannot be determined", where),
		}
	}

	name := stringField(document, "name")
	if name == "" {
		return meshResource{}, &cmd.ConfigurationError{
			Err: fmt.Errorf("%s has no 'name' field, so the resource cannot be addressed", where),
		}
	}

	mesh := stringField(document, "mesh")
	if mesh == "" {
		mesh = defaultMesh
	}

	body, err := json.Marshal(document)
	if err != nil {
		return meshResource{}, fmt.Errorf("failed to encode %s: %w", where, err)
	}

	return meshResource{
		Type:   resourceType,
		Name:   name,
		Mesh:   mesh,
		Body:   body,
		Origin: where,
	}, nil
}

func describeDocument(origin string, index int) string {
	if index == 0 {
		return origin
	}
	return fmt.Sprintf("%s (document %d)", origin, index+1)
}

// applyRow is the text table projection of one applied document.
type applyRow struct {
	Type   string `json:"type" table:"TYPE"`
	Name   string `json:"name" table:"NAME"`
	Mesh   string `json:"mesh" table:"MESH"`
	Result string `json:"result" table:"RESULT"`
}

// reportApplyResults renders what happened to each document. Every document is
// reported, successes and failures together, so a partial apply is legible
// rather than being masked by the first error.
func reportApplyResults(helper cmd.Helper, results []applyResult) error {
	outType, err := helper.GetOutputFormat()
	if err != nil {
		return err
	}

	printer, err := cli.Format(outType.String(), helper.GetStreams().Out)
	if err != nil {
		return err
	}
	defer printer.Flush()

	rows := make([]applyRow, 0, len(results))
	tableRows := make([]table.Row, 0, len(results))
	for _, result := range results {
		row := applyRow{
			Type:   result.Resource.Type,
			Name:   result.Resource.Name,
			Mesh:   result.Resource.Mesh,
			Result: describeApplyOutcome(result),
		}
		rows = append(rows, row)
		tableRows = append(tableRows, table.Row{row.Type, row.Name, row.Mesh, row.Result})
	}

	return tableview.RenderForFormat(
		helper,
		false,
		outType,
		printer,
		helper.GetStreams(),
		rows,
		rows,
		"Applied Mesh Resources",
		tableview.WithExactCustomTable([]string{colType, colName, colMesh, colResult}, tableRows),
		tableview.WithRootLabel(helper.GetCmd().Name()),
	)
}

func describeApplyOutcome(result applyResult) string {
	if result.Err != nil {
		return "failed: " + result.Err.Error()
	}
	if result.Created {
		return "created"
	}
	return "updated"
}
