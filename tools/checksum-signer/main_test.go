package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// These tests use a freshly generated throwaway key pair, never the
// real production signing key (which deliberately isn't in this repo).

func TestRun_SignsAndVerifies(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLBROWSE_SIGNING_KEY", hex.EncodeToString(priv))

	dir := t.TempDir()
	checksums := filepath.Join(dir, "checksums.txt")
	content := []byte("abc123  skillbrowse_darwin_arm64.tar.gz\n")
	if err := os.WriteFile(checksums, content, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"sign", checksums}); err != nil {
		t.Fatal(err)
	}

	sig, err := os.ReadFile(checksums + ".sig")
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(pub, content, sig) {
		t.Error("produced signature does not verify against the signing key's public half")
	}
}

func TestRun_MissingKeyErrors(t *testing.T) {
	t.Setenv("SKILLBROWSE_SIGNING_KEY", "")
	dir := t.TempDir()
	checksums := filepath.Join(dir, "checksums.txt")
	if err := os.WriteFile(checksums, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"sign", checksums}); err == nil {
		t.Error("expected an error when SKILLBROWSE_SIGNING_KEY is unset")
	}
}

func TestRun_InvalidKeyHexErrors(t *testing.T) {
	t.Setenv("SKILLBROWSE_SIGNING_KEY", "not-hex")
	dir := t.TempDir()
	checksums := filepath.Join(dir, "checksums.txt")
	if err := os.WriteFile(checksums, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := run([]string{"sign", checksums}); err == nil {
		t.Error("expected an error for non-hex key material")
	}
}

func TestRun_WrongArgsErrors(t *testing.T) {
	cases := [][]string{
		nil,
		{"sign"},
		{"verify", "x"},
		{"sign", "a", "b"},
	}
	for _, args := range cases {
		if err := run(args); err == nil {
			t.Errorf("run(%v) expected an error", args)
		}
	}
}

func TestRun_MissingFileErrors(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	t.Setenv("SKILLBROWSE_SIGNING_KEY", hex.EncodeToString(priv))

	if err := run([]string{"sign", filepath.Join(t.TempDir(), "does-not-exist.txt")}); err == nil {
		t.Error("expected an error for a missing input file")
	}
}
