package executor

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/kong/kongctl/internal/declarative/planner"
	"github.com/kong/kongctl/internal/secretoutput"
)

type generatedClientOutput struct {
	file      *secretoutput.File
	path      string
	metadata  secretoutput.Metadata
	attempted bool
}

func (e *Executor) prepareClientSecretOutputs(plan *planner.Plan) (resultErr error) {
	e.generatedOutputs = make(map[string]*generatedClientOutput)
	defer func() {
		if resultErr != nil {
			e.discardUnattemptedClientOutputs()
		}
	}()
	if e.dryRun {
		return nil
	}
	for _, change := range plan.Changes {
		if change.ResourceType != planner.ResourceTypeIdentityAuthServerClient ||
			change.Action != planner.ActionCreate {
			continue
		}
		if e.secretOutputDir == "" {
			return fmt.Errorf("creating OAuth clients requires --secret-output-dir for initial generated secrets")
		}
		if err := os.MkdirAll(e.secretOutputDir, 0o700); err != nil {
			return fmt.Errorf("prepare initial-secret output directory: %w", err)
		}
		directory, err := filepath.Abs(e.secretOutputDir)
		if err != nil {
			return err
		}
		parent := ""
		if change.Parent != nil {
			parent = change.Parent.Ref
		}
		name, _ := change.Fields[planner.FieldName].(string)
		digest := sha256.Sum256(
			[]byte(change.Namespace + "\x00" + parent + "\x00" + change.ResourceRef + "\x00" + name),
		)
		path := filepath.Join(directory, fmt.Sprintf("identity-client-%x.json", digest[:12]))
		metadata := secretoutput.Metadata{
			ResourceType: change.ResourceType, ResourceRef: change.ResourceRef,
			Namespace: change.Namespace, ParentRef: parent,
		}
		if change.Parent != nil && !unresolvedReferenceID(change.Parent.ID) {
			metadata.AuthServerID = change.Parent.ID
		}
		file, err := secretoutput.Reserve(path, metadata)
		if err != nil {
			return fmt.Errorf("reserve initial secret for client %q: %w", change.ResourceRef, err)
		}
		e.generatedOutputs[change.ID] = &generatedClientOutput{file: file, path: path, metadata: metadata}
	}
	return nil
}

func (e *Executor) discardUnattemptedClientOutputs() {
	for _, output := range e.generatedOutputs {
		if !output.attempted {
			_ = output.file.Discard()
		} else {
			_ = output.file.Close()
		}
	}
}

func (e *Executor) initialClientOutput(change *planner.PlannedChange) (*generatedClientOutput, error) {
	if change == nil {
		return nil, fmt.Errorf("client creation requires an execution context")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	output := e.generatedOutputs[change.ID]
	if output == nil {
		return nil, fmt.Errorf("client %q has no reserved initial-secret output", change.ResourceRef)
	}
	output.attempted = true
	return output, nil
}

func (e *Executor) finishClientSecretOutputs(result *ExecutionResult) {
	for _, output := range e.generatedOutputs {
		if output.attempted {
			_ = output.file.Close()
			result.GeneratedFiles = append(result.GeneratedFiles, output.path)
		} else if err := output.file.Discard(); err != nil {
			result.Errors = append(result.Errors, ExecutionError{
				Error: fmt.Sprintf("discard unused capture reservation: %v", err),
			})
			result.FailureCount++
		}
	}
	slices.Sort(result.GeneratedFiles)
}
