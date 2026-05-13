package common

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestValidateOutputFormat_BaseFormats(t *testing.T) {
	cmd := &cobra.Command{Use: "leaf"}
	for _, v := range []string{"json", "yaml", "text"} {
		if err := ValidateOutputFormat(cmd, v); err != nil {
			t.Errorf("ValidateOutputFormat(%q) returned error: %v", v, err)
		}
	}
}

func TestValidateOutputFormat_RejectsUnknown(t *testing.T) {
	cmd := &cobra.Command{Use: "leaf"}
	err := ValidateOutputFormat(cmd, "bogus")
	if err == nil {
		t.Fatalf("expected error for bogus value")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("expected error to mention value, got: %v", err)
	}
}

func TestValidateOutputFormat_AllowsExtraOnCommand(t *testing.T) {
	cmd := &cobra.Command{Use: "leaf"}
	AllowExtraOutputFormats(cmd, "helm")
	if err := ValidateOutputFormat(cmd, "helm"); err != nil {
		t.Errorf("expected helm to be allowed: %v", err)
	}
}

func TestValidateOutputFormat_RejectsHelmWithoutOptIn(t *testing.T) {
	cmd := &cobra.Command{Use: "leaf"}
	if err := ValidateOutputFormat(cmd, "helm"); err == nil {
		t.Fatalf("expected helm to be rejected on a bare command")
	}
}

func TestValidateOutputFormat_AllowsExtraFromAncestor(t *testing.T) {
	parent := &cobra.Command{Use: "parent"}
	child := &cobra.Command{Use: "child"}
	parent.AddCommand(child)
	AllowExtraOutputFormats(parent, "helm")
	if err := ValidateOutputFormat(child, "helm"); err != nil {
		t.Errorf("expected helm to be allowed via parent annotation: %v", err)
	}
}

func TestValidateOutputFormat_RejectsBogusEvenWithExtras(t *testing.T) {
	cmd := &cobra.Command{Use: "leaf"}
	AllowExtraOutputFormats(cmd, "helm")
	if err := ValidateOutputFormat(cmd, "bogus"); err == nil {
		t.Fatalf("expected bogus to be rejected even with helm opt-in")
	}
}

func TestAllowExtraOutputFormats_Merges(t *testing.T) {
	cmd := &cobra.Command{Use: "leaf"}
	AllowExtraOutputFormats(cmd, "helm")
	AllowExtraOutputFormats(cmd, "helm", "terraform")
	got := cmd.Annotations[ExtraOutputFormatsAnnotation]
	if got != "helm,terraform" {
		t.Errorf("expected dedup-merge, got %q", got)
	}
}

func TestAllowExtraOutputFormats_IgnoresEmpty(t *testing.T) {
	cmd := &cobra.Command{Use: "leaf"}
	AllowExtraOutputFormats(cmd, "", "  ", "helm")
	got := cmd.Annotations[ExtraOutputFormatsAnnotation]
	if got != "helm" {
		t.Errorf("expected empty values dropped, got %q", got)
	}
}
