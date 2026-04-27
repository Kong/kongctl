package extensions

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	githubOwnerPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)
	githubRepoPattern  = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
)

type GitHubSource struct {
	Owner string
	Repo  string
	Ref   string
}

type FetchedGitHubSource struct {
	Dir            string
	Repository     string
	URL            string
	Ref            string
	ResolvedCommit string
	Cleanup        func()
}

func ParseGitHubSource(source, ref string) (GitHubSource, bool, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return GitHubSource{}, false, fmt.Errorf("extension source is required")
	}
	if strings.HasPrefix(source, ".") || strings.HasPrefix(source, "/") || strings.HasPrefix(source, "~") {
		return GitHubSource{}, false, nil
	}

	owner, repo, ok := parseGitHubOwnerRepo(source)
	if !ok {
		return GitHubSource{}, false, nil
	}
	if !githubOwnerPattern.MatchString(owner) || !githubRepoPattern.MatchString(repo) {
		return GitHubSource{}, true, fmt.Errorf("invalid GitHub source %q", source)
	}
	return GitHubSource{Owner: owner, Repo: repo, Ref: strings.TrimSpace(ref)}, true, nil
}

func (s GitHubSource) Repository() string {
	return s.Owner + "/" + s.Repo
}

func (s GitHubSource) CloneURL() string {
	return "https://github.com/" + s.Repository() + ".git"
}

func FetchGitHubSource(ctx context.Context, source GitHubSource, tempRoot string) (FetchedGitHubSource, error) {
	if strings.TrimSpace(source.Owner) == "" || strings.TrimSpace(source.Repo) == "" {
		return FetchedGitHubSource{}, fmt.Errorf("GitHub source owner and repo are required")
	}
	if tempRoot == "" {
		tempRoot = os.TempDir()
	}
	if err := os.MkdirAll(tempRoot, 0o700); err != nil {
		return FetchedGitHubSource{}, err
	}
	workDir, err := os.MkdirTemp(tempRoot, "github-source-*")
	if err != nil {
		return FetchedGitHubSource{}, err
	}
	cleanup := func() {
		_ = os.RemoveAll(workDir)
	}

	cloneDir := filepath.Join(workDir, "repo")
	args := []string{"clone", "--depth", "1"}
	if source.Ref != "" {
		args = append(args, "--branch", source.Ref)
	}
	args = append(args, source.CloneURL(), cloneDir)
	if output, err := exec.CommandContext(ctx, "git", args...).CombinedOutput(); err != nil {
		cleanup()
		return FetchedGitHubSource{}, fmt.Errorf("git clone %s failed: %w\n%s",
			source.Repository(), err, strings.TrimSpace(string(output)))
	}

	commit, err := gitOutput(ctx, cloneDir, "rev-parse", "HEAD")
	if err != nil {
		cleanup()
		return FetchedGitHubSource{}, err
	}

	return FetchedGitHubSource{
		Dir:            cloneDir,
		Repository:     source.Repository(),
		URL:            source.CloneURL(),
		Ref:            source.Ref,
		ResolvedCommit: strings.TrimSpace(commit),
		Cleanup:        cleanup,
	}, nil
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func parseGitHubOwnerRepo(source string) (string, string, bool) {
	source = strings.TrimSpace(strings.TrimSuffix(source, ".git"))
	if strings.HasPrefix(source, "git@github.com:") {
		source = strings.TrimPrefix(source, "git@github.com:")
		return splitOwnerRepo(source)
	}

	if strings.HasPrefix(source, "https://") || strings.HasPrefix(source, "http://") {
		parsed, err := url.Parse(source)
		if err != nil || !strings.EqualFold(parsed.Host, "github.com") {
			return "", "", false
		}
		return splitOwnerRepo(strings.TrimPrefix(parsed.Path, "/"))
	}

	if strings.HasPrefix(source, "github.com/") {
		return splitOwnerRepo(strings.TrimPrefix(source, "github.com/"))
	}

	return splitOwnerRepo(source)
}

func splitOwnerRepo(value string) (string, string, bool) {
	owner, repo, ok := strings.Cut(strings.Trim(value, "/"), "/")
	if !ok || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return "", "", false
	}
	return owner, repo, true
}
