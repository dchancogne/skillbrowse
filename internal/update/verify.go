package update

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
)

// Verifier holds the Ed25519 public keys this binary trusts to sign a
// checksums.txt manifest. A signature is accepted if it verifies against
// ANY trusted key, which is what makes key rotation work per design doc
// §12.3: "a release signed by both the old and new keys... embeds both
// public keys."
type Verifier struct {
	TrustedKeys []ed25519.PublicKey
}

// VerifyManifestSignature checks sig against manifest using any trusted
// key.
func (v Verifier) VerifyManifestSignature(manifest, sig []byte) error {
	if len(v.TrustedKeys) == 0 {
		return fmt.Errorf("no trusted signing keys configured")
	}
	for _, key := range v.TrustedKeys {
		if ed25519.Verify(key, manifest, sig) {
			return nil
		}
	}
	return fmt.Errorf("checksums.txt signature does not verify against any trusted key")
}

// signingKeyHex is the hex-encoded Ed25519 public key official binaries
// trust to verify checksums.txt, per design doc §12.3. The matching
// private key lives only in the release workflow's secret storage
// (GitHub Actions secret SKILLBROWSE_SIGNING_KEY) — never in this
// repository. On key rotation, add the new key's hex here alongside
// this one (don't replace it) so releases signed with either key still
// verify, then retire this one in a later release per §12.3's rotation
// procedure.
const signingKeyHex = "b9991b8853a2b28d346199513d55850261eaa9667a66d1b53b154ac80eed5f3f"

var defaultTrustedKeys = mustDecodeKeys(signingKeyHex)

func mustDecodeKeys(hexKeys ...string) []ed25519.PublicKey {
	keys := make([]ed25519.PublicKey, len(hexKeys))
	for i, h := range hexKeys {
		raw, err := hex.DecodeString(h)
		if err != nil {
			panic(fmt.Sprintf("update: invalid embedded signing key: %s", err))
		}
		if len(raw) != ed25519.PublicKeySize {
			panic(fmt.Sprintf("update: embedded signing key has wrong length: got %d, want %d", len(raw), ed25519.PublicKeySize))
		}
		keys[i] = ed25519.PublicKey(raw)
	}
	return keys
}

// DefaultVerifier returns a Verifier using the embedded trusted keys.
func DefaultVerifier() Verifier {
	return Verifier{TrustedKeys: defaultTrustedKeys}
}
