package resources

import (
	"fmt"
	"strings"
)

// DeckConfig describes deck state files to apply or sync.
type DeckConfig struct {
	Files []string `yaml:"files"           json:"files"`
	Flags []string `yaml:"flags,omitempty" json:"flags,omitempty"`
}

func (d *DeckConfig) Validate() error {
	if d == nil || len(d.Files) == 0 {
		return fmt.Errorf("_deck.files must include at least one state file")
	}

	for i, file := range d.Files {
		value := strings.TrimSpace(file)
		if value == "" {
			return fmt.Errorf("_deck.files[%d] cannot be empty", i)
		}
		if strings.HasPrefix(value, "-") {
			return fmt.Errorf("_deck.files[%d] must be a file path, not a flag", i)
		}
		if strings.Contains(value, "{{kongctl.mode}}") {
			return fmt.Errorf("_deck.files[%d] cannot include {{kongctl.mode}}", i)
		}
	}

	for i, flag := range d.Flags {
		value := strings.TrimSpace(flag)
		if value == "" {
			return fmt.Errorf("_deck.flags[%d] cannot be empty", i)
		}
		if !strings.HasPrefix(value, "-") {
			return fmt.Errorf("_deck.flags[%d] must be a flag", i)
		}
		if strings.Contains(value, "{{kongctl.mode}}") {
			return fmt.Errorf("_deck.flags[%d] cannot include {{kongctl.mode}}", i)
		}
		if deckFlagConflicts(value) {
			return fmt.Errorf("_deck.flags[%d] cannot include %s", i, value)
		}
	}

	return nil
}

func deckFlagConflicts(flag string) bool {
	denied := []string{
		"--konnect-token",
		"--konnect-control-plane-name",
		"--konnect-addr",
		"--json-output",
		"--output",
	}

	for _, candidate := range denied {
		if flag == candidate || strings.HasPrefix(flag, candidate+"=") {
			return true
		}
	}

	return false
}
