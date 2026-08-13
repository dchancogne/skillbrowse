package update

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// parseChecksums parses a GoReleaser-style checksums.txt: one
// "<sha256hex>  <filename>" pair per line.
func parseChecksums(data []byte) (map[string]string, error) {
	sums := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("malformed checksums line: %q", line)
		}
		sums[fields[1]] = strings.ToLower(fields[0])
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return sums, nil
}

// verifyChecksum reports whether r's SHA-256 digest matches wantHex.
func verifyChecksum(r io.Reader, wantHex string) error {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return fmt.Errorf("hash archive: %w", err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, wantHex) {
		return fmt.Errorf("checksum mismatch: got %s, want %s", got, wantHex)
	}
	return nil
}
