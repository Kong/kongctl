package extensions

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"
)

const (
	ManifestFileName   = "kongctl-extension.yaml"
	ManifestSchemaV1   = 1
	MaxManifestBytes   = 256 * 1024
	maxCommandPaths    = 64
	maxPathSegments    = 8
	maxAliases         = 8
	maxExamples        = 16
	maxMetadataEntries = 64
	maxDescriptionLen  = 4096
	maxTextLen         = 512
)

var (
	identitySegmentPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)
	commandSegmentPattern  = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
	flagNamePattern        = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

	allowedTopLevelKeys = []string{
		"schema_version",
		"publisher",
		"name",
		"version",
		"summary",
		"runtime",
		"compatibility",
		"command_paths",
	}

	openBuiltInRoots = []string{"get", "list"}

	closedBuiltInRoots = []string{
		"add",
		"adopt",
		"api",
		"apply",
		"create",
		"delete",
		"diff",
		"dump",
		"explain",
		"export",
		"help",
		"install",
		"lint",
		"link",
		"listen",
		"login",
		"logout",
		"patch",
		"plan",
		"ps",
		"scaffold",
		"sync",
		"uninstall",
		"upgrade",
		"version",
		"view",
	}
)

type Manifest struct {
	SchemaVersion int           `json:"schema_version" yaml:"schema_version"`
	Publisher     string        `json:"publisher"      yaml:"publisher"`
	Name          string        `json:"name"           yaml:"name"`
	Version       string        `json:"version,omitempty" yaml:"version,omitempty"`
	Summary       string        `json:"summary,omitempty" yaml:"summary,omitempty"`
	Runtime       Runtime       `json:"runtime"        yaml:"runtime"`
	Compatibility Compatibility `json:"compatibility" yaml:"compatibility,omitempty"`
	CommandPaths  []CommandPath `json:"command_paths" yaml:"command_paths"`
}

type Runtime struct {
	Command string `json:"command" yaml:"command"`
}

type Compatibility struct {
	MinVersion string `json:"min_version,omitempty" yaml:"min_version,omitempty"`
	MaxVersion string `json:"max_version,omitempty" yaml:"max_version,omitempty"`
}

type CommandPath struct {
	ID          string        `json:"id"                    yaml:"id,omitempty"`
	Path        []PathSegment `json:"path"                  yaml:"path"`
	Summary     string        `json:"summary,omitempty"     yaml:"summary,omitempty"`
	Description string        `json:"description,omitempty" yaml:"description,omitempty"`
	Usage       string        `json:"usage,omitempty"       yaml:"usage,omitempty"`
	Examples    []string      `json:"examples,omitempty"    yaml:"examples,omitempty"`
	Args        []Argument    `json:"args,omitempty"        yaml:"args,omitempty"`
	Flags       []Flag        `json:"flags,omitempty"       yaml:"flags,omitempty"`
}

type PathSegment struct {
	Name    string   `json:"name"              yaml:"name"`
	Aliases []string `json:"aliases,omitempty" yaml:"aliases,omitempty"`
}

type Argument struct {
	Name        string `json:"name"                  yaml:"name"`
	Required    bool   `json:"required,omitempty"    yaml:"required,omitempty"`
	Repeatable  bool   `json:"repeatable,omitempty"  yaml:"repeatable,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type Flag struct {
	Name        string `json:"name"                  yaml:"name"`
	Type        string `json:"type,omitempty"        yaml:"type,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type Extension struct {
	ID           string        `json:"id"`
	InstallType  InstallType   `json:"install_type"`
	Manifest     Manifest      `json:"manifest"`
	CommandPaths []CommandPath `json:"command_paths"`
	PackageDir   string        `json:"package_dir,omitempty"`
	LinkedDir    string        `json:"linked_dir,omitempty"`
	Install      *InstallState `json:"install,omitempty"`
	Link         *LinkState    `json:"link,omitempty"`
}

type InstallType string

const (
	InstallTypeInstalled InstallType = "installed"
	InstallTypeLinked    InstallType = "linked"
)

func LoadManifestFile(path string) (Manifest, []byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return Manifest{}, nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, MaxManifestBytes+1))
	if err != nil {
		return Manifest{}, nil, err
	}
	if len(data) > MaxManifestBytes {
		return Manifest{}, nil, fmt.Errorf("%s exceeds %d bytes", ManifestFileName, MaxManifestBytes)
	}

	manifest, err := ParseManifest(data)
	if err != nil {
		return Manifest{}, nil, err
	}

	return manifest, data, nil
}

func ParseManifest(data []byte) (Manifest, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return Manifest{}, fmt.Errorf("%s is empty", ManifestFileName)
	}
	if err := validateYAMLDocument(data); err != nil {
		return Manifest{}, err
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode %s: %w", ManifestFileName, err)
	}
	var extra yaml.Node
	err := decoder.Decode(&extra)
	if err != nil && !errors.Is(err, io.EOF) {
		return Manifest{}, fmt.Errorf("decode %s: %w", ManifestFileName, err)
	}
	if err == nil && len(extra.Content) > 0 {
		return Manifest{}, fmt.Errorf("%s must contain exactly one YAML document", ManifestFileName)
	}

	if err := NormalizeAndValidateManifest(&manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func NormalizeAndValidateManifest(manifest *Manifest) error {
	if manifest == nil {
		return fmt.Errorf("manifest is required")
	}
	if manifest.SchemaVersion != ManifestSchemaV1 {
		return fmt.Errorf("unsupported schema_version %d", manifest.SchemaVersion)
	}

	manifest.Publisher = strings.TrimSpace(strings.ToLower(manifest.Publisher))
	manifest.Name = strings.TrimSpace(strings.ToLower(manifest.Name))
	if err := ValidateIdentitySegment("publisher", manifest.Publisher); err != nil {
		return err
	}
	if err := ValidateIdentitySegment("name", manifest.Name); err != nil {
		return err
	}

	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.Summary = strings.TrimSpace(manifest.Summary)
	if len(manifest.Summary) > maxTextLen {
		return fmt.Errorf("summary must be %d characters or fewer", maxTextLen)
	}

	manifest.Runtime.Command = filepath.ToSlash(strings.TrimSpace(manifest.Runtime.Command))
	if err := ValidateRuntimeCommand(manifest.Runtime.Command); err != nil {
		return err
	}

	if len(manifest.CommandPaths) == 0 {
		return fmt.Errorf("at least one command_paths entry is required")
	}
	if len(manifest.CommandPaths) > maxCommandPaths {
		return fmt.Errorf("command_paths must contain %d entries or fewer", maxCommandPaths)
	}

	seenIDs := map[string]struct{}{}
	seenPaths := map[string]struct{}{}
	extensionID := ExtensionID(manifest.Publisher, manifest.Name)
	for i := range manifest.CommandPaths {
		path := &manifest.CommandPaths[i]
		if err := normalizeAndValidateCommandPath(extensionID, path); err != nil {
			return fmt.Errorf("command_paths[%d]: %w", i, err)
		}
		if _, ok := seenIDs[path.ID]; ok {
			return fmt.Errorf("command_paths[%d]: duplicate id %q", i, path.ID)
		}
		seenIDs[path.ID] = struct{}{}

		canonical := strings.Join(CommandPathNames(*path), " ")
		if _, ok := seenPaths[canonical]; ok {
			return fmt.Errorf("command_paths[%d]: duplicate path %q", i, canonical)
		}
		seenPaths[canonical] = struct{}{}
	}

	return nil
}

func ValidateIdentitySegment(field, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	if value == "." || value == ".." {
		return fmt.Errorf("%s cannot be %q", field, value)
	}
	if strings.ContainsAny(value, `/\:`) {
		return fmt.Errorf("%s %q contains a reserved path character", field, value)
	}
	if !identitySegmentPattern.MatchString(value) {
		return fmt.Errorf("%s %q must match %s", field, value, identitySegmentPattern.String())
	}
	return nil
}

func ValidateExtensionID(id string) error {
	publisher, name, ok := strings.Cut(id, "/")
	if !ok {
		return fmt.Errorf("extension id must use publisher/name form")
	}
	if strings.Contains(name, "/") {
		return fmt.Errorf("extension id must use publisher/name form")
	}
	if err := ValidateIdentitySegment("publisher", publisher); err != nil {
		return err
	}
	return ValidateIdentitySegment("name", name)
}

func ExtensionID(publisher, name string) string {
	return publisher + "/" + name
}

func SplitExtensionID(id string) (string, string, error) {
	publisher, name, ok := strings.Cut(id, "/")
	if !ok || strings.Contains(name, "/") {
		return "", "", fmt.Errorf("extension id must use publisher/name form")
	}
	if err := ValidateIdentitySegment("publisher", publisher); err != nil {
		return "", "", err
	}
	if err := ValidateIdentitySegment("name", name); err != nil {
		return "", "", err
	}
	return publisher, name, nil
}

func ValidateRuntimeCommand(command string) error {
	if command == "" {
		return fmt.Errorf("runtime.command is required")
	}
	if filepath.IsAbs(command) || strings.HasPrefix(command, "/") {
		return fmt.Errorf("runtime.command must be relative")
	}
	if strings.Contains(command, `\`) || strings.Contains(command, ":") {
		return fmt.Errorf("runtime.command %q contains a reserved path character", command)
	}
	cleaned := filepath.Clean(command)
	if cleaned == "." {
		return fmt.Errorf("runtime.command must name a file")
	}
	for segment := range strings.SplitSeq(filepath.ToSlash(cleaned), "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("runtime.command must stay inside the extension root")
		}
	}
	return nil
}

func ResolveRuntime(root, command string) (string, error) {
	if err := ValidateRuntimeCommand(command); err != nil {
		return "", err
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve extension root: %w", err)
	}
	target := filepath.Join(rootReal, filepath.FromSlash(command))
	targetReal, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", fmt.Errorf("resolve runtime.command: %w", err)
	}
	if err := ensureInside(rootReal, targetReal); err != nil {
		return "", fmt.Errorf("runtime.command escapes extension root: %w", err)
	}
	info, err := os.Stat(targetReal)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("runtime.command %q is not a regular file", command)
	}
	if info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("runtime.command %q is not executable", command)
	}
	return targetReal, nil
}

func CommandPathNames(path CommandPath) []string {
	names := make([]string, 0, len(path.Path))
	for _, segment := range path.Path {
		names = append(names, segment.Name)
	}
	return names
}

func CommandPathString(path CommandPath) string {
	return strings.Join(CommandPathNames(path), " ")
}

func ContributionID(extensionID string, path CommandPath) string {
	if strings.TrimSpace(path.ID) != "" {
		return path.ID
	}
	return strings.ReplaceAll(extensionID+"/"+strings.Join(CommandPathNames(path), "_"), "/", "_")
}

func IsOpenBuiltInRoot(name string) bool {
	return slices.Contains(openBuiltInRoots, name)
}

func IsClosedBuiltInRoot(name string) bool {
	return slices.Contains(closedBuiltInRoots, name)
}

func normalizeAndValidateCommandPath(extensionID string, path *CommandPath) error {
	path.ID = strings.TrimSpace(path.ID)
	if path.ID != "" && !commandSegmentPattern.MatchString(path.ID) {
		return fmt.Errorf("id %q must match %s", path.ID, commandSegmentPattern.String())
	}
	if len(path.Path) == 0 {
		return fmt.Errorf("path is required")
	}
	if len(path.Path) > maxPathSegments {
		return fmt.Errorf("path must contain %d segments or fewer", maxPathSegments)
	}

	for i := range path.Path {
		segment := &path.Path[i]
		segment.Name = strings.TrimSpace(strings.ToLower(segment.Name))
		if !commandSegmentPattern.MatchString(segment.Name) {
			return fmt.Errorf("path[%d].name %q must match %s", i, segment.Name, commandSegmentPattern.String())
		}
		if len(segment.Aliases) > maxAliases {
			return fmt.Errorf("path[%d].aliases must contain %d entries or fewer", i, maxAliases)
		}
		seenAliases := map[string]struct{}{}
		for j := range segment.Aliases {
			alias := strings.TrimSpace(strings.ToLower(segment.Aliases[j]))
			if !commandSegmentPattern.MatchString(alias) {
				return fmt.Errorf("path[%d].aliases[%d] %q must match %s",
					i, j, alias, commandSegmentPattern.String())
			}
			if alias == segment.Name {
				return fmt.Errorf("path[%d].aliases[%d] duplicates canonical segment %q", i, j, segment.Name)
			}
			if _, ok := seenAliases[alias]; ok {
				return fmt.Errorf("path[%d].aliases[%d] duplicates alias %q", i, j, alias)
			}
			seenAliases[alias] = struct{}{}
			segment.Aliases[j] = alias
		}
	}

	root := path.Path[0]
	if IsOpenBuiltInRoot(root.Name) && len(root.Aliases) > 0 {
		return fmt.Errorf("built-in root segment %q cannot declare aliases", root.Name)
	}
	if IsClosedBuiltInRoot(root.Name) {
		return fmt.Errorf("built-in root command %q is closed to extension contributions", root.Name)
	}

	path.Summary = strings.TrimSpace(path.Summary)
	path.Description = strings.TrimSpace(path.Description)
	path.Usage = strings.TrimSpace(path.Usage)
	if path.ID == "" {
		path.ID = ContributionID(extensionID, *path)
	}
	if path.Summary == "" {
		path.Summary = fmt.Sprintf("Run %s extension command", extensionID)
	}
	if path.Usage == "" {
		path.Usage = "kongctl " + CommandPathString(*path) + " [args] [flags]"
	}
	if len(path.Summary) > maxTextLen {
		return fmt.Errorf("summary must be %d characters or fewer", maxTextLen)
	}
	if len(path.Description) > maxDescriptionLen {
		return fmt.Errorf("description must be %d characters or fewer", maxDescriptionLen)
	}
	if len(path.Usage) > maxTextLen {
		return fmt.Errorf("usage must be %d characters or fewer", maxTextLen)
	}
	if len(path.Examples) > maxExamples {
		return fmt.Errorf("examples must contain %d entries or fewer", maxExamples)
	}
	for i := range path.Examples {
		path.Examples[i] = strings.TrimSpace(path.Examples[i])
		if len(path.Examples[i]) > maxTextLen {
			return fmt.Errorf("examples[%d] must be %d characters or fewer", i, maxTextLen)
		}
	}
	if len(path.Args) > maxMetadataEntries {
		return fmt.Errorf("args must contain %d entries or fewer", maxMetadataEntries)
	}
	for i := range path.Args {
		if err := normalizeArg(&path.Args[i]); err != nil {
			return fmt.Errorf("args[%d]: %w", i, err)
		}
	}
	if len(path.Flags) > maxMetadataEntries {
		return fmt.Errorf("flags must contain %d entries or fewer", maxMetadataEntries)
	}
	for i := range path.Flags {
		if err := normalizeFlag(&path.Flags[i]); err != nil {
			return fmt.Errorf("flags[%d]: %w", i, err)
		}
	}

	return nil
}

func normalizeArg(arg *Argument) error {
	arg.Name = strings.TrimSpace(strings.ToLower(arg.Name))
	arg.Description = strings.TrimSpace(arg.Description)
	if !commandSegmentPattern.MatchString(arg.Name) {
		return fmt.Errorf("name %q must match %s", arg.Name, commandSegmentPattern.String())
	}
	if len(arg.Description) > maxTextLen {
		return fmt.Errorf("description must be %d characters or fewer", maxTextLen)
	}
	return nil
}

func normalizeFlag(flag *Flag) error {
	flag.Name = strings.TrimSpace(strings.ToLower(flag.Name))
	flag.Type = strings.TrimSpace(strings.ToLower(flag.Type))
	flag.Description = strings.TrimSpace(flag.Description)
	if !flagNamePattern.MatchString(flag.Name) {
		return fmt.Errorf("name %q must match %s", flag.Name, flagNamePattern.String())
	}
	if strings.HasPrefix(flag.Name, "-") {
		return fmt.Errorf("name %q must not include leading dashes", flag.Name)
	}
	if len(flag.Type) > 64 {
		return fmt.Errorf("type must be 64 characters or fewer")
	}
	if len(flag.Description) > maxTextLen {
		return fmt.Errorf("description must be %d characters or fewer", maxTextLen)
	}
	return nil
}

func validateYAMLDocument(data []byte) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var node yaml.Node
	if err := decoder.Decode(&node); err != nil {
		return fmt.Errorf("decode %s: %w", ManifestFileName, err)
	}
	var extra yaml.Node
	err := decoder.Decode(&extra)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("decode %s: %w", ManifestFileName, err)
	}
	if err == nil && len(extra.Content) > 0 {
		return fmt.Errorf("%s must contain exactly one YAML document", ManifestFileName)
	}
	if len(node.Content) == 0 {
		return fmt.Errorf("%s is empty", ManifestFileName)
	}
	if err := rejectUnsafeYAML(&node); err != nil {
		return err
	}
	document := node.Content[0]
	if document.Kind != yaml.MappingNode {
		return fmt.Errorf("%s must contain a mapping document", ManifestFileName)
	}
	for i := 0; i < len(document.Content); i += 2 {
		key := document.Content[i]
		if key.Kind != yaml.ScalarNode {
			return fmt.Errorf("%s contains a non-scalar top-level key", ManifestFileName)
		}
		if !slices.Contains(allowedTopLevelKeys, key.Value) {
			return fmt.Errorf("%s contains unknown top-level key %q", ManifestFileName, key.Value)
		}
	}
	return nil
}

func rejectUnsafeYAML(node *yaml.Node) error {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.AliasNode || node.Anchor != "" {
		return fmt.Errorf("%s must not contain YAML aliases or anchors", ManifestFileName)
	}
	if strings.HasPrefix(node.Tag, "!") && !strings.HasPrefix(node.Tag, "!!") {
		return fmt.Errorf("%s must not contain custom YAML tags", ManifestFileName)
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind == yaml.ScalarNode && key.Value == "<<" {
				return fmt.Errorf("%s must not contain YAML merge keys", ManifestFileName)
			}
		}
	}
	for i := range node.Content {
		if err := rejectUnsafeYAML(node.Content[i]); err != nil {
			return err
		}
	}
	return nil
}

func ensureInside(root, target string) error {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	if rel == "." {
		return nil
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		return fmt.Errorf("%q is outside %q", target, root)
	}
	return nil
}
