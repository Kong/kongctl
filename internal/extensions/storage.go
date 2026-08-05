package extensions

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/kong/kongctl/internal/config"
	"github.com/kong/kongctl/internal/meta"
)

const (
	stateSchemaVersion = 1
	commandsCacheName  = "commands.cache.json"
	installStateName   = "install.json"
	linkStateName      = "link.json"
)

type Store struct {
	root string
}

type InstallState struct {
	SchemaVersion  int          `json:"schema_version"`
	ID             string       `json:"id"`
	InstalledAt    string       `json:"installed_at"`
	CLIVersion     string       `json:"cli_version"`
	Source         SourceState  `json:"source"`
	ManifestHash   string       `json:"manifest_hash"`
	RuntimeHash    string       `json:"runtime_hash"`
	PackageHash    string       `json:"package_hash,omitempty"`
	RuntimeCommand string       `json:"runtime_command"`
	Trust          TrustState   `json:"trust"`
	Upgrade        UpgradeState `json:"upgrade"`
}

type LinkState struct {
	SchemaVersion  int    `json:"schema_version"`
	ID             string `json:"id"`
	LinkedAt       string `json:"linked_at"`
	CLIVersion     string `json:"cli_version"`
	Path           string `json:"path"`
	RuntimeCommand string `json:"runtime_command"`
}

type SourceState struct {
	Type           string `json:"type"`
	Path           string `json:"path,omitempty"`
	Repository     string `json:"repository,omitempty"`
	URL            string `json:"url,omitempty"`
	Ref            string `json:"ref,omitempty"`
	ResolvedCommit string `json:"resolved_commit,omitempty"`
	ReleaseTag     string `json:"release_tag,omitempty"`
	AssetName      string `json:"asset_name,omitempty"`
	AssetURL       string `json:"asset_url,omitempty"`
}

type TrustState struct {
	Confirmed bool   `json:"confirmed"`
	Model     string `json:"model"`
}

type UpgradeState struct {
	Policy string `json:"policy"`
}

type CommandCache struct {
	SchemaVersion int           `json:"schema_version"`
	ID            string        `json:"id"`
	GeneratedAt   string        `json:"generated_at"`
	InstallType   InstallType   `json:"install_type"`
	Manifest      Manifest      `json:"manifest"`
	CommandPaths  []CommandPath `json:"command_paths"`
}

type InstallResult struct {
	Extension    Extension `json:"extension"`
	ManifestHash string    `json:"manifest_hash"`
	RuntimeHash  string    `json:"runtime_hash"`
	PackageHash  string    `json:"package_hash"`
}

// PackageObservation captures the observable package identity shown before a
// remote executable extension is trusted.
type PackageObservation struct {
	Manifest       Manifest `json:"manifest"`
	ManifestHash   string   `json:"manifest_hash"`
	RuntimeHash    string   `json:"runtime_hash"`
	PackageHash    string   `json:"package_hash"`
	RuntimeCommand string   `json:"runtime_command"`
}

type UninstallResult struct {
	ID             string `json:"id"`
	RemovedInstall bool   `json:"removed_install"`
	RemovedLink    bool   `json:"removed_link"`
	RemovedData    bool   `json:"removed_data"`
}

type installDirectoryOptions struct {
	Source  SourceState
	Trust   TrustState
	Upgrade UpgradeState
}

func NewStore(root string) Store {
	return Store{root: root}
}

func DefaultStore() (Store, error) {
	root, err := config.GetDefaultConfigPath()
	if err != nil {
		return Store{}, err
	}
	return NewStore(filepath.Join(root, "extensions")), nil
}

func (s Store) Root() string {
	return s.root
}

func (s Store) RuntimeDir() string {
	return filepath.Join(s.root, "runtime")
}

func (s Store) TempDir() string {
	return filepath.Join(s.root, "tmp")
}

func (s Store) DataDir(id string) (string, error) {
	publisher, name, err := SplitExtensionID(id)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.root, "data", publisher, name), nil
}

func (s Store) InstallLocal(source, cliVersion string, now time.Time) (InstallResult, error) {
	candidate, err := LoadLocalExtension(source, InstallTypeInstalled)
	if err != nil {
		return InstallResult{}, err
	}
	sourceRoot := candidate.PackageDir
	return s.installDirectory(sourceRoot, cliVersion, now, installDirectoryOptions{
		Source: SourceState{
			Type: SourceTypeLocalPath,
			Path: sourceRoot,
		},
		Trust: TrustState{
			Confirmed: true,
			Model:     "local",
		},
		Upgrade: UpgradeState{
			Policy: "reinstall",
		},
	})
}

func (s Store) InstallGitHubSource(
	sourceRoot string,
	fetched FetchedGitHubSource,
	cliVersion string,
	now time.Time,
	trustConfirmed bool,
) (InstallResult, error) {
	sourceType := fetched.SourceType
	if sourceType == "" {
		sourceType = SourceTypeGitHubSource
	}
	trustModel := "github_source_clone"
	upgradePolicy := "explicit_ref"
	if sourceType == SourceTypeGitHubReleaseAsset {
		trustModel = "github_release_asset"
		upgradePolicy = "github_release"
	}
	return s.installDirectory(sourceRoot, cliVersion, now, installDirectoryOptions{
		Source: SourceState{
			Type:           sourceType,
			Repository:     fetched.Repository,
			URL:            fetched.URL,
			Ref:            fetched.Ref,
			ResolvedCommit: fetched.ResolvedCommit,
			ReleaseTag:     fetched.ReleaseTag,
			AssetName:      fetched.AssetName,
			AssetURL:       fetched.AssetURL,
		},
		Trust: TrustState{
			Confirmed: trustConfirmed,
			Model:     trustModel,
		},
		Upgrade: UpgradeState{
			Policy: upgradePolicy,
		},
	})
}

func (s Store) installDirectory(
	sourceRoot string,
	cliVersion string,
	now time.Time,
	opts installDirectoryOptions,
) (InstallResult, error) {
	observation, err := ObservePackage(sourceRoot)
	if err != nil {
		return InstallResult{}, err
	}
	manifest := observation.Manifest
	if err := EnsureCompatible(manifest, cliVersion); err != nil {
		return InstallResult{}, err
	}

	id := ExtensionID(manifest.Publisher, manifest.Name)
	if err := s.ensureNotLinked(id); err != nil {
		return InstallResult{}, err
	}

	installDir, packageDir, err := s.installPaths(id)
	if err != nil {
		return InstallResult{}, err
	}
	if err := ensureNotInside(sourceRoot, installDir); err != nil {
		return InstallResult{}, err
	}

	if err := os.RemoveAll(installDir); err != nil {
		return InstallResult{}, err
	}
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		return InstallResult{}, err
	}
	if err := copyExtensionTree(sourceRoot, packageDir); err != nil {
		return InstallResult{}, err
	}
	runtimePath, err := ResolveRuntime(packageDir, manifest.Runtime.Command)
	if err != nil {
		return InstallResult{}, err
	}
	runtimeHash, err := hashFile(runtimePath)
	if err != nil {
		return InstallResult{}, err
	}
	packageHash, err := hashTree(packageDir)
	if err != nil {
		return InstallResult{}, err
	}
	if cliVersion == "" {
		cliVersion = meta.DefaultCLIVersion
	}

	state := InstallState{
		SchemaVersion:  stateSchemaVersion,
		ID:             id,
		InstalledAt:    now.UTC().Format(time.RFC3339),
		CLIVersion:     cliVersion,
		Source:         opts.Source,
		ManifestHash:   observation.ManifestHash,
		RuntimeHash:    runtimeHash,
		PackageHash:    packageHash,
		RuntimeCommand: manifest.Runtime.Command,
		Trust:          opts.Trust,
		Upgrade:        opts.Upgrade,
	}
	if err := writeJSON(filepath.Join(installDir, installStateName), state); err != nil {
		return InstallResult{}, err
	}

	ext := Extension{
		ID:           id,
		InstallType:  InstallTypeInstalled,
		Health:       ExtensionHealth{Status: ExtensionHealthReady},
		Manifest:     manifest,
		CommandPaths: manifest.CommandPaths,
		PackageDir:   packageDir,
		Install:      &state,
	}
	if err := s.writeCommandCache(id, ext, now); err != nil {
		return InstallResult{}, err
	}

	return InstallResult{
		Extension:    ext,
		ManifestHash: observation.ManifestHash,
		RuntimeHash:  runtimeHash,
		PackageHash:  packageHash,
	}, nil
}

// ObservePackage computes package identity and integrity hashes without
// modifying managed extension state.
func ObservePackage(sourceRoot string) (PackageObservation, error) {
	manifest, manifestBytes, err := LoadManifestFile(filepath.Join(sourceRoot, ManifestFileName))
	if err != nil {
		return PackageObservation{}, err
	}
	runtimePath, err := ResolveRuntime(sourceRoot, manifest.Runtime.Command)
	if err != nil {
		return PackageObservation{}, err
	}
	runtimeHash, err := hashFile(runtimePath)
	if err != nil {
		return PackageObservation{}, err
	}
	packageHash, err := hashTree(sourceRoot)
	if err != nil {
		return PackageObservation{}, err
	}
	return PackageObservation{
		Manifest:       manifest,
		ManifestHash:   hashBytes(manifestBytes),
		RuntimeHash:    runtimeHash,
		PackageHash:    packageHash,
		RuntimeCommand: manifest.Runtime.Command,
	}, nil
}

func (s Store) LinkLocal(source, cliVersion string, now time.Time) (Extension, error) {
	candidate, err := LoadLocalExtension(source, InstallTypeLinked)
	if err != nil {
		return Extension{}, err
	}
	sourceRoot := candidate.LinkedDir
	manifest := candidate.Manifest
	if cliVersion == "" {
		cliVersion = meta.DefaultCLIVersion
	}
	if err := EnsureCompatible(manifest, cliVersion); err != nil {
		return Extension{}, err
	}

	id := ExtensionID(manifest.Publisher, manifest.Name)
	if err := s.ensureNotInstalled(id); err != nil {
		return Extension{}, err
	}
	linkDir, err := s.linkDir(id)
	if err != nil {
		return Extension{}, err
	}
	if err := os.RemoveAll(linkDir); err != nil {
		return Extension{}, err
	}
	if err := os.MkdirAll(linkDir, 0o755); err != nil {
		return Extension{}, err
	}

	state := LinkState{
		SchemaVersion:  stateSchemaVersion,
		ID:             id,
		LinkedAt:       now.UTC().Format(time.RFC3339),
		CLIVersion:     cliVersion,
		Path:           sourceRoot,
		RuntimeCommand: manifest.Runtime.Command,
	}
	if err := writeJSON(filepath.Join(linkDir, linkStateName), state); err != nil {
		return Extension{}, err
	}

	ext := Extension{
		ID:           id,
		InstallType:  InstallTypeLinked,
		Health:       ExtensionHealth{Status: ExtensionHealthReady},
		Manifest:     manifest,
		CommandPaths: manifest.CommandPaths,
		LinkedDir:    sourceRoot,
		Link:         &state,
	}
	if err := s.writeCommandCache(id, ext, now); err != nil {
		return Extension{}, err
	}

	return ext, nil
}

func LoadLocalExtension(source string, installType InstallType) (Extension, error) {
	sourceRoot, err := validateLocalExtensionRoot(source)
	if err != nil {
		return Extension{}, err
	}
	manifest, _, err := LoadManifestFile(filepath.Join(sourceRoot, ManifestFileName))
	if err != nil {
		return Extension{}, err
	}
	if _, err := ResolveRuntime(sourceRoot, manifest.Runtime.Command); err != nil {
		return Extension{}, err
	}
	id := ExtensionID(manifest.Publisher, manifest.Name)
	ext := Extension{
		ID:           id,
		InstallType:  installType,
		Health:       ExtensionHealth{Status: ExtensionHealthReady},
		Manifest:     manifest,
		CommandPaths: manifest.CommandPaths,
	}
	switch installType {
	case InstallTypeInstalled:
		ext.PackageDir = sourceRoot
	case InstallTypeLinked:
		ext.LinkedDir = sourceRoot
	default:
		return Extension{}, fmt.Errorf("unsupported extension install type %q", installType)
	}
	return ext, nil
}

func (s Store) Uninstall(id string, removeData bool) (UninstallResult, error) {
	if err := ValidateExtensionID(id); err != nil {
		return UninstallResult{}, err
	}
	installDir, _, err := s.installPaths(id)
	if err != nil {
		return UninstallResult{}, err
	}
	linkDir, err := s.linkDir(id)
	if err != nil {
		return UninstallResult{}, err
	}
	result := UninstallResult{ID: id}
	if _, err := os.Stat(installDir); err == nil {
		result.RemovedInstall = true
		if err := os.RemoveAll(installDir); err != nil {
			return UninstallResult{}, err
		}
	} else if err != nil && !os.IsNotExist(err) {
		return UninstallResult{}, err
	}
	if _, err := os.Stat(linkDir); err == nil {
		result.RemovedLink = true
		if err := os.RemoveAll(linkDir); err != nil {
			return UninstallResult{}, err
		}
	} else if err != nil && !os.IsNotExist(err) {
		return UninstallResult{}, err
	}
	if removeData {
		dataDir, err := s.DataDir(id)
		if err != nil {
			return UninstallResult{}, err
		}
		if _, err := os.Stat(dataDir); err == nil {
			result.RemovedData = true
			if err := os.RemoveAll(dataDir); err != nil {
				return UninstallResult{}, err
			}
		} else if err != nil && !os.IsNotExist(err) {
			return UninstallResult{}, err
		}
	}
	if !result.RemovedInstall && !result.RemovedLink {
		return UninstallResult{}, fmt.Errorf("extension %q is not installed or linked", id)
	}
	return result, nil
}

func (s Store) List() ([]Extension, error) {
	installed, err := s.listInstalled()
	if err != nil {
		return nil, err
	}
	linked, err := s.listLinked()
	if err != nil {
		return nil, err
	}
	extensions := make([]Extension, 0, len(installed)+len(linked))
	extensions = append(extensions, installed...)
	extensions = append(extensions, linked...)
	slices.SortFunc(extensions, func(a, b Extension) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		if a.InstallType < b.InstallType {
			return -1
		}
		if a.InstallType > b.InstallType {
			return 1
		}
		return 0
	})
	for i := 0; i < len(extensions); {
		j := i + 1
		for j < len(extensions) && extensions[j].ID == extensions[i].ID {
			j++
		}
		if j-i > 1 {
			for index := i; index < j; index++ {
				remediation := fmt.Sprintf(
					"run `kongctl uninstall extension %s` to remove both records, then install or link it again",
					extensions[index].ID,
				)
				extensions[index].Health = unhealthy(
					ExtensionHealthConflict,
					"duplicate_install_state",
					"both installed and linked state exist for this extension id",
					remediation,
				)
			}
		}
		i = j
	}
	return extensions, nil
}

func (s Store) Get(id string) (Extension, error) {
	if err := ValidateExtensionID(id); err != nil {
		return Extension{}, err
	}
	linked, linkedErr := s.loadLinked(id)
	installed, installedErr := s.loadInstalled(id)
	if linkedErr == nil && installedErr == nil {
		linked.Health = unhealthy(
			ExtensionHealthConflict,
			"duplicate_install_state",
			"both installed and linked state exist for this extension id",
			fmt.Sprintf("run `kongctl uninstall extension %s` to remove both records, then install or link it again", id),
		)
		return linked, nil
	}
	if linkedErr == nil {
		return linked, nil
	}
	if !os.IsNotExist(linkedErr) {
		return Extension{}, linkedErr
	}
	if installedErr == nil {
		return installed, nil
	}
	if os.IsNotExist(installedErr) {
		return Extension{}, fmt.Errorf("extension %q is not installed or linked", id)
	}
	return Extension{}, installedErr
}

func (s Store) VerifyInstalledRuntime(ext Extension) (string, error) {
	if ext.InstallType != InstallTypeInstalled || ext.Install == nil {
		return "", nil
	}
	runtimePath, err := ResolveRuntime(ext.PackageDir, ext.Manifest.Runtime.Command)
	if err != nil {
		return "", err
	}
	actual, err := hashFile(runtimePath)
	if err != nil {
		return "", err
	}
	if actual != ext.Install.RuntimeHash {
		return "", fmt.Errorf(
			"runtime hash mismatch for %s: expected %s, got %s",
			ext.ID,
			ext.Install.RuntimeHash,
			actual,
		)
	}
	return runtimePath, nil
}

func (s Store) ResolveRuntime(ext Extension) (string, error) {
	switch ext.InstallType {
	case InstallTypeInstalled:
		return s.VerifyInstalledRuntime(ext)
	case InstallTypeLinked:
		return ResolveRuntime(ext.LinkedDir, ext.Manifest.Runtime.Command)
	default:
		return "", fmt.Errorf("unsupported extension install type %q", ext.InstallType)
	}
}

func (s Store) writeCommandCache(id string, ext Extension, now time.Time) error {
	var cacheDir string
	var err error
	switch ext.InstallType {
	case InstallTypeInstalled:
		cacheDir, _, err = s.installPaths(id)
	case InstallTypeLinked:
		cacheDir, err = s.linkDir(id)
	default:
		err = fmt.Errorf("unsupported extension install type %q", ext.InstallType)
	}
	if err != nil {
		return err
	}
	cache := CommandCache{
		SchemaVersion: stateSchemaVersion,
		ID:            id,
		GeneratedAt:   now.UTC().Format(time.RFC3339),
		InstallType:   ext.InstallType,
		Manifest:      ext.Manifest,
		CommandPaths:  ext.CommandPaths,
	}
	return writeJSON(filepath.Join(cacheDir, commandsCacheName), cache)
}

func (s Store) listInstalled() ([]Extension, error) {
	root := filepath.Join(s.root, "installed")
	return s.walkExtensionState(root, s.loadInstalled)
}

func (s Store) listLinked() ([]Extension, error) {
	root := filepath.Join(s.root, "linked")
	return s.walkExtensionState(root, s.loadLinked)
}

func (s Store) walkExtensionState(root string, load func(string) (Extension, error)) ([]Extension, error) {
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	var extensions []Extension
	publishers, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, publisher := range publishers {
		if !publisher.IsDir() {
			continue
		}
		names, err := os.ReadDir(filepath.Join(root, publisher.Name()))
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			if !name.IsDir() {
				continue
			}
			id := ExtensionID(publisher.Name(), name.Name())
			if ValidateExtensionID(id) != nil {
				continue
			}
			ext, err := load(id)
			if err != nil {
				return nil, err
			}
			extensions = append(extensions, ext)
		}
	}
	return extensions, nil
}

func (s Store) loadInstalled(id string) (Extension, error) {
	installDir, packageDir, err := s.installPaths(id)
	if err != nil {
		return Extension{}, err
	}
	if _, err := os.Stat(installDir); err != nil {
		return Extension{}, err
	}
	cache, cacheErr := s.loadCommandCache(id, InstallTypeInstalled)
	var state InstallState
	if err := readJSON(filepath.Join(installDir, installStateName), &state); err != nil {
		return degradedExtension(id, InstallTypeInstalled, cache, ExtensionHealthDamaged,
			"install_state_invalid", fmt.Sprintf("cannot read install state: %v", err),
			"reinstall or uninstall this extension"), nil
	}
	if state.SchemaVersion != stateSchemaVersion {
		return degradedExtension(id, InstallTypeInstalled, cache, ExtensionHealthDamaged,
			"install_state_invalid", fmt.Sprintf("unsupported install state schema_version %d", state.SchemaVersion),
			"reinstall or uninstall this extension"), nil
	}
	if state.ID != id {
		return degradedExtension(id, InstallTypeInstalled, cache, ExtensionHealthDamaged,
			"install_identity_mismatch", fmt.Sprintf("install state id %q does not match path id %q", state.ID, id),
			"reinstall or uninstall this extension"), nil
	}
	manifest, manifestBytes, err := LoadManifestFile(filepath.Join(packageDir, ManifestFileName))
	if err != nil {
		ext := degradedExtension(id, InstallTypeInstalled, cache, ExtensionHealthDamaged,
			"installed_manifest_invalid", fmt.Sprintf("cannot load installed manifest: %v", err),
			"reinstall, upgrade, or uninstall this extension")
		ext.PackageDir = packageDir
		ext.Install = &state
		return ext, nil
	}
	if ExtensionID(manifest.Publisher, manifest.Name) != id || hashBytes(manifestBytes) != state.ManifestHash {
		ext := degradedExtension(id, InstallTypeInstalled, cache, ExtensionHealthDamaged,
			"installed_manifest_modified", "the installed manifest no longer matches the install record",
			"reinstall, upgrade, or uninstall this extension")
		ext.PackageDir = packageDir
		ext.Install = &state
		return ext, nil
	}
	if _, err := ResolveRuntime(packageDir, manifest.Runtime.Command); err != nil {
		ext := degradedExtension(id, InstallTypeInstalled, cache, ExtensionHealthDamaged,
			"installed_runtime_invalid", fmt.Sprintf("cannot use installed runtime: %v", err),
			"reinstall, upgrade, or uninstall this extension")
		ext.PackageDir = packageDir
		ext.Install = &state
		return ext, nil
	}
	if cacheErr != nil {
		_ = s.writeCommandCache(id, Extension{
			ID: id, InstallType: InstallTypeInstalled, Manifest: manifest, CommandPaths: manifest.CommandPaths,
		}, time.Now())
	}
	return Extension{
		ID:           id,
		InstallType:  InstallTypeInstalled,
		Health:       manifestHealth(manifest),
		Manifest:     manifest,
		CommandPaths: manifest.CommandPaths,
		PackageDir:   packageDir,
		Install:      &state,
	}, nil
}

func (s Store) loadLinked(id string) (Extension, error) {
	linkDir, err := s.linkDir(id)
	if err != nil {
		return Extension{}, err
	}
	if _, err := os.Stat(linkDir); err != nil {
		return Extension{}, err
	}
	cache, cacheErr := s.loadCommandCache(id, InstallTypeLinked)
	var state LinkState
	if err := readJSON(filepath.Join(linkDir, linkStateName), &state); err != nil {
		return degradedExtension(id, InstallTypeLinked, cache, ExtensionHealthInvalid,
			"link_state_invalid", fmt.Sprintf("cannot read link state: %v", err),
			fmt.Sprintf("re-link the extension or run `kongctl uninstall extension %s`", id)), nil
	}
	if state.SchemaVersion != stateSchemaVersion {
		ext := degradedExtension(id, InstallTypeLinked, cache, ExtensionHealthInvalid,
			"link_state_invalid", fmt.Sprintf("unsupported link state schema_version %d", state.SchemaVersion),
			fmt.Sprintf("re-link the extension or run `kongctl uninstall extension %s`", id))
		ext.LinkedDir = state.Path
		ext.Link = &state
		return ext, nil
	}
	if state.ID != id {
		ext := degradedExtension(id, InstallTypeLinked, cache, ExtensionHealthInvalid,
			"link_identity_mismatch", fmt.Sprintf("link state id %q does not match path id %q", state.ID, id),
			fmt.Sprintf("re-link the extension or run `kongctl uninstall extension %s`", id))
		ext.LinkedDir = state.Path
		ext.Link = &state
		return ext, nil
	}
	manifest, _, err := LoadManifestFile(filepath.Join(state.Path, ManifestFileName))
	if err != nil {
		status := ExtensionHealthInvalid
		code := "linked_manifest_invalid"
		if os.IsNotExist(err) || os.IsPermission(err) {
			status = ExtensionHealthUnavailable
			code = "linked_source_unavailable"
		}
		ext := degradedExtension(id, InstallTypeLinked, cache, status, code,
			fmt.Sprintf("cannot load linked manifest from %q: %v", state.Path, err),
			fmt.Sprintf("restore the source, re-link it, or run `kongctl uninstall extension %s`", id))
		ext.LinkedDir = state.Path
		ext.Link = &state
		return ext, nil
	}
	if ExtensionID(manifest.Publisher, manifest.Name) != id {
		ext := degradedExtension(id, InstallTypeLinked, cache, ExtensionHealthInvalid,
			"linked_identity_mismatch",
			fmt.Sprintf("linked manifest declares %q instead of %q", ExtensionID(manifest.Publisher, manifest.Name), id),
			fmt.Sprintf("restore the original identity, re-link it, or run `kongctl uninstall extension %s`", id))
		ext.LinkedDir = state.Path
		ext.Link = &state
		return ext, nil
	}
	if _, err := ResolveRuntime(state.Path, manifest.Runtime.Command); err != nil {
		status := ExtensionHealthInvalid
		code := "linked_runtime_invalid"
		if os.IsNotExist(err) || os.IsPermission(err) {
			status = ExtensionHealthUnavailable
			code = "linked_runtime_unavailable"
		}
		ext := Extension{
			ID: id, InstallType: InstallTypeLinked, Manifest: manifest, CommandPaths: manifest.CommandPaths,
			LinkedDir: state.Path, Link: &state,
			Health: unhealthy(status, code, fmt.Sprintf("cannot use linked runtime: %v", err),
				fmt.Sprintf("restore or rebuild the runtime, re-link it, or run `kongctl uninstall extension %s`", id)),
		}
		return ext, nil
	}
	if cacheErr != nil {
		_ = s.writeCommandCache(id, Extension{
			ID: id, InstallType: InstallTypeLinked, Manifest: manifest, CommandPaths: manifest.CommandPaths,
		}, time.Now())
	}
	return Extension{
		ID:           id,
		InstallType:  InstallTypeLinked,
		Health:       manifestHealth(manifest),
		Manifest:     manifest,
		CommandPaths: manifest.CommandPaths,
		LinkedDir:    state.Path,
		Link:         &state,
	}, nil
}

func (s Store) loadCommandCache(id string, installType InstallType) (*CommandCache, error) {
	var cacheDir string
	var err error
	switch installType {
	case InstallTypeInstalled:
		cacheDir, _, err = s.installPaths(id)
	case InstallTypeLinked:
		cacheDir, err = s.linkDir(id)
	default:
		err = fmt.Errorf("unsupported extension install type %q", installType)
	}
	if err != nil {
		return nil, err
	}
	var cache CommandCache
	if err := readJSON(filepath.Join(cacheDir, commandsCacheName), &cache); err != nil {
		return nil, err
	}
	if cache.SchemaVersion != stateSchemaVersion || cache.ID != id || cache.InstallType != installType {
		return nil, fmt.Errorf("command cache metadata does not match extension %q", id)
	}
	if err := NormalizeAndValidateManifest(&cache.Manifest); err != nil {
		return nil, fmt.Errorf("invalid command cache manifest: %w", err)
	}
	if ExtensionID(cache.Manifest.Publisher, cache.Manifest.Name) != id {
		return nil, fmt.Errorf("command cache manifest identity does not match extension %q", id)
	}
	cache.CommandPaths = cache.Manifest.CommandPaths
	return &cache, nil
}

func degradedExtension(
	id string,
	installType InstallType,
	cache *CommandCache,
	status ExtensionHealthStatus,
	code, message, remediation string,
) Extension {
	ext := Extension{
		ID:          id,
		InstallType: installType,
		Health:      unhealthy(status, code, message, remediation),
	}
	if cache != nil {
		ext.Manifest = cache.Manifest
		ext.CommandPaths = cache.CommandPaths
	}
	return ext
}

func unhealthy(status ExtensionHealthStatus, code, message, remediation string) ExtensionHealth {
	return ExtensionHealth{
		Status: status,
		Diagnostics: []ExtensionDiagnostic{{
			Code: code, Message: message, Remediation: remediation,
		}},
	}
}

func manifestHealth(manifest Manifest) ExtensionHealth {
	result, err := CheckCompatibility(manifest, meta.CLIVersion())
	if err == nil && !result.Compatible {
		return unhealthy(
			ExtensionHealthIncompatible,
			"extension_incompatible",
			fmt.Sprintf("requires kongctl %s; current version is %s", result.Constraint, result.CurrentVersion),
			"upgrade the extension or use a compatible kongctl version",
		)
	}
	return ExtensionHealth{Status: ExtensionHealthReady}
}

func (s Store) installPaths(id string) (string, string, error) {
	publisher, name, err := SplitExtensionID(id)
	if err != nil {
		return "", "", err
	}
	installDir := filepath.Join(s.root, "installed", publisher, name)
	return installDir, filepath.Join(installDir, "package"), nil
}

func (s Store) linkDir(id string) (string, error) {
	publisher, name, err := SplitExtensionID(id)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.root, "linked", publisher, name), nil
}

func (s Store) ensureNotInstalled(id string) error {
	installDir, _, err := s.installPaths(id)
	if err != nil {
		return err
	}
	if _, err := os.Stat(installDir); err == nil {
		return fmt.Errorf("extension %q is already installed; uninstall it before linking", id)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s Store) ensureNotLinked(id string) error {
	linkDir, err := s.linkDir(id)
	if err != nil {
		return err
	}
	if _, err := os.Stat(linkDir); err == nil {
		return fmt.Errorf(
			"extension %q is linked; run `kongctl uninstall extension %s` before installing",
			id, id,
		)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func validateLocalExtensionRoot(source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", fmt.Errorf("extension source path is required")
	}
	expanded := os.ExpandEnv(source)
	if strings.HasPrefix(expanded, "~"+string(filepath.Separator)) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		expanded = filepath.Join(home, strings.TrimPrefix(expanded, "~"+string(filepath.Separator)))
	}
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return "", err
	}
	realPath, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(realPath)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("extension source %q must be a directory", source)
	}
	if _, err := os.Stat(filepath.Join(realPath, ManifestFileName)); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("extension source %q does not contain %s", source, ManifestFileName)
		}
		return "", err
	}
	return realPath, nil
}

func copyExtensionTree(source, target string) error {
	source = filepath.Clean(source)
	target = filepath.Clean(target)
	return filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return fmt.Errorf("refusing to copy path outside extension root: %q", path)
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("local extension installs do not support symlinks: %q", rel)
		}
		targetPath := filepath.Join(target, rel)
		if d.IsDir() {
			return os.MkdirAll(targetPath, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported extension package entry %q", rel)
		}
		return copyFile(path, targetPath, info.Mode().Perm())
	})
}

func copyFile(source, target string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func readJSON(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func hashTree(root string) (string, error) {
	var files []string
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported extension package entry %q", path)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		return "", err
	}
	slices.Sort(files)
	hasher := sha256.New()
	for _, rel := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		fmt.Fprintf(hasher, "path:%s\n", rel)
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(hasher, file); err != nil {
			_ = file.Close()
			return "", err
		}
		if err := file.Close(); err != nil {
			return "", err
		}
		fmt.Fprintln(hasher)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func ensureNotInside(sourceRoot, target string) error {
	sourceReal, err := filepath.EvalSymlinks(sourceRoot)
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(targetAbs, sourceReal)
	if err != nil {
		return err
	}
	if rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." && !filepath.IsAbs(rel)) {
		return fmt.Errorf("extension source %q is inside managed install directory %q", sourceRoot, target)
	}
	return nil
}
