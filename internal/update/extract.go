package update

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// maxArchiveEntries bounds tar-bomb style archives with an absurd entry
// count.
const maxArchiveEntries = 10000

// extractExecutable safely extracts the single archive entry named
// binaryName from the tar.gz at archivePath into a new file in destDir,
// per design doc §12.3 step 4: reject absolute paths, "..", symlinks,
// extra executable candidates, and oversized entries. Every entry in the
// archive is validated, not just the one being extracted, since a
// malicious archive could hide an escape in an unrelated entry.
func extractExecutable(archivePath, binaryName, destDir string, maxSize int64) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("open gzip stream: %w", err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)

	var stagedPath string
	found := false

	for i := 0; ; i++ {
		if i >= maxArchiveEntries {
			cleanup(stagedPath)
			return "", fmt.Errorf("archive has more than %d entries", maxArchiveEntries)
		}

		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			cleanup(stagedPath)
			return "", fmt.Errorf("read archive: %w", err)
		}

		if err := validateEntryName(hdr.Name); err != nil {
			cleanup(stagedPath)
			return "", err
		}

		switch hdr.Typeflag {
		case tar.TypeSymlink, tar.TypeLink:
			cleanup(stagedPath)
			return "", fmt.Errorf("archive entry %q is a symlink, which is not allowed", hdr.Name)
		case tar.TypeReg:
			if filepath.Base(hdr.Name) != binaryName {
				continue
			}
			if found {
				cleanup(stagedPath)
				return "", fmt.Errorf("archive contains more than one entry named %q", binaryName)
			}
			staged, err := extractEntry(tr, hdr, destDir, maxSize)
			if err != nil {
				return "", err
			}
			stagedPath = staged
			found = true
		default:
			continue
		}
	}

	if !found {
		return "", fmt.Errorf("archive does not contain expected executable %q", binaryName)
	}
	return stagedPath, nil
}

func extractEntry(tr *tar.Reader, hdr *tar.Header, destDir string, maxSize int64) (string, error) {
	if hdr.Size > maxSize {
		return "", fmt.Errorf("archive entry %q (%d bytes) exceeds the %d byte limit", hdr.Name, hdr.Size, maxSize)
	}

	out, err := os.CreateTemp(destDir, ".skillbrowse-update-*")
	if err != nil {
		return "", fmt.Errorf("create staging file: %w", err)
	}
	stagedPath := out.Name()

	n, copyErr := io.Copy(out, io.LimitReader(tr, maxSize+1))
	closeErr := out.Close()

	if copyErr != nil {
		cleanup(stagedPath)
		return "", fmt.Errorf("extract %q: %w", hdr.Name, copyErr)
	}
	if closeErr != nil {
		cleanup(stagedPath)
		return "", closeErr
	}
	if n > maxSize {
		cleanup(stagedPath)
		return "", fmt.Errorf("archive entry %q exceeds the %d byte limit", hdr.Name, maxSize)
	}

	if err := os.Chmod(stagedPath, 0o755); err != nil {
		cleanup(stagedPath)
		return "", fmt.Errorf("set executable permission: %w", err)
	}

	return stagedPath, nil
}

func validateEntryName(name string) error {
	if filepath.IsAbs(name) || (len(name) >= 2 && name[1] == ':') { // reject POSIX and Windows-style absolute paths
		return fmt.Errorf("archive entry %q has an absolute path, which is not allowed", name)
	}

	cleaned := path.Clean(name)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("archive entry %q attempts to escape the extraction directory", name)
	}
	return nil
}

func cleanup(path string) {
	if path != "" {
		_ = os.Remove(path)
	}
}
