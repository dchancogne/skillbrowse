package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"testing"
)

func TestVerifier_AcceptsValidSignature(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	manifest := []byte("sha256sums")
	sig := ed25519.Sign(priv, manifest)

	v := Verifier{TrustedKeys: []ed25519.PublicKey{pub}}
	if err := v.VerifyManifestSignature(manifest, sig); err != nil {
		t.Errorf("expected valid signature to verify, got %v", err)
	}
}

func TestVerifier_RejectsWrongKey(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	otherPub, _, _ := ed25519.GenerateKey(rand.Reader)
	manifest := []byte("sha256sums")
	sig := ed25519.Sign(priv, manifest)

	v := Verifier{TrustedKeys: []ed25519.PublicKey{otherPub}}
	if err := v.VerifyManifestSignature(manifest, sig); err == nil {
		t.Error("expected verification to fail against an untrusted key")
	}
}

func TestVerifier_RejectsTamperedManifest(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	manifest := []byte("sha256sums")
	sig := ed25519.Sign(priv, manifest)

	v := Verifier{TrustedKeys: []ed25519.PublicKey{pub}}
	if err := v.VerifyManifestSignature([]byte("sha256sums-tampered"), sig); err == nil {
		t.Error("expected verification to fail against a tampered manifest")
	}
}

func TestVerifier_KeyRotationAcceptsOldOrNewKey(t *testing.T) {
	oldPub, oldPriv, _ := ed25519.GenerateKey(rand.Reader)
	newPub, _, _ := ed25519.GenerateKey(rand.Reader)
	manifest := []byte("sha256sums")
	sig := ed25519.Sign(oldPriv, manifest) // signed with the OLD key only

	v := Verifier{TrustedKeys: []ed25519.PublicKey{oldPub, newPub}} // both embedded
	if err := v.VerifyManifestSignature(manifest, sig); err != nil {
		t.Errorf("expected rotation set to still accept the old key's signature: %v", err)
	}
}

func TestVerifier_NoTrustedKeysFails(t *testing.T) {
	v := Verifier{}
	if err := v.VerifyManifestSignature([]byte("x"), []byte("y")); err == nil {
		t.Error("expected failure with no trusted keys configured")
	}
}

func TestDefaultVerifier_EmbeddedKeyIsWellFormed(t *testing.T) {
	v := DefaultVerifier()
	if len(v.TrustedKeys) == 0 {
		t.Fatal("expected at least one embedded trusted key")
	}
	for _, k := range v.TrustedKeys {
		if len(k) != ed25519.PublicKeySize {
			t.Errorf("embedded key has length %d, want %d", len(k), ed25519.PublicKeySize)
		}
	}
}

func TestDefaultVerifier_VerifiesASignatureFromTheMatchingPrivateKey(t *testing.T) {
	// This doesn't (and can't) exercise the real embedded key, since its
	// private half deliberately isn't in this repository — it's a
	// structural check that DefaultVerifier's key decoding round-trips
	// through ed25519.Verify correctly in general, using the same
	// mustDecodeKeys path the embedded key goes through.
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	v := Verifier{TrustedKeys: mustDecodeKeys(hex.EncodeToString(pub))}
	manifest := []byte("sha256sums")
	if err := v.VerifyManifestSignature(manifest, ed25519.Sign(priv, manifest)); err != nil {
		t.Errorf("round-tripped key failed to verify: %v", err)
	}
}
