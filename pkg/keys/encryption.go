package keys

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"
)

const (
	PrivateKeyPath = "private.pem"
	PublicKeyPath  = "public.pem"
	KeySize        = 4096
)

// GenerateRSAKeyPair generates a new RSA key pair and saves them to disk
func GenerateRSAKeyPair() error {
	// Check if private key exists
	if _, err := os.Stat(PrivateKeyPath); err == nil {
		// Private key exists, do not overwrite
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat private key file: %w", err)
	}

	// Check if public key exists
	if _, err := os.Stat(PublicKeyPath); err == nil {
		// Public key exists, do not overwrite
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat public key file: %w", err)
	}

	privKey, err := rsa.GenerateKey(rand.Reader, KeySize)
	if err != nil {
		return fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Save private key
	privFile, err := os.Create(PrivateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to create private key file: %w", err)
	}
	defer privFile.Close()
	privBytes := x509.MarshalPKCS1PrivateKey(privKey)
	privBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes}
	if err := pem.Encode(privFile, privBlock); err != nil {
		return fmt.Errorf("failed to encode private key: %w", err)
	}

	// Save public key
	pubFile, err := os.Create(PublicKeyPath)
	if err != nil {
		return fmt.Errorf("failed to create public key file: %w", err)
	}
	defer pubFile.Close()
	pubBytes := x509.MarshalPKCS1PublicKey(&privKey.PublicKey)
	pubBlock := &pem.Block{Type: "RSA PUBLIC KEY", Bytes: pubBytes}
	if err := pem.Encode(pubFile, pubBlock); err != nil {
		return fmt.Errorf("failed to encode public key: %w", err)
	}

	return nil
}

// LoadPrivateKey loads the RSA private key from disk
func LoadPrivateKey() (*rsa.PrivateKey, error) {

	privFile, err := os.Open(PrivateKeyPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := GenerateRSAKeyPair(); err != nil {
				return nil, fmt.Errorf("failed to generate RSA key pair: %w", err)
			}
			// Try opening again after generating
			privFile, err = os.Open(PrivateKeyPath)
			if err != nil {
				return nil, fmt.Errorf("failed to open private key file after generation: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to open private key file: %w", err)
		}
	}
	defer privFile.Close()
	pemBytes, err := io.ReadAll(privFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("invalid private key PEM")
	}
	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	return privKey, nil
}

// LoadPublicKey loads the RSA public key from disk
func LoadPublicKey() (*rsa.PublicKey, error) {
	pubFile, err := os.Open(PublicKeyPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := GenerateRSAKeyPair(); err != nil {
				return nil, fmt.Errorf("failed to generate RSA key pair: %w", err)
			}
			// Try opening again after generating
			pubFile, err = os.Open(PublicKeyPath)
			if err != nil {
				return nil, fmt.Errorf("failed to open private key file after generation: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to open private key file: %w", err)
		}
	}
	defer pubFile.Close()
	pemBytes, err := io.ReadAll(pubFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "RSA PUBLIC KEY" {
		return nil, fmt.Errorf("invalid public key PEM")
	}
	pubKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}
	return pubKey, nil
}

func GenerateRandomKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

func EncryptData(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}
