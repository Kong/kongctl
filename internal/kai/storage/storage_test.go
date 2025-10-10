package storage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewSessionRecorderInitializesMetadata(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", temp)

	createdAt := time.Date(2024, time.January, 2, 15, 4, 5, 0, time.UTC)
	recorder, err := NewSessionRecorder("session-abc", Options{
		SessionName:    "Test Session",
		SessionCreated: createdAt,
		CLIVersion:     "1.2.3",
	})
	if err != nil {
		t.Fatalf("NewSessionRecorder() error = %v", err)
	}
	if recorder == nil {
		t.Fatalf("recorder is nil")
	}

	expectedDir := filepath.Join(temp, "kongctl", "kai", "sessions", "session-abc")
	if got := recorder.Directory(); got != expectedDir {
		t.Fatalf("Directory() = %s, want %s", got, expectedDir)
	}

	raw, err := os.ReadFile(filepath.Join(expectedDir, "metadata.json"))
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	var meta Metadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta.SessionID != "session-abc" {
		t.Errorf("SessionID = %s, want %s", meta.SessionID, "session-abc")
	}
	if meta.SessionName != "Test Session" {
		t.Errorf("SessionName = %s, want %s", meta.SessionName, "Test Session")
	}
	if meta.CLIVersion != "1.2.3" {
		t.Errorf("CLIVersion = %s, want 1.2.3", meta.CLIVersion)
	}
	if meta.EventCount != 0 {
		t.Errorf("EventCount = %d, want 0", meta.EventCount)
	}
	if meta.SessionCreatedAt == nil || !meta.SessionCreatedAt.Equal(createdAt) {
		t.Fatalf("SessionCreatedAt = %v, want %v", meta.SessionCreatedAt, createdAt)
	}
}

func TestSessionRecorderAppendEventUpdatesTranscript(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", temp)

	recorder, err := NewSessionRecorder("session-xyz", Options{})
	if err != nil {
		t.Fatalf("NewSessionRecorder() error = %v", err)
	}

	event := Event{Kind: EventKindMessage, Role: "user", Content: "hello"}
	if err := recorder.AppendEvent(event); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	event2 := Event{Kind: EventKindMessage, Role: "agent", Content: "hi"}
	if err := recorder.AppendEvent(event2); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	transcript := filepath.Join(recorder.Directory(), "transcript.jsonl")
	file, err := os.Open(transcript)
	if err != nil {
		t.Fatalf("open transcript: %v", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan transcript: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 transcript lines, got %d", len(lines))
	}

	var recorded Event
	if err := json.Unmarshal([]byte(lines[0]), &recorded); err != nil {
		t.Fatalf("unmarshal transcript line 1: %v", err)
	}
	if recorded.Sequence != 1 || recorded.Role != "user" || recorded.Content != "hello" {
		t.Fatalf("unexpected first event: %+v", recorded)
	}

	raw, err := os.ReadFile(filepath.Join(recorder.Directory(), "metadata.json"))
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	var meta Metadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta.EventCount != 2 {
		t.Fatalf("EventCount = %d, want 2", meta.EventCount)
	}
	if meta.UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt is zero")
	}
}

func TestSetSessionInfoNoChangeDoesNotUpdate(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", temp)

	createdAt := time.Now().UTC().Add(-time.Minute)
	recorder, err := NewSessionRecorder("session-name", Options{
		SessionName:    "Session",
		SessionCreated: createdAt,
	})
	if err != nil {
		t.Fatalf("NewSessionRecorder() error = %v", err)
	}

	metaPath := filepath.Join(recorder.Directory(), "metadata.json")
	before, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	if err := recorder.SetSessionInfo("Session", createdAt); err != nil {
		t.Fatalf("SetSessionInfo() error = %v", err)
	}

	after, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	if !bytes.Equal(before, after) {
		t.Fatal("metadata changed despite identical update")
	}

	if err := recorder.SetSessionInfo("Session Renamed", createdAt); err != nil {
		t.Fatalf("SetSessionInfo() rename error = %v", err)
	}

	raw, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if bytes.Equal(raw, before) {
		t.Fatal("metadata should change after rename")
	}

	var meta Metadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta.SessionName != "Session Renamed" {
		t.Fatalf("SessionName = %s, want Session Renamed", meta.SessionName)
	}
}

func TestSanitizeSessionID(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", temp)

	recorder, err := NewSessionRecorder("../unsafe/session", Options{})
	if err != nil {
		t.Fatalf("NewSessionRecorder() error = %v", err)
	}

	expectedSuffix := filepath.Join("kongctl", "kai", "sessions", "unsafe_session")
	if !strings.HasSuffix(recorder.Directory(), expectedSuffix) {
		t.Fatalf("unexpected directory: %s, want suffix %s", recorder.Directory(), expectedSuffix)
	}
}
