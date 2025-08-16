package keys

import (
	"crypto/ecdh"
	"crypto/rand"
)

// GenerateKeyPair creates a new ECDH key pair
func GenerateKeyPair() (*ecdh.PrivateKey, *ecdh.PublicKey, error) {
	// Use X25519 for key exchange
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return priv, priv.PublicKey(), nil
}

// DeriveSharedSecret derives a shared secret using ECDH
func DeriveSharedSecret(priv *ecdh.PrivateKey, pub *ecdh.PublicKey) ([]byte, error) {
	return priv.ECDH(pub)
}
