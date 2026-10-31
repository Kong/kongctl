//go:build e2e

package scenario

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kong/kongctl/internal/declarative/resources"
	"github.com/kong/kongctl/test/e2e/harness"
	"github.com/stretchr/testify/require"
)

func cleanupFixtureCLI(t *testing.T) (*harness.CLI, string) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is required")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "kongctl-fixture")
	//nolint:gosec // Executable local CLI fixture.
	require.NoError(t, os.WriteFile(bin, []byte(cleanupFixture), 0o700))
	return &harness.CLI{
		BinPath: bin, TestDir: dir, Timeout: 5 * time.Second,
		Env: append(os.Environ(), "CLEANUP_STATE="+dir),
	}, dir
}

func TestResourceCleanupAfterFailedScenario(t *testing.T) {
	cli, dir := cleanupFixtureCLI(t)
	var generated string
	t.Run("interrupted-run", func(t *testing.T) {
		vars := map[string]any{}
		specs := []CleanupResource{
			{
				Resource: string(resources.ResourceTypeIdentityDirectory),
				NameVar:  "directory",
				NameEnv:  "TEST_DIRECTORY",
			},
			{Resource: string(resources.ResourceTypeControlPlane), NameVar: "gateway", NameEnv: "TEST_GATEWAY"},
		}
		require.NoError(t, registerResourceCleanup(t, cli, specs, vars))
		generated = vars["directory"].(string)
		require.True(t, strings.HasPrefix(generated, "kongctl-e2e-identity-directory-"))
		observed := []map[string]any{
			{"id": "c0b37b67-06c8-49fb-b51b-2c8f49b20fb1", "name": generated},
			{"id": "c0b37b67-06c8-49fb-b51b-2c8f49b20fb2", "name": vars["gateway"].(string)},
			{"id": "c0b37b67-06c8-49fb-b51b-2c8f49b20fb3", "name": "pre-existing-directory"},
			{
				"id": "c0b37b67-06c8-49fb-b51b-2c8f49b20fb4", "kind": "principal",
				"directory_id": "c0b37b67-06c8-49fb-b51b-2c8f49b20fb1",
				"labels":       map[string]string{"e2e-fixture": "c0b37b67-06c8-49fb-b51b-2c8f49b20fb1"},
			},
			{
				"id": "c0b37b67-06c8-49fb-b51b-2c8f49b20fb5", "name": "foreign-principal",
				"kind": "principal", "directory_id": "unrelated-directory",
			},
		}
		data, err := json.Marshal(observed)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "resources.json"), data, 0o600))
		s := Scenario{Vars: vars, Steps: []Step{{
			Name: "failure", SkipInputs: true,
			Commands: []Command{{Name: "fail", Run: []string{"fail"}}},
		}}}
		diagnostics := newScenarioDiagnostics(filepath.Join(dir, "diagnostics.json"), "cleanup-test")
		require.Error(t, executeScenario(t, "cleanup-test", s, cli, diagnostics))
	})
	data, err := os.ReadFile(filepath.Join(dir, "resources.json"))
	require.NoError(t, err)
	require.NotContains(t, string(data), generated)
	require.Contains(t, string(data), "pre-existing-directory")
	require.Contains(t, string(data), "foreign-principal")
	deleted, err := os.ReadFile(filepath.Join(dir, "deleted"))
	require.NoError(t, err)
	require.Equal(t, "/v2/directories/c0b37b67-06c8-49fb-b51b-2c8f49b20fb1/"+
		"principals/c0b37b67-06c8-49fb-b51b-2c8f49b20fb4\n"+
		"/v2/directories/c0b37b67-06c8-49fb-b51b-2c8f49b20fb1?force=false\n"+
		"/v2/control-planes/c0b37b67-06c8-49fb-b51b-2c8f49b20fb2\n", string(deleted))
}

func TestResourceCleanupRejectsAmbiguousAndIncompleteObservations(t *testing.T) {
	for _, fault := range []string{"read-error", "duplicate", "missing-id"} {
		t.Run(fault, func(t *testing.T) {
			cli, dir := cleanupFixtureCLI(t)
			cli.WithEnv(map[string]string{"CLEANUP_FAULT": fault})
			require.NoError(t, os.WriteFile(filepath.Join(dir, "resources.json"),
				[]byte(`[{"id":"c0b37b67-06c8-49fb-b51b-2c8f49b20fb1","name":"generated"}]`), 0o600))
			require.Error(t, cleanupResource(t.Context(), cli,
				cleanupTarget{kind: resources.ResourceTypeIdentityDirectory, name: "generated"}))
			_, err := os.Stat(filepath.Join(dir, "deleted"))
			require.True(t, os.IsNotExist(err), "incomplete observations must never authorize deletion")
		})
	}
}

func TestResourceCleanupConfiguration(t *testing.T) {
	cli, _ := cleanupFixtureCLI(t)
	for _, specs := range [][]CleanupResource{
		{{Resource: "portal", NameVar: "name", NameEnv: "NAME"}},
		{{Resource: string(resources.ResourceTypeIdentityDirectory), NameVar: "", NameEnv: "NAME"}},
		{{Resource: string(resources.ResourceTypeIdentityDirectory), NameVar: "name", NameEnv: "bad-name"}},
		{{Resource: string(resources.ResourceTypeIdentityDirectory), NameVar: "taken", NameEnv: "NAME"}},
	} {
		require.Error(t, registerResourceCleanup(t, cli, specs, map[string]any{"taken": "existing"}))
	}
	require.NoError(t, registerResourceCleanup(t, cli, nil, nil), "unconfigured scenarios remain unchanged")
}

func TestResourceCleanupPreservesUnmarkedPrincipalInTestDirectory(t *testing.T) {
	cli, dir := cleanupFixtureCLI(t)
	data := `[
        {"id":"c0b37b67-06c8-49fb-b51b-2c8f49b20fb1","name":"generated"},
        {"id":"c0b37b67-06c8-49fb-b51b-2c8f49b20fb2","kind":"principal",
         "directory_id":"c0b37b67-06c8-49fb-b51b-2c8f49b20fb1","labels":{}}
    ]`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "resources.json"), []byte(data), 0o600))
	target := cleanupTarget{kind: resources.ResourceTypeIdentityDirectory, name: "generated"}
	err := cleanupResource(t.Context(), cli, target)
	require.ErrorContains(t, err, "unmarked principal")
	_, err = os.Stat(filepath.Join(dir, "deleted"))
	require.True(t, os.IsNotExist(err), "neither foreign descendants nor the directory may be deleted")
}

const cleanupFixture = `#!/usr/bin/env python3
import json,os,sys
from pathlib import Path
p=Path(os.environ['CLEANUP_STATE'])
args=sys.argv[1:]
if args[0]=='fail':
    print('scenario failure',file=sys.stderr)
    sys.exit(1)
values=json.loads((p/'resources.json').read_text())
fault=os.environ.get('CLEANUP_FAULT','')
if args[0]=='get':
    if fault=='read-error':
        print('collection read failed',file=sys.stderr)
        sys.exit(1)
    if fault=='duplicate': values=values+values
    if fault=='missing-id': values[0]['id']=''
    if args[:3]==['get','identity','principals']:
        directory=args[args.index('--directory-id')+1]
        values=[v for v in values if v.get('kind')=='principal' and v.get('directory_id')==directory]
    else:
        values=[v for v in values if v.get('kind')!='principal']
    print(json.dumps(values))
elif args[:2]==['api','delete']:
    endpoint=args[2]
    ident=endpoint.rsplit('/',1)[1].split('?')[0]
    values=[v for v in values if v['id']!=ident]
    (p/'resources.json').write_text(json.dumps(values))
    with (p/'deleted').open('a') as f: f.write(endpoint+'\n')
    print('{}')
else:
    raise RuntimeError('unexpected command')
`
