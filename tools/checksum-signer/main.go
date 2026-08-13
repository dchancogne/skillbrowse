// Command checksum-signer signs a GoReleaser checksums.txt manifest with
// the Ed25519 private key half of the key pair skillbrowse binaries
// trust (see internal/update/verify.go), for the release workflow's
// GoReleaser `signs:` step.
//
// The private key is read from the SKILLBROWSE_SIGNING_KEY environment
// variable (hex-encoded, never a command-line argument, so it never
// appears in process listings or shell history) and is expected to live
// only in the release workflow's secret storage.
//
// Usage:
//
//	SKILLBROWSE_SIGNING_KEY=<hex> checksum-signer sign <path/to/checksums.txt>
//
// Writes the raw Ed25519 signature bytes to <path>.sig.
package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "checksum-signer:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 2 || args[0] != "sign" {
		return fmt.Errorf("usage: checksum-signer sign <path/to/checksums.txt>")
	}
	path := args[1]

	keyHex := os.Getenv("SKILLBROWSE_SIGNING_KEY")
	if keyHex == "" {
		return fmt.Errorf("SKILLBROWSE_SIGNING_KEY is not set")
	}
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		return fmt.Errorf("SKILLBROWSE_SIGNING_KEY is not valid hex: %w", err)
	}
	if len(keyBytes) != ed25519.PrivateKeySize {
		return fmt.Errorf("SKILLBROWSE_SIGNING_KEY has wrong length: got %d bytes, want %d", len(keyBytes), ed25519.PrivateKeySize)
	}
	priv := ed25519.PrivateKey(keyBytes)

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	sig := ed25519.Sign(priv, data)

	sigPath := path + ".sig"
	if err := os.WriteFile(sigPath, sig, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", sigPath, err)
	}

	fmt.Println(sigPath)
	return nil
}
