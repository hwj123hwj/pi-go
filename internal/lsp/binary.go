package lsp

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// ─── BinaryManager ────────────────────────────────────────────────────────────
//
// Manages automatic download and caching of language server binaries.
// Ported from hwjcode's TypeScript BinaryManager.
//
// Cache location: ~/.pi-go/lsp/<server-id>/
//
// Three installer strategies:
//   - NPM:    typescript-language-server, pyright, yaml, docker, etc.
//   - Go:     gopls (requires Go toolchain)
//   - GitHub: rust-analyzer, clangd (direct prebuilt binary download)

// LSPCacheDir returns the cache directory for LSP binaries.
func LSPCacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".pi-go", "lsp")
}

// serverCacheDir returns the per-server cache directory, creating it if needed.
func serverCacheDir(serverID string) (string, error) {
	dir := filepath.Join(LSPCacheDir(), serverID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create cache dir %s: %w", dir, err)
	}
	return dir, nil
}

// ─── Installer function type ──────────────────────────────────────────────────

// InstallerFunc downloads/installs a binary into destDir and returns its path.
type InstallerFunc func(destDir string) (string, error)

// EnsureBinary checks if the binary exists in cache; if not, runs the installer.
// On failure, it cleans the cache and retries once.
func EnsureBinary(serverID string, installer InstallerFunc) (string, error) {
	maxRetries := 1
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		destDir, err := serverCacheDir(serverID)
		if err != nil {
			return "", err
		}

		binPath, err := installer(destDir)
		if err == nil {
			return binPath, nil
		}

		lastErr = err
		if attempt < maxRetries {
			slog.Warn("LSP binary install failed, cleaning cache and retrying",
				"server", serverID, "attempt", attempt+1, "err", err)
			cleanBinaryCache(serverID)
		}
	}
	return "", fmt.Errorf("install %s: %w", serverID, lastErr)
}

// cleanBinaryCache removes the per-server cache directory.
func cleanBinaryCache(serverID string) {
	dir := filepath.Join(LSPCacheDir(), serverID)
	if err := os.RemoveAll(dir); err != nil {
		slog.Warn("LSP failed to clean binary cache", "server", serverID, "err", err)
	}
}

// ─── findOnPath: locate an executable on PATH ─────────────────────────────────

func findOnPath(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}

// ─── NPM Installer ────────────────────────────────────────────────────────────
//
// Installs npm packages into a temp directory and returns the path to the
// binary in node_modules/.bin/.

// NpmInstaller returns an InstallerFunc that installs npm packages.
func NpmInstaller(packages []string, binName string) InstallerFunc {
	return func(destDir string) (string, error) {
		binPath := filepath.Join(destDir, "node_modules", ".bin", binName)
		if runtime.GOOS == "windows" {
			binPath += ".cmd"
		}

		if _, err := os.Stat(binPath); err == nil {
			return binPath, nil // cached
		}

		npmBin := findOnPath("npm")
		if npmBin == "" {
			return "", fmt.Errorf("npm not found in PATH (required to install %s). Please install Node.js", binName)
		}

		// Ensure package.json exists so npm install works in this dir
		pkgJSON := filepath.Join(destDir, "package.json")
		if _, err := os.Stat(pkgJSON); err != nil {
			pkgContent := fmt.Sprintf(`{"name":"pi-go-lsp-%s","version":"1.0.0","private":true}`, binName)
			if err := os.WriteFile(pkgJSON, []byte(pkgContent), 0o644); err != nil {
				return "", fmt.Errorf("write package.json: %w", err)
			}
		}

		slog.Info("LSP installing via npm", "packages", packages, "bin", binName)
		args := append([]string{"install"}, packages...)
		args = append(args, "--no-save")

		cmd := exec.Command(npmBin, args...)
		cmd.Dir = destDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("npm install failed: %w\n%s", err, string(output))
		}

		if _, err := os.Stat(binPath); err != nil {
			return "", fmt.Errorf("npm install completed but binary not found at %s", binPath)
		}
		return binPath, nil
	}
}

// ─── Go Installer ─────────────────────────────────────────────────────────────
//
// Installs a Go tool via `go install`. Requires Go toolchain on PATH.

// GoInstaller returns an InstallerFunc that installs a Go module.
func GoInstaller(modulePath, binName string) InstallerFunc {
	return func(destDir string) (string, error) {
		binNameWithExt := binName
		if runtime.GOOS == "windows" {
			binNameWithExt += ".exe"
		}
		binPath := filepath.Join(destDir, binNameWithExt)

		if _, err := os.Stat(binPath); err == nil {
			return binPath, nil // cached
		}

		goBin := findOnPath("go")
		if goBin == "" {
			return "", fmt.Errorf("Go toolchain not found in PATH (required to install %s). Please install Go from https://go.dev/dl/", binName)
		}

		slog.Info("LSP installing via go", "module", modulePath, "bin", binName)
		cmd := exec.Command(goBin, "install", modulePath+"@latest")
		cmd.Dir = destDir
		cmd.Env = append(os.Environ(), "GOBIN="+destDir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("go install failed: %w\n%s", err, string(output))
		}

		if _, err := os.Stat(binPath); err != nil {
			return "", fmt.Errorf("go install completed but binary not found at %s", binPath)
		}
		return binPath, nil
	}
}

// ─── GitHub Installer ─────────────────────────────────────────────────────────
//
// Downloads a prebuilt binary from GitHub Releases.

// GitHubReleaseAsset represents an asset in a GitHub release.
type GitHubReleaseAsset struct {
	Name                 string `json:"name"`
	BrowserDownloadURL   string `json:"browser_download_url"`
	Size                 int64  `json:"size"`
}

// GitHubRelease represents a GitHub release.
type GitHubRelease struct {
	TagName string               `json:"tag_name"`
	Assets  []GitHubReleaseAsset `json:"assets"`
}

// AssetMatcher is a function that picks the right asset for the current platform.
type AssetMatcher func(goos, goarch string) *regexp.Regexp

// GitHubInstaller returns an InstallerFunc that downloads from GitHub Releases.
func GitHubInstaller(owner, repo string, matcher AssetMatcher) InstallerFunc {
	return func(destDir string) (string, error) {
		binName := repo
		if runtime.GOOS == "windows" {
			binName += ".exe"
		}
		binPath := filepath.Join(destDir, binName)

		if _, err := os.Stat(binPath); err == nil {
			return binPath, nil // cached
		}

		// 1. Fetch latest release info
		apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
		release, err := fetchGitHubRelease(apiURL)
		if err != nil {
			return "", fmt.Errorf("fetch GitHub release for %s/%s: %w", owner, repo, err)
		}

		// 2. Find matching asset
		expectedPattern := matcher(runtime.GOOS, runtime.GOARCH)
		var asset *GitHubReleaseAsset
		for i := range release.Assets {
			if expectedPattern.MatchString(release.Assets[i].Name) {
				asset = &release.Assets[i]
				break
			}
		}
		if asset == nil {
			// Fallback: log available assets
			names := make([]string, len(release.Assets))
			for i, a := range release.Assets {
				names[i] = a.Name
			}
			return "", fmt.Errorf("no matching asset for %s/%s on %s/%s. Available: %s",
				owner, repo, runtime.GOOS, runtime.GOARCH, strings.Join(names, ", "))
		}

		slog.Info("LSP downloading from GitHub", "repo", repo, "asset", asset.Name,
			"size_mb", fmt.Sprintf("%.1f", float64(asset.Size)/1024/1024))

		// 3. Download
		tempPath := filepath.Join(destDir, asset.Name)
		if err := downloadFile(asset.BrowserDownloadURL, tempPath); err != nil {
			return "", fmt.Errorf("download %s: %w", asset.Name, err)
		}

		// 4. Extract / install
		lower := strings.ToLower(asset.Name)
		switch {
		case strings.HasSuffix(lower, ".gz"):
			if err := extractGzip(tempPath, binPath); err != nil {
				return "", fmt.Errorf("extract gzip: %w", err)
			}
			os.Remove(tempPath)
		case strings.HasSuffix(lower, ".zip"):
			if err := extractZip(tempPath, binPath, binName); err != nil {
				return "", fmt.Errorf("extract zip: %w", err)
			}
			os.Remove(tempPath)
		case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
			if err := extractTarGz(tempPath, binPath, binName); err != nil {
				return "", fmt.Errorf("extract tar.gz: %w", err)
			}
			os.Remove(tempPath)
		default:
			// Uncompressed binary
			if err := os.Rename(tempPath, binPath); err != nil {
				return "", fmt.Errorf("move binary: %w", err)
			}
		}

		// 5. chmod +x (Unix only)
		if runtime.GOOS != "windows" {
			if err := os.Chmod(binPath, 0o755); err != nil {
				slog.Warn("LSP failed to chmod binary", "path", binPath, "err", err)
			}
		}

		if _, err := os.Stat(binPath); err != nil {
			return "", fmt.Errorf("binary not found after extraction at %s", binPath)
		}

		slog.Info("LSP binary installed", "server", repo, "path", binPath)
		return binPath, nil
	}
}

// fetchGitHubRelease fetches release info from the GitHub API.
func fetchGitHubRelease(apiURL string) (*GitHubRelease, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "pi-go-lsp")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode release JSON: %w", err)
	}
	return &release, nil
}

// downloadFile downloads a URL to a local path with a timeout.
func downloadFile(url, destPath string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// extractGzip decompresses a single-file .gz archive to outPath.
func extractGzip(gzPath, outPath string) error {
	gzFile, err := os.Open(gzPath)
	if err != nil {
		return err
	}
	defer gzFile.Close()

	gzReader, err := gzip.NewReader(gzFile)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, gzReader)
	return err
}

// extractZip extracts a .zip archive, finding the binary matching expectedName.
func extractZip(zipPath, outPath, expectedName string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	// Find the entry whose basename matches expectedName
	for _, f := range r.File {
		base := strings.ToLower(filepath.Base(f.Name))
		if base == strings.ToLower(expectedName) && !f.FileInfo().IsDir() {
			return extractZipFile(f, outPath)
		}
	}
	return fmt.Errorf("zip archive does not contain %s", expectedName)
}

func extractZipFile(f *zip.File, outPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

// extractTarGz extracts a .tar.gz archive, finding the binary matching expectedName.
func extractTarGz(tgzPath, outPath, expectedName string) error {
	f, err := os.Open(tgzPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	expectedLower := strings.ToLower(expectedName)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		base := strings.ToLower(filepath.Base(header.Name))
		if base == expectedLower && header.Typeflag == tar.TypeReg {
			out, err := os.Create(outPath)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tarReader)
			out.Close()
			return err
		}
	}
	return fmt.Errorf("tar.gz archive does not contain %s", expectedName)
}
