package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kong/kongctl/internal/declarative/constants"
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
		if svc.External == nil || svc.External.Requires == nil {
			continue
		}
		for j := range svc.External.Requires.Deck {
			step := &svc.External.Requires.Deck[j]
			if err := validateDeckStepArgs(step.Args, baseDirAbs, rootDirAbs); err != nil {
				return fmt.Errorf("gateway_service %q deck step %d: %w", svc.Ref, j, err)
			}
		}
	}

	return nil
}

func validateDeckStepArgs(args []string, baseDirAbs string, rootDirAbs string) error {
	if len(args) == 0 {
		return nil
	}

	startIdx := 2
	if len(args) < startIdx {
		startIdx = len(args)
	}

	for i := startIdx; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		if arg == "" {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		if arg == "-" || strings.Contains(arg, constants.DeckModePlaceholder) {
			continue
		}
		if looksLikeURL(arg) {
			continue
		}

		candidate := arg
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(baseDirAbs, candidate)
		}
		candidate = filepath.Clean(candidate)
		if rootDirAbs != "" && !pathWithinBase(rootDirAbs, candidate) {
			return fmt.Errorf("deck state file resolves outside base dir %s: %s", rootDirAbs, arg)
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
