package processes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kong/kongctl/internal/config"
)

const (
	defaultDirPerm  = 0o700
	defaultFilePerm = 0o600

	rootDirName = "processes"
	pidToken    = "%PID%"

	// ProcessRecordPathEnv stores the detached process record path template.
	// The `%PID%` token, if present, is replaced by the running process PID.
	ProcessRecordPathEnv = "KONGCTL_PROCESS_RECORD_FILE"
)

// Record describes a detached kongctl process.
type Record struct {
	PID            int       `json:"pid" yaml:"pid"`
	Kind           string    `json:"kind" yaml:"kind"`
	Profile        string    `json:"profile,omitempty" yaml:"profile,omitempty"`
	CreatedAt      time.Time `json:"created_at" yaml:"created_at"`
	LogFile        string    `json:"log_file,omitempty" yaml:"log_file,omitempty"`
	Args           []string  `json:"args,omitempty" yaml:"args,omitempty"`
	StartTimeTicks uint64    `json:"start_time_ticks,omitempty" yaml:"start_time_ticks,omitempty"`
}

// StoredRecord includes the record and backing file path.
type StoredRecord struct {
	Record
	File string `json:"file" yaml:"file"`
}

// ResolveDir returns the detached process registry directory.
func ResolveDir() (string, error) {
	configDir, err := config.GetDefaultConfigPath()
	if err != nil {
		return "", fmt.Errorf("resolve default config path: %w", err)
	}

	return filepath.Join(configDir, rootDirName), nil
}

// ResolvePathTemplate returns the default per-process record path template.
func ResolvePathTemplate() (string, error) {
	dir, err := ResolveDir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dir, defaultDirPerm); err != nil {
		return "", fmt.Errorf("create detached process directory: %w", err)
	}

	return filepath.Join(dir, pidToken+".json"), nil
}

// ResolvePathForPID returns a concrete record path for a PID.
func ResolvePathForPID(pid int) (string, error) {
	template, err := ResolvePathTemplate()
	if err != nil {
		return "", err
	}
	return ResolvePathFromTemplate(template, pid), nil
}

// ResolvePathFromTemplate expands a PID token inside a record path template.
func ResolvePathFromTemplate(template string, pid int) string {
	if strings.TrimSpace(template) == "" {
		return ""
	}
	return strings.ReplaceAll(template, pidToken, fmt.Sprintf("%d", pid))
}

// ResolvePathFromEnv resolves the detached process record path from the
// environment for the provided PID.
func ResolvePathFromEnv(pid int) string {
	template := strings.TrimSpace(os.Getenv(ProcessRecordPathEnv))
	if template == "" {
		return ""
	}
	return ResolvePathFromTemplate(template, pid)
}

// WriteRecord persists a process record atomically at path.
func WriteRecord(path string, record Record) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("process record path is required")
	}
	if record.PID <= 0 {
		return fmt.Errorf("process PID must be greater than zero")
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now().UTC()
	}

	record.Kind = strings.TrimSpace(record.Kind)
	record.Profile = strings.TrimSpace(record.Profile)
	record.LogFile = strings.TrimSpace(record.LogFile)
	record.Args = RedactArgs(record.Args)

	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal process record: %w", err)
	}

	return writeAtomic(path, raw, defaultFilePerm)
}

// RemoveRecordByPath removes a process record file.
func RemoveRecordByPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// RemoveRecordByPID removes the process record for pid.
func RemoveRecordByPID(pid int) error {
	path, err := ResolvePathForPID(pid)
	if err != nil {
		return err
	}
	return RemoveRecordByPath(path)
}

// ListRecords returns all stored detached process records.
func ListRecords() ([]StoredRecord, error) {
	dir, err := ResolveDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	records := make([]StoredRecord, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		record, err := loadRecord(path)
		if err != nil {
			continue
		}
		records = append(records, StoredRecord{
			Record: record,
			File:   path,
		})
	}

	sort.Slice(records, func(i, j int) bool {
		left := records[i].CreatedAt
		right := records[j].CreatedAt
		if !left.Equal(right) {
			return left.After(right)
		}
		return records[i].PID < records[j].PID
	})

	return records, nil
}

func loadRecord(path string) (Record, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Record{}, err
	}

	var record Record
	if err := json.Unmarshal(raw, &record); err != nil {
		return Record{}, err
	}
	if record.PID <= 0 {
		return Record{}, fmt.Errorf("invalid process record PID")
	}

	return record, nil
}

func writeAtomic(path string, payload []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, defaultDirPerm); err != nil {
		return fmt.Errorf("create process record directory: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, ".process-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp process record: %w", err)
	}
	tmpPath := tmpFile.Name()

	cleanup := func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
	}

	if _, err := tmpFile.Write(payload); err != nil {
		cleanup()
		return fmt.Errorf("write temp process record: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temp process record: %w", err)
	}
	if err := tmpFile.Chmod(perm); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp process record: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp process record: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace process record: %w", err)
	}

	return nil
}
