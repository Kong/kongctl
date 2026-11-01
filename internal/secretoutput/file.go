// Package secretoutput captures sensitive operation results in private files.
package secretoutput

import (
	"encoding/json"
	"fmt"
	"os"
)

// Metadata identifies the remote record without containing credential material.
type Metadata struct {
	ResourceType  string `json:"resource_type,omitempty"`
	ResourceRef   string `json:"resource_ref,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
	ParentRef     string `json:"parent_ref,omitempty"`
	AuthServerID  string `json:"auth_server_id,omitempty"`
	ClientID      string `json:"client_id,omitempty"`
	SecretID      string `json:"secret_id,omitempty"`
	DirectoryID   string `json:"directory_id,omitempty"`
	PrincipalID   string `json:"principal_id,omitempty"`
	GroupID       string `json:"group_id,omitempty"`
	APIKeyID      string `json:"api_key_id,omitempty"`
	BasicAuthID   string `json:"basic_auth_id,omitempty"`
	PasswordID    string `json:"password_id,omitempty"`
	ProviderID    string `json:"provider_id,omitempty"`
	CredentialID  string `json:"credential_id,omitempty"`
	DryRun        *bool  `json:"dry_run,omitempty"`
	RequestSHA256 string `json:"request_sha256,omitempty"`
}

type record struct {
	Metadata
	Status        string          `json:"status"`
	Secret        string          `json:"secret,omitempty"`
	EnrollmentURL string          `json:"enrollment_url,omitempty"`
	Report        json.RawMessage `json:"report,omitempty"`
	ResponseBody  []byte          `json:"response_body_base64,omitempty"`
}

// File reserves a new output path before a remote create can issue a secret.
type File struct {
	file *os.File
	path string
	kind string
}

// Path identifies the reserved file without reading captured material.
func (f *File) Path() string {
	if f == nil {
		return ""
	}
	return f.path
}

// Reserve refuses existing files and symlinks. The initial record allows a user
// to identify the parent if a remote request has an uncertain outcome.
func Reserve(path string, metadata Metadata) (*File, error) {
	return reserve(path, metadata, "secret output", "creation_not_confirmed")
}

// ReserveReport uses the same exclusive private-file guarantees for operation
// reports whose diagnostics can contain personal or otherwise sensitive data.
func ReserveReport(path string, metadata Metadata) (*File, error) {
	return reserve(path, metadata, "report output", "request_not_confirmed")
}

func reserve(path string, metadata Metadata, kind, status string) (*File, error) {
	if path == "" {
		return nil, fmt.Errorf("%s file is required", kind)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("reserve %s file: %w", kind, err)
	}
	output := &File{file: file, path: path, kind: kind}
	if err := output.write(record{Metadata: metadata, Status: status}); err != nil {
		owned := output.checkPath() == nil
		_ = file.Close()
		if owned {
			_ = os.Remove(path)
		}
		return nil, fmt.Errorf("prepare %s file: %w", kind, err)
	}
	return output, nil
}

// CaptureReport preserves a complete response before interpreting its result.
// Non-JSON bodies are stored as base64 rather than discarded on parse failure.
func (f *File) CaptureReport(metadata Metadata, body []byte) error {
	value := record{Metadata: metadata, Status: "captured"}
	valid := json.Valid(body)
	if valid {
		value.Report = body
	} else {
		value.Status, value.ResponseBody = "response_not_json", body
	}
	if err := f.write(value); err != nil {
		return fmt.Errorf("persist report output: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close report output: %w", err)
	}
	return nil
}

// Capture persists material once. Failures retain the file and never trigger a
// second remote create or delete a credential that might already be in use.
func (f *File) Capture(metadata Metadata, value string) error {
	status := "captured"
	if value == "" {
		status = "secret_missing"
	}
	if err := f.write(record{Metadata: metadata, Status: status, Secret: value}); err != nil {
		return fmt.Errorf("persist secret output: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close secret output: %w", err)
	}
	if value == "" {
		return fmt.Errorf("creation response contained no secret value")
	}
	return nil
}

// CaptureEnrollmentURL persists an issued OAuth capability exclusively in the
// reserved private file. Captured means the URL was saved, not consent completed.
func (f *File) CaptureEnrollmentURL(metadata Metadata, value string) error {
	status := "captured"
	if value == "" {
		status = "enrollment_url_missing"
	}
	if err := f.write(record{Metadata: metadata, Status: status, EnrollmentURL: value}); err != nil {
		return fmt.Errorf("persist enrollment output: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close enrollment output: %w", err)
	}
	if value == "" {
		return fmt.Errorf("enrollment response contained no URL")
	}
	return nil
}

// RecordPending fills identities resolved after reservation, before the POST.
func (f *File) RecordPending(metadata Metadata) error {
	return f.write(record{Metadata: metadata, Status: "creation_not_confirmed"})
}

func (f *File) write(value record) error {
	if f == nil || f.file == nil {
		return fmt.Errorf("private output is closed")
	}
	if err := f.checkPath(); err != nil {
		return err
	}
	//nolint:gosec // G117: Deliberate credential capture in an exclusively reserved 0600 file.
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if _, err := f.file.Seek(0, 0); err != nil {
		return err
	}
	if err := f.file.Truncate(0); err != nil {
		return err
	}
	if _, err := f.file.Write(append(encoded, '\n')); err != nil {
		return err
	}
	if err := f.file.Sync(); err != nil {
		return err
	}
	return f.checkPath()
}

func (f *File) checkPath() error {
	current, err := os.Lstat(f.path)
	if err != nil {
		return fmt.Errorf("%s path is no longer available: %w", f.kind, err)
	}
	opened, err := f.file.Stat()
	if err != nil {
		return err
	}
	if !current.Mode().IsRegular() || !os.SameFile(current, opened) {
		return fmt.Errorf("%s path was replaced", f.kind)
	}
	return nil
}

func (f *File) Close() error {
	if f.file == nil {
		return nil
	}
	err := f.file.Close()
	f.file = nil
	return err
}

// Discard removes a reservation before any remote request was attempted.
func (f *File) Discard() error {
	if f == nil || f.file == nil {
		return nil
	}
	if err := f.checkPath(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Remove(f.path)
}
