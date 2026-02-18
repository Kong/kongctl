package auditlogs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/kong/kongctl/internal/config"
)

const (
	defaultDirPerm  = 0o700
	defaultFilePerm = 0o600

	rootDirName              = "audit-logs"
	eventsFileName           = "events.jsonl"
	listenerStateFileName    = "listener.json"
	destinationStateFileName = "destination.json"
)

// Paths contains filesystem locations used by audit-log commands.
type Paths struct {
	BaseDir              string `json:"base_dir"`
	EventsFile           string `json:"events_file"`
	ListenerStateFile    string `json:"listener_state_file"`
	DestinationStateFile string `json:"destination_state_file"`
}

// Store appends audit-log events to a JSONL file.
type Store struct {
	path string
	mu   sync.Mutex
}

// ResolvePaths returns profile-scoped storage locations for audit logs.
func ResolvePaths(profile string) (Paths, error) {
	configDir, err := config.GetDefaultConfigPath()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve default config path: %w", err)
	}

	safeProfile := sanitizePathComponent(profile)
	baseDir := filepath.Join(configDir, rootDirName, safeProfile)

	return Paths{
		BaseDir:              baseDir,
		EventsFile:           filepath.Join(baseDir, eventsFileName),
		ListenerStateFile:    filepath.Join(baseDir, listenerStateFileName),
		DestinationStateFile: filepath.Join(baseDir, destinationStateFileName),
	}, nil
}

// NewStore creates a JSONL event store at the provided path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Path returns the configured output path.
func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Append writes one or more payload records as JSONL lines.
// The payload is split into newline-delimited records, blank lines are ignored.
func (s *Store) Append(payload []byte) (int, error) {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return 0, fmt.Errorf("event store path is not configured")
	}

	records := SplitPayloadRecords(payload)
	return s.AppendRecords(records)
}

// AppendRecords writes pre-split payload records as JSONL lines.
func (s *Store) AppendRecords(records [][]byte) (int, error) {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return 0, fmt.Errorf("event store path is not configured")
	}
	if len(records) == 0 {
		return 0, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), defaultDirPerm); err != nil {
		return 0, fmt.Errorf("create audit-log directory: %w", err)
	}

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, defaultFilePerm)
	if err != nil {
		return 0, fmt.Errorf("open audit-log file: %w", err)
	}
	defer f.Close()

	written := 0
	for _, record := range records {
		if _, err := f.Write(record); err != nil {
			return written, fmt.Errorf("write audit-log event: %w", err)
		}
		if _, err := f.Write([]byte{'\n'}); err != nil {
			return written, fmt.Errorf("write audit-log event: %w", err)
		}
		written++
	}
	if err := f.Sync(); err != nil {
		return written, fmt.Errorf("sync audit-log event file: %w", err)
	}

	return written, nil
}

// WriteState writes listener metadata atomically.
func WriteState(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal listener state: %w", err)
	}
	return writeAtomic(path, raw, defaultFilePerm)
}

// SplitPayloadRecords splits a payload into newline-delimited records.
// Empty or whitespace-only lines are discarded.
func SplitPayloadRecords(payload []byte) [][]byte {
	trimmedPayload := bytes.TrimSpace(payload)
	if len(trimmedPayload) == 0 {
		return nil
	}

	lines := bytes.Split(trimmedPayload, []byte{'\n'})
	records := make([][]byte, 0, len(lines))
	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		trimmed = bytes.TrimSuffix(trimmed, []byte{'\r'})
		if len(trimmed) == 0 {
			continue
		}
		records = append(records, append([]byte(nil), trimmed...))
	}

	return records
}

func sanitizePathComponent(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "default"
	}

	var b strings.Builder
	for _, r := range trimmed {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}

	safe := b.String()
	if safe == "" {
		return "default"
	}
	return safe
}

func writeAtomic(path string, payload []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, defaultDirPerm); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, ".auditlogs-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp state file: %w", err)
	}
	tmpPath := tmpFile.Name()

	cleanup := func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}

	if _, err := tmpFile.Write(payload); err != nil {
		cleanup()
		return fmt.Errorf("write temp state file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temp state file: %w", err)
	}
	if err := tmpFile.Chmod(perm); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp state file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp state file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace state file: %w", err)
	}

	return nil
}
