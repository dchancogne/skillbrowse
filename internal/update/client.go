// Package update implements the signed self-upgrade flow: release
// lookup, checksum and signature verification, safe extraction, and
// atomic replacement, per
// docs/superpowers/specs/2026-08-12-skillbrowse-design.md §12.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dchancogne/skillbrowse/internal/debug"
)

// Owner and Repo are the compiled-in repository identity official
// binaries use to look up releases, per design doc §12.2.
const (
	Owner = "dchancogne"
	Repo  = "skillbrowse"
)

// requestTimeout bounds every network call the updater makes.
const requestTimeout = 15 * time.Second

// Asset is one file attached to a GitHub release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// Release is the subset of GitHub's release metadata the updater needs.
type Release struct {
	TagName string  `json:"tag_name"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

// FindAsset returns the release asset with the given exact name.
func (r *Release) FindAsset(name string) (Asset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

// Client looks up release metadata from GitHub. APIBaseURL is
// overridable so tests can point it at a local fake server.
type Client struct {
	HTTPClient *http.Client
	APIBaseURL string
	Owner      string
	Repo       string
}

// NewClient returns a Client configured for the official repository.
func NewClient() *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: requestTimeout},
		APIBaseURL: "https://api.github.com",
		Owner:      Owner,
		Repo:       Repo,
	}
}

// LatestRelease fetches the latest stable release. GitHub's
// /releases/latest endpoint itself excludes prereleases and drafts, so no
// additional filtering is needed here (design doc §12.1: "No prerelease
// is selected for a stable installed version").
func (c *Client) LatestRelease(ctx context.Context) (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.APIBaseURL, c.Owner, c.Repo)
	debug.Log("update: GET %s", url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch latest release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch latest release: unexpected status %s", resp.Status)
	}

	var release Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode release metadata: %w", err)
	}
	return &release, nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: requestTimeout}
}

// AssetName is the strict, GoReleaser-style archive name expected for a
// given OS/architecture, e.g. "skillbrowse_darwin_arm64.tar.gz".
func AssetName(goos, goarch string) string {
	return fmt.Sprintf("skillbrowse_%s_%s.tar.gz", goos, goarch)
}

// ChecksumsAssetName and SignatureAssetName are the fixed names of the
// checksum manifest and its Ed25519 signature, per design doc §12.3.
const (
	ChecksumsAssetName = "checksums.txt"
	SignatureAssetName = "checksums.txt.sig"
)
