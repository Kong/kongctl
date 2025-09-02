//go:build e2e

package e2e

import (
	"context"
	"testing"

	"github.com/kong/kongctl/test/e2e/harness"
)

func Test_VersionFull_JSON(t *testing.T) {
	harness.RequireBinary(t)

	cli, err := harness.NewCLIT(t)
	if err != nil {
		t.Fatalf("harness init failed: %v", err)
	}
	var out struct {
		Version string `json:"version"`
		Commit  string `json:"commit"`
		Date    string `json:"date"`
	}
	res, err := cli.RunJSON(context.Background(), &out, "version", "--full")
	if err != nil {
		t.Fatalf("command failed: exit=%d stderr=%s err=%v", res.ExitCode, res.Stderr, err)
	}
	if out.Version == "" {
		t.Fatalf("expected non-empty version in JSON output")
	}
}
