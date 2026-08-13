package update

import (
	"strings"
	"testing"
)

func TestParseChecksums(t *testing.T) {
	data := []byte("abc123  skillbrowse_darwin_arm64.tar.gz\ndef456  checksums-other.tar.gz\n")
	sums, err := parseChecksums(data)
	if err != nil {
		t.Fatal(err)
	}
	if sums["skillbrowse_darwin_arm64.tar.gz"] != "abc123" {
		t.Errorf("got %q", sums["skillbrowse_darwin_arm64.tar.gz"])
	}
	if len(sums) != 2 {
		t.Errorf("expected 2 entries, got %d", len(sums))
	}
}

func TestParseChecksums_MalformedLine(t *testing.T) {
	_, err := parseChecksums([]byte("not-a-valid-line-with-only-one-field\n"))
	if err == nil {
		t.Fatal("expected an error for a malformed line")
	}
}

func TestVerifyChecksum(t *testing.T) {
	content := []byte("hello world")
	// sha256("hello world")
	const want = "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"

	if err := verifyChecksum(strings.NewReader(string(content)), want); err != nil {
		t.Errorf("expected match, got %v", err)
	}
	if err := verifyChecksum(strings.NewReader(string(content)), "0000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Error("expected mismatch error")
	}
}
