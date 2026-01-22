package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kong/kongctl/internal/declarative/resources"
)

func (l *Loader) resolveDeckRequiresPaths(rs *resources.ResourceSet, baseDir string, rootDir string) error {
	if rs == nil {
		return nil
	}
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		baseDir = "."
	}
	baseDirAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("resolve base dir %q: %w", baseDir, err)
	}

	rootDir = strings.TrimSpace(rootDir)
	rootDirAbs := ""
	if rootDir != "" {
		rootDirAbs, err = filepath.Abs(rootDir)
		if err != nil {
			return fmt.Errorf("resolve base dir boundary %q: %w", rootDir, err)
		}
	}

	for i := range rs.GatewayServices {
		svc := &rs.GatewayServices[i]
		svc.SetDeckBaseDir(baseDirAbs)
		if svc.External == nil || svc.External.Requires == nil || svc.External.Requires.Deck == nil {
			continue
		}
		if err := validateDeckFiles(svc.External.Requires.Deck.Files, baseDirAbs, rootDirAbs); err != nil {
			return fmt.Errorf("gateway_service %q deck files: %w", svc.Ref, err)
		}
	}

	return nil
}

func validateDeckFiles(files []string, baseDirAbs string, rootDirAbs string) error {
	if len(files) == 0 {
		return nil
	}

	for _, file := range files {
		value := strings.TrimSpace(file)
		if value == "" {
			continue
		}
		if strings.HasPrefix(value, "-") {
			continue
		}
		if looksLikeURL(value) {
			continue
		}

		candidate := value
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(baseDirAbs, candidate)
		}
		candidate = filepath.Clean(candidate)
		if rootDirAbs != "" && !pathWithinBase(rootDirAbs, candidate) {
			return fmt.Errorf("deck state file resolves outside base dir %s: %s", rootDirAbs, file)
		}
	}

	return nil
}

func pathWithinBase(base string, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	if rel == "." || rel == "" {
		return true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false
	}
	return true
}

func looksLikeURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}
