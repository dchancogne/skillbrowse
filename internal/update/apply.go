package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dchancogne/skillbrowse/internal/debug"
)

const (
	maxArchiveSize   = 200 << 20 // 200 MiB
	maxChecksumsSize = 1 << 20   // 1 MiB
	maxSignatureSize = 4096
)

// CheckResult is the outcome of checking for an update, per design doc
// §12.1: current/target versions, release URL, and download size.
type CheckResult struct {
	Current         string
	Latest          string
	UpdateAvailable bool
	Release         *Release
	Asset           Asset
	ReleaseURL      string
}

// CheckForUpdate fetches the latest release and compares it against
// currentVersion.
func CheckForUpdate(ctx context.Context, client *Client, currentVersion string) (*CheckResult, error) {
	release, err := client.LatestRelease(ctx)
	if err != nil {
		return nil, err
	}

	assetName := AssetName(runtime.GOOS, runtime.GOARCH)
	asset, ok := release.FindAsset(assetName)
	if !ok {
		return nil, fmt.Errorf("no release asset found for %s/%s (expected %q)", runtime.GOOS, runtime.GOARCH, assetName)
	}

	return &CheckResult{
		Current:         currentVersion,
		Latest:          strings.TrimPrefix(release.TagName, "v"),
		UpdateAvailable: compareVersions(currentVersion, release.TagName) < 0,
		Release:         release,
		Asset:           asset,
		ReleaseURL:      release.HTMLURL,
	}, nil
}

// Options configures Apply.
type Options struct {
	Client   *Client
	Verifier Verifier

	// ExecPath overrides the executable to replace. Empty means the
	// currently running binary, resolved via os.Executable().
	ExecPath string
}

// Apply downloads, verifies, and installs the release described by
// check, following the exact ordering from design doc §12.3: download
// bounded temp files, verify signature, verify checksum, safely extract,
// confirm the staged binary's version, then atomically rename it over
// the running executable. Every check happens before that final rename;
// any error returned here leaves the current executable byte-for-byte
// untouched.
func Apply(ctx context.Context, opts Options, check *CheckResult) error {
	execPath, err := resolveExecPath(opts.ExecPath)
	if err != nil {
		return err
	}
	installDir := filepath.Dir(execPath)

	if err := preflightWritable(installDir); err != nil {
		return err
	}

	checksumsAsset, ok := check.Release.FindAsset(ChecksumsAssetName)
	if !ok {
		return fmt.Errorf("release is missing %s", ChecksumsAssetName)
	}
	sigAsset, ok := check.Release.FindAsset(SignatureAssetName)
	if !ok {
		return fmt.Errorf("release is missing %s", SignatureAssetName)
	}

	tmpDir, err := os.MkdirTemp("", "skillbrowse-update-")
	if err != nil {
		return fmt.Errorf("create temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	httpClient := opts.Client.httpClient()

	archivePath, err := downloadToFile(ctx, httpClient, check.Asset.BrowserDownloadURL, tmpDir, maxArchiveSize)
	if err != nil {
		return fmt.Errorf("download release archive: %w", err)
	}

	checksumsData, err := downloadToMemory(ctx, httpClient, checksumsAsset.BrowserDownloadURL, maxChecksumsSize)
	if err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}

	sigData, err := downloadToMemory(ctx, httpClient, sigAsset.BrowserDownloadURL, maxSignatureSize)
	if err != nil {
		return fmt.Errorf("download signature: %w", err)
	}

	if err := opts.Verifier.VerifyManifestSignature(checksumsData, sigData); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}
	debug.Log("update: signature verified")

	sums, err := parseChecksums(checksumsData)
	if err != nil {
		return fmt.Errorf("parse checksums: %w", err)
	}
	wantSum, ok := sums[check.Asset.Name]
	if !ok {
		return fmt.Errorf("checksums manifest does not list %s", check.Asset.Name)
	}
	if err := verifyArchiveChecksum(archivePath, wantSum); err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
	}
	debug.Log("update: checksum verified (%s)", wantSum)

	binaryName := filepath.Base(execPath)
	stagedPath, err := extractExecutable(archivePath, binaryName, installDir, maxArchiveSize)
	if err != nil {
		return fmt.Errorf("extract release archive: %w", err)
	}
	defer func() {
		if stagedPath != "" {
			_ = os.Remove(stagedPath)
		}
	}()
	debug.Log("update: extracted %s to %s", binaryName, stagedPath)

	gotVersion, err := stagedVersion(ctx, stagedPath)
	if err != nil {
		return fmt.Errorf("verify staged binary: %w", err)
	}
	if gotVersion != check.Latest {
		return fmt.Errorf("staged binary reports version %q, expected %q", gotVersion, check.Latest)
	}
	debug.Log("update: staged binary version confirmed (%s)", gotVersion)

	if err := os.Rename(stagedPath, execPath); err != nil {
		return fmt.Errorf("install updated binary: %w", err)
	}
	stagedPath = "" // renamed away; nothing left for the deferred cleanup to remove
	debug.Log("update: installed to %s", execPath)

	return nil
}

func resolveExecPath(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	p, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("determine running executable: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", fmt.Errorf("resolve running executable: %w", err)
	}
	return resolved, nil
}

// preflightWritable fails fast, before any network activity, if the
// install directory isn't writable — design doc's risk table calls for
// "installation-specific recovery guidance without privilege escalation."
func preflightWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".skillbrowse-update-preflight-*")
	if err != nil {
		return fmt.Errorf("cannot write to %s: %w (reinstall using the method you originally used; skillbrowse does not escalate privileges)", dir, err)
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return nil
}

func verifyArchiveChecksum(path, wantHex string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return verifyChecksum(f, wantHex)
}

// stagedVersion runs the staged binary's "version" subcommand and parses
// its first line, per design doc §12.3 step 5 ("Confirms the staged
// binary reports the expected target version").
func stagedVersion(ctx context.Context, path string) (string, error) {
	out, err := exec.CommandContext(ctx, path, "version").Output()
	if err != nil {
		return "", err
	}

	line := strings.SplitN(string(out), "\n", 2)[0]
	const prefix = "skillbrowse "
	if !strings.HasPrefix(line, prefix) {
		return "", fmt.Errorf("unrecognized version output: %q", line)
	}
	return strings.TrimSpace(strings.TrimPrefix(line, prefix)), nil
}

func downloadToFile(ctx context.Context, client *http.Client, url, dir string, maxSize int64) (string, error) {
	resp, err := doGet(ctx, client, url)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	out, err := os.CreateTemp(dir, ".skillbrowse-download-*")
	if err != nil {
		return "", err
	}
	path := out.Name()

	n, copyErr := io.Copy(out, io.LimitReader(resp.Body, maxSize+1))
	closeErr := out.Close()

	if copyErr != nil {
		cleanup(path)
		return "", copyErr
	}
	if closeErr != nil {
		cleanup(path)
		return "", closeErr
	}
	if n > maxSize {
		cleanup(path)
		return "", fmt.Errorf("download from %s exceeds %d byte limit", url, maxSize)
	}
	return path, nil
}

func downloadToMemory(ctx context.Context, client *http.Client, url string, maxSize int64) ([]byte, error) {
	resp, err := doGet(ctx, client, url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSize {
		return nil, fmt.Errorf("download from %s exceeds %d byte limit", url, maxSize)
	}
	return data, nil
}

func doGet(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	debug.Log("update: GET %s", url)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("GET %s: unexpected status %s", url, resp.Status)
	}
	return resp, nil
}
