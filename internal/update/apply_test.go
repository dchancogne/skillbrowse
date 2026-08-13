package update

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// fixtureServer serves a fake GitHub releases/latest endpoint plus
// asset downloads, so the update pipeline can be exercised end-to-end
// against real HTTP without touching the network, per
// docs/skillbrowse-implementation-plan.md Phase 3 exit criteria (design
// doc §14.3's 9 fake-release-server cases).
type fixtureServer struct {
	*httptest.Server
	assets    map[string][]byte
	overrides map[string]http.HandlerFunc
	tag       string
}

func newFixtureServer(t *testing.T, tag string) *fixtureServer {
	t.Helper()
	fs := &fixtureServer{assets: map[string][]byte{}, overrides: map[string]http.HandlerFunc{}, tag: tag}

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/o/r/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		release := Release{TagName: fs.tag, HTMLURL: "https://example.invalid/releases/" + fs.tag}
		for name, data := range fs.assets {
			release.Assets = append(release.Assets, Asset{
				Name:               name,
				BrowserDownloadURL: fs.URL + "/assets/" + name,
				Size:               int64(len(data)),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(release)
	})
	mux.HandleFunc("/assets/", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Path[len("/assets/"):]
		if h, ok := fs.overrides[name]; ok {
			h(w, r)
			return
		}
		data, ok := fs.assets[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	})

	fs.Server = httptest.NewServer(mux)
	t.Cleanup(fs.Close)
	return fs
}

func (fs *fixtureServer) client() *Client {
	return &Client{HTTPClient: &http.Client{Timeout: 5 * time.Second}, APIBaseURL: fs.URL, Owner: "o", Repo: "r"}
}

// versionScript is the fake "binary": a shell script that prints the
// version line our real version.go outputs, so stagedVersion's exec-and-
// parse step works unmodified against the fixture.
func versionScript(version string) string {
	return fmt.Sprintf("#!/bin/sh\necho \"skillbrowse %s\"\n", version)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// setValidRelease populates fs with a fully valid, correctly signed
// release for goos/goarch/version, and returns the signing key pair used
// (so a test can build a compatible or incompatible Verifier).
func (fs *fixtureServer) setValidRelease(t *testing.T, goos, goarch, version string) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	assetName := AssetName(goos, goarch)
	archive := buildTarGz(t, []tarEntry{{name: "skillbrowse", content: versionScript(version)}})
	archiveData, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}

	checksums := []byte(sha256Hex(archiveData) + "  " + assetName + "\n")
	sig := ed25519.Sign(priv, checksums)

	fs.assets[assetName] = archiveData
	fs.assets[ChecksumsAssetName] = checksums
	fs.assets[SignatureAssetName] = sig

	return pub, priv
}

func writeFile(t *testing.T, path string, content []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatal(err)
	}
}

func TestCheckForUpdate_NoUpdateAvailable(t *testing.T) {
	fs := newFixtureServer(t, "v1.0.0")
	fs.setValidRelease(t, testGOOS, testGOARCH, "1.0.0")

	res, err := CheckForUpdate(context.Background(), fs.client(), "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if res.UpdateAvailable {
		t.Error("expected no update available when current == latest")
	}
}

func TestApply_SuccessfulUpdate(t *testing.T) {
	fs := newFixtureServer(t, "v1.2.0")
	pub, _ := fs.setValidRelease(t, testGOOS, testGOARCH, "1.2.0")

	check, err := CheckForUpdate(context.Background(), fs.client(), "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !check.UpdateAvailable {
		t.Fatal("expected an update to be available")
	}

	execDir := t.TempDir()
	execPath := filepath.Join(execDir, "skillbrowse")
	writeFile(t, execPath, []byte(versionScript("1.0.0")), 0o755)

	opts := Options{Client: fs.client(), Verifier: Verifier{TrustedKeys: []ed25519.PublicKey{pub}}, ExecPath: execPath}
	if err := Apply(context.Background(), opts, check); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	data, err := os.ReadFile(execPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != versionScript("1.2.0") {
		t.Errorf("executable was not replaced with the new version, got %q", data)
	}
}

func TestApply_Timeout(t *testing.T) {
	fs := newFixtureServer(t, "v1.2.0")
	pub, _ := fs.setValidRelease(t, testGOOS, testGOARCH, "1.2.0")
	fs.overrides[AssetName(testGOOS, testGOARCH)] = func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write(fs.assets[AssetName(testGOOS, testGOARCH)])
	}

	check, err := CheckForUpdate(context.Background(), fs.client(), "1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	execPath := setupExecutable(t)
	original := readFile(t, execPath)

	client := fs.client()
	client.HTTPClient = &http.Client{Timeout: 50 * time.Millisecond}
	opts := Options{Client: client, Verifier: Verifier{TrustedKeys: []ed25519.PublicKey{pub}}, ExecPath: execPath}

	if err := Apply(context.Background(), opts, check); err == nil {
		t.Fatal("expected a timeout error")
	}
	assertUnchanged(t, execPath, original)
}

func TestApply_CorruptArchive(t *testing.T) {
	fs := newFixtureServer(t, "v1.2.0")
	pub, _ := fs.setValidRelease(t, testGOOS, testGOARCH, "1.2.0")
	// Serve different bytes than the ones checksums.txt was computed
	// against, simulating corruption in transit.
	fs.overrides[AssetName(testGOOS, testGOARCH)] = func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not a valid tar.gz at all"))
	}

	check, err := CheckForUpdate(context.Background(), fs.client(), "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	execPath := setupExecutable(t)
	original := readFile(t, execPath)

	opts := Options{Client: fs.client(), Verifier: Verifier{TrustedKeys: []ed25519.PublicKey{pub}}, ExecPath: execPath}
	if err := Apply(context.Background(), opts, check); err == nil {
		t.Fatal("expected a checksum verification error for a corrupted archive")
	}
	assertUnchanged(t, execPath, original)
}

func TestApply_BadChecksum(t *testing.T) {
	fs := newFixtureServer(t, "v1.2.0")
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	assetName := AssetName(testGOOS, testGOARCH)
	archive := buildTarGz(t, []tarEntry{{name: "skillbrowse", content: versionScript("1.2.0")}})
	archiveData := readFile(t, archive)

	wrongChecksums := []byte(sha256Hex([]byte("wrong")) + "  " + assetName + "\n")
	sig := ed25519.Sign(priv, wrongChecksums)
	fs.assets[assetName] = archiveData
	fs.assets[ChecksumsAssetName] = wrongChecksums
	fs.assets[SignatureAssetName] = sig

	check, err := CheckForUpdate(context.Background(), fs.client(), "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	execPath := setupExecutable(t)
	original := readFile(t, execPath)

	opts := Options{Client: fs.client(), Verifier: Verifier{TrustedKeys: []ed25519.PublicKey{pub}}, ExecPath: execPath}
	if err := Apply(context.Background(), opts, check); err == nil {
		t.Fatal("expected a checksum mismatch error")
	}
	assertUnchanged(t, execPath, original)
}

func TestApply_BadSignature(t *testing.T) {
	fs := newFixtureServer(t, "v1.2.0")
	_, _ = fs.setValidRelease(t, testGOOS, testGOARCH, "1.2.0")
	untrustedPub, _, _ := ed25519.GenerateKey(rand.Reader) // NOT the key that signed the release

	check, err := CheckForUpdate(context.Background(), fs.client(), "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	execPath := setupExecutable(t)
	original := readFile(t, execPath)

	opts := Options{Client: fs.client(), Verifier: Verifier{TrustedKeys: []ed25519.PublicKey{untrustedPub}}, ExecPath: execPath}
	if err := Apply(context.Background(), opts, check); err == nil {
		t.Fatal("expected a signature verification error")
	}
	assertUnchanged(t, execPath, original)
}

func TestCheckForUpdate_WrongAsset(t *testing.T) {
	fs := newFixtureServer(t, "v1.2.0")
	// Only publish an asset for an unrelated OS/arch pair.
	fs.setValidRelease(t, "plan9", "mips", "1.2.0")

	if _, err := CheckForUpdate(context.Background(), fs.client(), "1.0.0"); err == nil {
		t.Fatal("expected an error when no asset matches this OS/arch")
	}
}

func TestApply_NonWritableTarget(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission checks")
	}

	fs := newFixtureServer(t, "v1.2.0")
	pub, _ := fs.setValidRelease(t, testGOOS, testGOARCH, "1.2.0")

	check, err := CheckForUpdate(context.Background(), fs.client(), "1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	readOnlyDir := t.TempDir()
	execPath := filepath.Join(readOnlyDir, "skillbrowse")
	writeFile(t, execPath, []byte(versionScript("1.0.0")), 0o755)
	if err := os.Chmod(readOnlyDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnlyDir, 0o755) })

	original := readFile(t, execPath)
	opts := Options{Client: fs.client(), Verifier: Verifier{TrustedKeys: []ed25519.PublicKey{pub}}, ExecPath: execPath}
	if err := Apply(context.Background(), opts, check); err == nil {
		t.Fatal("expected a preflight error for a non-writable install directory")
	}
	assertUnchanged(t, execPath, original)
}

func TestApply_InterruptedDownload(t *testing.T) {
	fs := newFixtureServer(t, "v1.2.0")
	pub, _ := fs.setValidRelease(t, testGOOS, testGOARCH, "1.2.0")

	full := fs.assets[AssetName(testGOOS, testGOARCH)]
	fs.overrides[AssetName(testGOOS, testGOARCH)] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(full)))
		_, _ = w.Write(full[:len(full)/2]) // write half, then stop — no error, just an incomplete body
	}

	check, err := CheckForUpdate(context.Background(), fs.client(), "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	execPath := setupExecutable(t)
	original := readFile(t, execPath)

	opts := Options{Client: fs.client(), Verifier: Verifier{TrustedKeys: []ed25519.PublicKey{pub}}, ExecPath: execPath}
	if err := Apply(context.Background(), opts, check); err == nil {
		t.Fatal("expected an error for an interrupted/incomplete download")
	}
	assertUnchanged(t, execPath, original)
}

// TestApply_RollbackInvariant is the "rollback invariant test" from
// design doc §14.4: every updater failure before the final rename must
// leave the original executable byte-for-byte unchanged. It sweeps
// through each failure mode and re-checks the invariant for all of them.
func TestApply_RollbackInvariant(t *testing.T) {
	scenarios := map[string]func(fs *fixtureServer, pub ed25519.PublicKey) Options{
		"bad signature": func(fs *fixtureServer, _ ed25519.PublicKey) Options {
			untrusted, _, _ := ed25519.GenerateKey(rand.Reader)
			return Options{Client: fs.client(), Verifier: Verifier{TrustedKeys: []ed25519.PublicKey{untrusted}}}
		},
		"corrupt archive": func(fs *fixtureServer, pub ed25519.PublicKey) Options {
			fs.overrides[AssetName(testGOOS, testGOARCH)] = func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("corrupted"))
			}
			return Options{Client: fs.client(), Verifier: Verifier{TrustedKeys: []ed25519.PublicKey{pub}}}
		},
	}

	for name, setup := range scenarios {
		t.Run(name, func(t *testing.T) {
			fs := newFixtureServer(t, "v1.2.0")
			pub, _ := fs.setValidRelease(t, testGOOS, testGOARCH, "1.2.0")

			check, err := CheckForUpdate(context.Background(), fs.client(), "1.0.0")
			if err != nil {
				t.Fatal(err)
			}
			execPath := setupExecutable(t)
			original := readFile(t, execPath)

			opts := setup(fs, pub)
			opts.ExecPath = execPath
			if err := Apply(context.Background(), opts, check); err == nil {
				t.Fatalf("scenario %q: expected Apply to fail", name)
			}
			assertUnchanged(t, execPath, original)
		})
	}
}

// testGOOS/testGOARCH let fixtures target "the current platform".
var testGOOS, testGOARCH = runtime.GOOS, runtime.GOARCH

func setupExecutable(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "skillbrowse")
	writeFile(t, path, []byte(versionScript("1.0.0")), 0o755)
	return path
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertUnchanged(t *testing.T, path string, original []byte) {
	t.Helper()
	got := readFile(t, path)
	if string(got) != string(original) {
		t.Errorf("executable at %s was modified despite the failure; rollback invariant violated", path)
	}
}
