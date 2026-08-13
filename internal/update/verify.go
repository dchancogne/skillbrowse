package update

import (
	"crypto/ed25519"
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

// defaultTrustedKeys are the Ed25519 public keys embedded in official
// binaries. This is a development placeholder: Phase 6's signed release
// workflow must generate the real signing key pair, commit only the
// public key here, and keep the private key in the release environment's
// secret storage — never in this repository.
var defaultTrustedKeys []ed25519.PublicKey

// DefaultVerifier returns a Verifier using the embedded trusted keys.
func DefaultVerifier() Verifier {
	return Verifier{TrustedKeys: defaultTrustedKeys}
}
