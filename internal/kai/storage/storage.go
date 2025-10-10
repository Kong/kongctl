package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kong/kongctl/internal/config"
)

const (
	defaultDirPerm  = 0o700
	defaultFilePerm = 0o600

	metadataFileName   = "metadata.json"
	transcriptFileName = "transcript.jsonl"
)

// Options describe the properties known at the time a recorder is created.
type Options struct {
	SessionName    string
	SessionCreated time.Time
	CLIVersion     string
}

// Metadata captures high-level information about a stored session.
type Metadata struct {
	SessionID        string     `json:"session_id"`
	SessionName      string     `json:"session_name,omitempty"`
	SessionCreatedAt *time.Time `json:"session_created_at,omitempty"`
	RecorderCreated  time.Time  `json:"recorder_created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	EventCount       int64      `json:"event_count"`
	CLIVersion       string     `json:"cli_version,omitempty"`
}

// EventKind enumerates the supported transcript event types.
type EventKind string

const (
	EventKindLifecycle EventKind = "lifecycle"
	EventKindMessage   EventKind = "message"
	EventKindTask      EventKind = "task"
	EventKindTaskState EventKind = "task_state"
	EventKindError     EventKind = "error"
)

// TaskEvent captures details related to Kai task proposals and updates.
type TaskEvent struct {
	ID            string         `json:"id,omitempty"`
	Type          string         `json:"type,omitempty"`
	Status        string         `json:"status,omitempty"`
	Action        string         `json:"action,omitempty"`
	Message       string         `json:"message,omitempty"`
	Confirmation  string         `json:"confirmation,omitempty"`
	Error         string         `json:"error,omitempty"`
	Context       []ContextRef   `json:"context,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	AnalysisReady bool           `json:"analysis_ready,omitempty"`
}

// ContextRef references a Kai entity shared with the agent.
type ContextRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// Event represents a single line within the transcript.
type Event struct {
	Sequence  int64          `json:"sequence"`
	Timestamp time.Time      `json:"timestamp"`
	Kind      EventKind      `json:"kind"`
	Role      string         `json:"role,omitempty"`
	Content   string         `json:"content,omitempty"`
	Duration  time.Duration  `json:"duration,omitempty"`
	Task      *TaskEvent     `json:"task,omitempty"`
	Context   []ContextRef   `json:"context,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// SessionRecorder manages on-disk persistence for a Kai session.
type SessionRecorder struct {
	sessionID string
	version   string
	dir       string

	metaPath       string
	transcriptPath string

	mu       sync.Mutex
	metadata Metadata
}

// NewSessionRecorder constructs or loads a recorder for the provided session.
func NewSessionRecorder(sessionID string, opts Options) (*SessionRecorder, error) {
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" {
		return nil, errors.New("session id cannot be empty")
	}
	safeID := sanitizeComponent(trimmed)

	baseDir, err := config.GetDefaultConfigPath()
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}
	sessionDir := filepath.Join(baseDir, "kai", "sessions", safeID)
	if err := os.MkdirAll(sessionDir, defaultDirPerm); err != nil {
		return nil, fmt.Errorf("create session directory: %w", err)
	}

	r := &SessionRecorder{
		sessionID:      trimmed,
		version:        strings.TrimSpace(opts.CLIVersion),
		dir:            sessionDir,
		metaPath:       filepath.Join(sessionDir, metadataFileName),
		transcriptPath: filepath.Join(sessionDir, transcriptFileName),
	}

	meta, err := r.loadMetadata()
	if err != nil {
		return nil, err
	}

	if meta.SessionID == "" {
		meta.SessionID = trimmed
	}
	if meta.RecorderCreated.IsZero() {
		meta.RecorderCreated = time.Now().UTC()
	}
	if opts.SessionName != "" {
		meta.SessionName = opts.SessionName
	}
	if !opts.SessionCreated.IsZero() {
		created := opts.SessionCreated.UTC()
		meta.SessionCreatedAt = &created
	}
	if r.version != "" {
		meta.CLIVersion = r.version
	}
	if meta.UpdatedAt.IsZero() {
		meta.UpdatedAt = meta.RecorderCreated
	}

	r.metadata = meta
	if err := r.saveMetadata(meta); err != nil {
		return nil, err
	}

	return r, nil
}

// SessionID returns the identifier associated with the recorder.
func (r *SessionRecorder) SessionID() string {
	if r == nil {
		return ""
	}
	return r.sessionID
}

// Directory exposes the path used for session persistence.
func (r *SessionRecorder) Directory() string {
	if r == nil {
		return ""
	}
	return r.dir
}

// SetSessionInfo updates the stored metadata with the latest view of the session details.
func (r *SessionRecorder) SetSessionInfo(name string, createdAt time.Time) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	updated := false
	if trimmed := strings.TrimSpace(name); trimmed != "" && trimmed != r.metadata.SessionName {
		r.metadata.SessionName = trimmed
		updated = true
	}
	if !createdAt.IsZero() {
		utc := createdAt.UTC()
		if r.metadata.SessionCreatedAt == nil || !r.metadata.SessionCreatedAt.Equal(utc) {
			r.metadata.SessionCreatedAt = &utc
			updated = true
		}
	}

	if !updated {
		return nil
	}
	r.metadata.UpdatedAt = time.Now().UTC()
	return r.saveMetadataLocked()
}

// AppendEvent appends a new transcript line and updates metadata atomically.
func (r *SessionRecorder) AppendEvent(evt Event) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	if evt.Timestamp.IsZero() {
		evt.Timestamp = now
	} else {
		evt.Timestamp = evt.Timestamp.UTC()
	}
	if evt.Kind == "" {
		return errors.New("event kind cannot be empty")
	}
	if evt.Metadata == nil {
		evt.Metadata = map[string]any{}
	}
	if evt.Duration < 0 {
		evt.Duration = 0
	}

	r.metadata.EventCount++
	evt.Sequence = r.metadata.EventCount

	payload, err := json.Marshal(evt)
	if err != nil {
		r.metadata.EventCount--
		return fmt.Errorf("marshal event: %w", err)
	}

	if err := appendLine(r.transcriptPath, payload); err != nil {
		r.metadata.EventCount--
		return err
	}

	r.metadata.UpdatedAt = evt.Timestamp
	return r.saveMetadataLocked()
}

// RecordError logs an error event in the transcript.
func (r *SessionRecorder) RecordError(message string, attrs map[string]any) error {
	if message == "" {
		return nil
	}
	return r.AppendEvent(Event{
		Kind:     EventKindError,
		Error:    message,
		Metadata: attrs,
	})
}

// loadMetadata reads previously stored metadata, if present.
func (r *SessionRecorder) loadMetadata() (Metadata, error) {
	raw, err := os.ReadFile(r.metaPath)
	if errors.Is(err, os.ErrNotExist) {
		return Metadata{}, nil
	}
	if err != nil {
		return Metadata{}, fmt.Errorf("read metadata: %w", err)
	}

	var meta Metadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return Metadata{}, fmt.Errorf("decode metadata: %w", err)
	}
	return meta, nil
}

func (r *SessionRecorder) saveMetadata(meta Metadata) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metadata = meta
	return r.saveMetadataLocked()
}

func (r *SessionRecorder) saveMetadataLocked() error {
	raw, err := json.MarshalIndent(r.metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("encode metadata: %w", err)
	}
	if err := writeAtomic(r.metaPath, raw, defaultFilePerm); err != nil {
		return err
	}
	return nil
}

func appendLine(path string, payload []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), defaultDirPerm); err != nil {
		return fmt.Errorf("ensure transcript directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, defaultFilePerm)
	if err != nil {
		return fmt.Errorf("open transcript: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("write transcript: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync transcript: %w", err)
	}
	return nil
}

func writeAtomic(path string, payload []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, defaultDirPerm); err != nil {
		return fmt.Errorf("ensure metadata directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".metadata-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp metadata: %w", err)
	}
	defer func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}()

	if _, err := tmp.Write(payload); err != nil {
		return fmt.Errorf("write temp metadata: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp metadata: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp metadata: %w", err)
	}
	if err := os.Chmod(tmp.Name(), perm); err != nil {
		return fmt.Errorf("chmod metadata: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replace metadata: %w", err)
	}
	return nil
}

func sanitizeComponent(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "session"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	output := strings.Trim(b.String(), "_")
	if output == "" {
		return "session"
	}
	return output
}
