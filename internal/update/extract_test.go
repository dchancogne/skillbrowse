package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

type tarEntry struct {
	name     string
	typeflag byte
	linkname string
	content  string
}

func buildTarGz(t *testing.T, entries []tarEntry) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for _, e := range entries {
		hdr := &tar.Header{
			Name:     e.name,
			Typeflag: e.typeflag,
			Linkname: e.linkname,
			Size:     int64(len(e.content)),
			Mode:     0o755,
		}
		if hdr.Typeflag == 0 {
			hdr.Typeflag = tar.TypeReg
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if hdr.Typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(e.content)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "archive.tar.gz")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractExecutable_HappyPath(t *testing.T) {
	archive := buildTarGz(t, []tarEntry{
		{name: "README.md", content: "hello"},
		{name: "skillbrowse", content: "#!/bin/sh\necho hi\n"},
	})

	staged, err := extractExecutable(archive, "skillbrowse", t.TempDir(), maxArchiveSize)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(staged)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "#!/bin/sh\necho hi\n" {
		t.Errorf("staged content = %q", data)
	}
	info, err := os.Stat(staged)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0o100 == 0 {
		t.Error("expected staged file to be executable")
	}
}

func TestExtractExecutable_RejectsAbsolutePath(t *testing.T) {
	archive := buildTarGz(t, []tarEntry{{name: "/etc/skillbrowse", content: "x"}})
	if _, err := extractExecutable(archive, "skillbrowse", t.TempDir(), maxArchiveSize); err == nil {
		t.Error("expected rejection of an absolute path entry")
	}
}

func TestExtractExecutable_RejectsPathTraversal(t *testing.T) {
	archive := buildTarGz(t, []tarEntry{{name: "../../etc/passwd", content: "x"}})
	if _, err := extractExecutable(archive, "skillbrowse", t.TempDir(), maxArchiveSize); err == nil {
		t.Error("expected rejection of a path-traversal entry")
	}
}

func TestExtractExecutable_RejectsSymlink(t *testing.T) {
	archive := buildTarGz(t, []tarEntry{{name: "skillbrowse", typeflag: tar.TypeSymlink, linkname: "/bin/sh"}})
	if _, err := extractExecutable(archive, "skillbrowse", t.TempDir(), maxArchiveSize); err == nil {
		t.Error("expected rejection of a symlink entry")
	}
}

func TestExtractExecutable_RejectsMultipleCandidates(t *testing.T) {
	archive := buildTarGz(t, []tarEntry{
		{name: "skillbrowse", content: "one"},
		{name: "nested/skillbrowse", content: "two"},
	})
	if _, err := extractExecutable(archive, "skillbrowse", t.TempDir(), maxArchiveSize); err == nil {
		t.Error("expected rejection when more than one entry matches the binary name")
	}
}

func TestExtractExecutable_RejectsOversizedEntry(t *testing.T) {
	archive := buildTarGz(t, []tarEntry{{name: "skillbrowse", content: "this content is longer than the size cap below"}})
	if _, err := extractExecutable(archive, "skillbrowse", t.TempDir(), 10); err == nil {
		t.Error("expected rejection of an entry exceeding the size cap")
	}
}

func TestExtractExecutable_MissingBinaryIsAnError(t *testing.T) {
	archive := buildTarGz(t, []tarEntry{{name: "README.md", content: "hello"}})
	if _, err := extractExecutable(archive, "skillbrowse", t.TempDir(), maxArchiveSize); err == nil {
		t.Error("expected an error when the archive has no matching entry")
	}
}
