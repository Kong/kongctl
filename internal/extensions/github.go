package extensions

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"unicode"

	"github.com/kong/kongctl/internal/meta"
	"github.com/kong/kongctl/internal/util/httpheaders"
)

const (
	SourceTypeLocalPath          = "local_path"
	SourceTypeGitHubSource       = "github_source"
	SourceTypeGitHubReleaseAsset = "github_release_asset"

	maxGitHubReleaseArchiveEntryBytes = 256 * 1024 * 1024
)

var (
	githubOwnerPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)
	githubRepoPattern  = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

	errGitHubReleaseUnavailable = errors.New("GitHub release artifact unavailable")

	githubAPIBaseURL = "https://api.github.com"
	githubHTTPClient = http.DefaultClient
)

type GitHubSource struct {
	Owner string
	Repo  string
	Ref   string
}

type FetchedGitHubSource struct {
	SourceType     string
	Dir            string
	Repository     string
	URL            string
	Ref            string
	ResolvedCommit string
	ReleaseTag     string
	AssetName      string
	AssetURL       string
	Cleanup        func()
}

func ParseGitHubSource(source, ref string) (GitHubSource, bool, error) {
	source = strings.TrimSpace(source)
	ref = strings.TrimSpace(ref)
	if source == "" {
		return GitHubSource{}, false, fmt.Errorf("extension source is required")
	}
	if strings.HasPrefix(source, ".") || strings.HasPrefix(source, "/") || strings.HasPrefix(source, "~") {
		return GitHubSource{}, false, nil
	}
	var inlineRef string
	source, inlineRef = splitGitHubInlineRef(source)
	if ref != "" && inlineRef != "" {
		return GitHubSource{}, true, fmt.Errorf("use either --ref or @ref, not both")
	}
	if ref == "" {
		ref = inlineRef
	}

	owner, repo, ok := parseGitHubOwnerRepo(source)
	if !ok {
		return GitHubSource{}, false, nil
	}
	if !githubOwnerPattern.MatchString(owner) || !githubRepoPattern.MatchString(repo) {
		return GitHubSource{}, true, fmt.Errorf("invalid GitHub source %q", source)
	}
	return GitHubSource{Owner: owner, Repo: repo, Ref: ref}, true, nil
}

func (s GitHubSource) Repository() string {
	return s.Owner + "/" + s.Repo
}

func (s GitHubSource) RepositoryURL() string {
	return "https://github.com/" + s.Repository()
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

	fetched, err := fetchGitHubReleaseAsset(ctx, source, tempRoot)
	if err == nil {
		return fetched, nil
	}
	if !errors.Is(err, errGitHubReleaseUnavailable) {
		return FetchedGitHubSource{}, err
	}

	return fetchGitHubSourceClone(ctx, source, tempRoot)
}

func fetchGitHubReleaseAsset(ctx context.Context, source GitHubSource, tempRoot string) (FetchedGitHubSource, error) {
	release, err := getGitHubRelease(ctx, source)
	if err != nil {
		return FetchedGitHubSource{}, err
	}
	asset, err := selectGitHubReleaseAsset(release.Assets)
	if err != nil {
		return FetchedGitHubSource{}, err
	}

	workDir, err := os.MkdirTemp(tempRoot, "github-release-*")
	if err != nil {
		return FetchedGitHubSource{}, err
	}
	cleanup := func() {
		_ = os.RemoveAll(workDir)
	}

	archivePath := filepath.Join(workDir, "release-asset")
	if err := downloadGitHubAsset(ctx, asset.DownloadURL, archivePath); err != nil {
		cleanup()
		return FetchedGitHubSource{}, err
	}

	packageDir := filepath.Join(workDir, "package")
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		cleanup()
		return FetchedGitHubSource{}, err
	}
	if err := extractGitHubReleaseArchive(archivePath, asset.Name, packageDir); err != nil {
		cleanup()
		return FetchedGitHubSource{}, err
	}
	if _, err := os.Stat(filepath.Join(packageDir, ManifestFileName)); err != nil {
		cleanup()
		if os.IsNotExist(err) {
			return FetchedGitHubSource{}, fmt.Errorf(
				"release asset %q must contain %s at the archive root",
				asset.Name,
				ManifestFileName,
			)
		}
		return FetchedGitHubSource{}, err
	}

	return FetchedGitHubSource{
		SourceType: SourceTypeGitHubReleaseAsset,
		Dir:        packageDir,
		Repository: source.Repository(),
		URL:        source.RepositoryURL(),
		Ref:        release.TagName,
		ReleaseTag: release.TagName,
		AssetName:  asset.Name,
		AssetURL:   asset.DownloadURL,
		Cleanup:    cleanup,
	}, nil
}

func fetchGitHubSourceClone(ctx context.Context, source GitHubSource, tempRoot string) (FetchedGitHubSource, error) {
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
		SourceType:     SourceTypeGitHubSource,
		Dir:            cloneDir,
		Repository:     source.Repository(),
		URL:            source.CloneURL(),
		Ref:            source.Ref,
		ResolvedCommit: strings.TrimSpace(commit),
		Cleanup:        cleanup,
	}, nil
}

type githubRelease struct {
	TagName string               `json:"tag_name"`
	Assets  []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
}

func getGitHubRelease(ctx context.Context, source GitHubSource) (githubRelease, error) {
	releaseURL := strings.TrimRight(githubAPIBaseURL, "/") + "/repos/" + source.Owner + "/" + source.Repo
	if source.Ref != "" {
		releaseURL += "/releases/tags/" + url.PathEscape(source.Ref)
	} else {
		releaseURL += "/releases/latest"
	}
	request, err := newGitHubRequest(ctx, releaseURL)
	if err != nil {
		return githubRelease{}, err
	}
	httpheaders.SetAccept(request, "application/vnd.github+json")

	response, err := githubHTTPClient.Do(request)
	if err != nil {
		return githubRelease{}, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return githubRelease{}, errGitHubReleaseUnavailable
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 64*1024))
		return githubRelease{}, fmt.Errorf(
			"fetch GitHub release for %s failed: %s\n%s",
			source.Repository(),
			response.Status,
			strings.TrimSpace(string(body)),
		)
	}

	var release githubRelease
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(&release); err != nil {
		return githubRelease{}, fmt.Errorf("decode GitHub release response: %w", err)
	}
	if strings.TrimSpace(release.TagName) == "" {
		return githubRelease{}, fmt.Errorf("GitHub release for %s did not include a tag name", source.Repository())
	}
	release.TagName = strings.TrimSpace(release.TagName)
	return release, nil
}

func selectGitHubReleaseAsset(assets []githubReleaseAsset) (githubReleaseAsset, error) {
	candidates := make([]githubReleaseAsset, 0, len(assets))
	for _, asset := range assets {
		asset.Name = strings.TrimSpace(asset.Name)
		asset.DownloadURL = strings.TrimSpace(asset.DownloadURL)
		if asset.Name == "" || asset.DownloadURL == "" {
			continue
		}
		if releaseArchiveKind(asset.Name) == "" {
			continue
		}
		candidates = append(candidates, asset)
	}
	if len(candidates) == 0 {
		return githubReleaseAsset{}, errGitHubReleaseUnavailable
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}

	platformMatches := make([]githubReleaseAsset, 0, len(candidates))
	for _, candidate := range candidates {
		if githubAssetMatchesCurrentPlatform(candidate.Name) {
			platformMatches = append(platformMatches, candidate)
		}
	}
	switch len(platformMatches) {
	case 1:
		return platformMatches[0], nil
	case 0:
	default:
		return githubReleaseAsset{}, fmt.Errorf(
			"multiple compatible release archive assets found for %s/%s: %s",
			runtime.GOOS,
			runtime.GOARCH,
			githubAssetNames(platformMatches),
		)
	}

	universalMatches := make([]githubReleaseAsset, 0, len(candidates))
	for _, candidate := range candidates {
		if githubAssetIsUniversal(candidate.Name) {
			universalMatches = append(universalMatches, candidate)
		}
	}
	switch len(universalMatches) {
	case 0:
		return githubReleaseAsset{}, fmt.Errorf(
			"multiple release archive assets found; publish one archive asset or include %s and %s "+
				"in the platform-specific asset name: %s",
			runtime.GOOS,
			runtime.GOARCH,
			githubAssetNames(candidates),
		)
	case 1:
		return universalMatches[0], nil
	default:
		return githubReleaseAsset{}, fmt.Errorf(
			"multiple universal release archive assets found: %s",
			githubAssetNames(universalMatches),
		)
	}
}

func githubAssetMatchesCurrentPlatform(name string) bool {
	tokens := githubAssetNameTokens(name)
	return slices.Contains(tokens, runtime.GOOS) && slices.Contains(tokens, runtime.GOARCH)
}

func githubAssetIsUniversal(name string) bool {
	tokens := githubAssetNameTokens(name)
	return slices.Contains(tokens, "universal") || slices.Contains(tokens, "all") || slices.Contains(tokens, "any")
}

func githubAssetNameTokens(name string) []string {
	return strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func githubAssetNames(assets []githubReleaseAsset) string {
	names := make([]string, 0, len(assets))
	for _, asset := range assets {
		names = append(names, asset.Name)
	}
	return strings.Join(names, ", ")
}

func downloadGitHubAsset(ctx context.Context, downloadURL, target string) error {
	request, err := newGitHubRequest(ctx, downloadURL)
	if err != nil {
		return err
	}
	httpheaders.SetAccept(request, "application/octet-stream")

	response, err := githubHTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 64*1024))
		return fmt.Errorf(
			"download GitHub release asset failed: %s\n%s",
			response.Status,
			strings.TrimSpace(string(body)),
		)
	}

	file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, response.Body); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func newGitHubRequest(ctx context.Context, rawURL string) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	httpheaders.SetUserAgent(request, meta.UserAgent())
	if token := githubToken(); token != "" {
		httpheaders.SetBearerAuthorization(request, token)
	}
	return request, nil
}

func githubToken() string {
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		return token
	}
	return strings.TrimSpace(os.Getenv("GH_TOKEN"))
}

func extractGitHubReleaseArchive(archivePath, assetName, targetDir string) error {
	switch releaseArchiveKind(assetName) {
	case "zip":
		return extractGitHubReleaseZip(archivePath, targetDir)
	case "tar.gz":
		return extractGitHubReleaseTarGzip(archivePath, targetDir)
	default:
		return fmt.Errorf("unsupported release asset archive %q", assetName)
	}
}

func releaseArchiveKind(assetName string) string {
	lower := strings.ToLower(assetName)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return "tar.gz"
	case strings.HasSuffix(lower, ".zip"):
		return "zip"
	default:
		return ""
	}
}

func extractGitHubReleaseZip(archivePath, targetDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		if strings.Contains(file.Name, "..") {
			return fmt.Errorf("release archive entry %q contains a parent-directory marker", file.Name)
		}
		target, err := safeArchiveTarget(targetDir, file.Name)
		if err != nil {
			return err
		}
		if target == "" {
			continue
		}
		info := file.FileInfo()
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("release archive entry %q is a symlink; symlinks are not supported", file.Name)
		}
		if info.IsDir() {
			if err := os.MkdirAll(target, archiveDirMode(info.Mode())); err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported release archive entry %q", file.Name)
		}
		if file.UncompressedSize64 > maxGitHubReleaseArchiveEntryBytes {
			return fmt.Errorf(
				"release archive entry %q exceeds %d bytes",
				file.Name,
				maxGitHubReleaseArchiveEntryBytes,
			)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := file.Open()
		if err != nil {
			return err
		}
		mode := archiveFileMode(info.Mode())
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		if err != nil {
			_ = in.Close()
			return err
		}
		written, copyErr := io.Copy(out, io.LimitReader(in, maxGitHubReleaseArchiveEntryBytes+1))
		closeInErr := in.Close()
		if copyErr != nil {
			_ = out.Close()
			return copyErr
		}
		if closeInErr != nil {
			_ = out.Close()
			return closeInErr
		}
		if written > maxGitHubReleaseArchiveEntryBytes {
			_ = out.Close()
			return fmt.Errorf(
				"release archive entry %q exceeds %d bytes",
				file.Name,
				maxGitHubReleaseArchiveEntryBytes,
			)
		}
		if err := out.Close(); err != nil {
			return err
		}
	}
	return nil
}

func extractGitHubReleaseTarGzip(archivePath, targetDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := extractGitHubReleaseTarEntry(reader, header, targetDir); err != nil {
			return err
		}
	}
}

func extractGitHubReleaseTarEntry(reader *tar.Reader, header *tar.Header, targetDir string) error {
	switch header.Typeflag {
	case tar.TypeXGlobalHeader, tar.TypeXHeader:
		return nil
	case tar.TypeSymlink, tar.TypeLink:
		return fmt.Errorf("release archive entry %q is a link; links are not supported", header.Name)
	}
	if strings.Contains(header.Name, "..") {
		return fmt.Errorf("release archive entry %q contains a parent-directory marker", header.Name)
	}

	target, err := safeArchiveTarget(targetDir, header.Name)
	if err != nil {
		return err
	}
	if target == "" {
		return nil
	}

	mode := fs.FileMode(header.Mode).Perm()
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, archiveDirMode(mode))
	case tar.TypeReg, 0:
		if header.Size > maxGitHubReleaseArchiveEntryBytes {
			return fmt.Errorf(
				"release archive entry %q exceeds %d bytes",
				header.Name,
				maxGitHubReleaseArchiveEntryBytes,
			)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, archiveFileMode(mode))
		if err != nil {
			return err
		}
		written, err := io.Copy(out, io.LimitReader(reader, header.Size))
		if err != nil {
			_ = out.Close()
			return err
		}
		if written != header.Size {
			_ = out.Close()
			return fmt.Errorf("release archive entry %q ended unexpectedly", header.Name)
		}
		return out.Close()
	default:
		return fmt.Errorf("unsupported release archive entry %q", header.Name)
	}
}

func safeArchiveTarget(root, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("release archive entry %q contains a parent-directory marker", name)
	}
	if strings.Contains(name, `\`) || strings.Contains(name, ":") {
		return "", fmt.Errorf("release archive entry %q contains a reserved path character", name)
	}
	cleaned := path.Clean(name)
	if cleaned == "." {
		return "", nil
	}
	if path.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("release archive entry %q escapes the extension root", name)
	}
	target := filepath.Join(root, filepath.FromSlash(cleaned))
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if err := ensureInside(rootAbs, targetAbs); err != nil {
		return "", fmt.Errorf("release archive entry %q escapes the extension root: %w", name, err)
	}
	return targetAbs, nil
}

func archiveFileMode(mode fs.FileMode) fs.FileMode {
	mode = mode.Perm()
	if mode == 0 {
		return 0o644
	}
	return mode
}

func archiveDirMode(mode fs.FileMode) fs.FileMode {
	mode = mode.Perm()
	if mode == 0 {
		return 0o755
	}
	return mode
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

func splitGitHubInlineRef(source string) (string, string) {
	if strings.HasPrefix(source, "git@") || strings.Contains(source, "://") {
		return source, ""
	}
	before, after, ok := strings.Cut(source, "@")
	if !ok {
		return source, ""
	}
	if before == "" || after == "" {
		return source, ""
	}
	return before, after
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
