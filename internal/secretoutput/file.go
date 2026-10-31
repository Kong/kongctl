// Package secretoutput captures newly issued credentials in private local files.
package secretoutput

import (
	"encoding/json"
	"fmt"
	"os"
)

// Metadata identifies the remote record without containing credential material.
type Metadata struct {
	AuthServerID string `json:"auth_server_id"`
	ClientID     string `json:"client_id,omitempty"`
	SecretID     string `json:"secret_id,omitempty"`
}

type record struct {
	Metadata
	Status string `json:"status"`
	Secret string `json:"secret,omitempty"`
}

// File reserves a new output path before a remote create can issue a secret.
type File struct {
	file *os.File
	path string
}

// Reserve refuses existing files and symlinks. The initial record allows a user
// to identify the parent if a remote request has an uncertain outcome.
func Reserve(path string, metadata Metadata) (*File, error) {
	if path == "" {
		return nil, fmt.Errorf("secret output file is required")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("reserve secret output file: %w", err)
	}
	output := &File{file: file, path: path}
	if err := output.write(record{Metadata: metadata, Status: "creation_not_confirmed"}); err != nil {
		owned := output.checkPath() == nil
		_ = file.Close()
		if owned {
			_ = os.Remove(path)
		}
		return nil, fmt.Errorf("prepare secret output file: %w", err)
	}
	return output, nil
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

func (f *File) write(value record) error {
	if f == nil || f.file == nil {
		return fmt.Errorf("secret output is closed")
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
		return fmt.Errorf("secret output path is no longer available: %w", err)
	}
	opened, err := f.file.Stat()
	if err != nil {
		return err
	}
	if !current.Mode().IsRegular() || !os.SameFile(current, opened) {
		return fmt.Errorf("secret output path was replaced")
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
