package transfer

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/udit2303/p2p-client/pkg/keys"
	"github.com/udit2303/p2p-client/pkg/util"
)

func encryptFile(filePath string, key []byte) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	plaintext, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	encryptedData, err := keys.EncryptData(plaintext, key)
	if err != nil {
		return nil, fmt.Errorf("encryption failed: %w", err)
	}
	return encryptedData, nil
}

// SendFile sends a file with its manifest over the given connection
// SendFile sends a file with its manifest over the given connection.
// receiverPubKey must be the receiver's RSA public key used to encrypt the session key.
func SendFile(conn io.Writer, filePath string, receiverPubKey *rsa.PublicKey) error {
	// Create manifest
	manifest, err := CreateManifest(filePath)
	if err != nil {
		return fmt.Errorf("failed to create manifest: %w", err)
	}

	// Serialize manifest
	manifestBytes, err := SerializeManifest(manifest)
	if err != nil {
		return fmt.Errorf("failed to serialize manifest: %w", err)
	}
	//Generate session key
	fileKey, err := keys.GenerateRandomKey()
	if err != nil {
		return fmt.Errorf("failed to generate file key: %w", err)
	}

	// Send manifest length first
	if err := util.SendWithLength(conn, manifestBytes); err != nil {
		return fmt.Errorf("failed to send manifest: %w", err)
	}

	// Load sender public key and send it so receiver can identify sender
	senderPub, err := keys.LoadPublicKey()
	if err != nil {
		return fmt.Errorf("failed to load sender public key: %w", err)
	}
	senderPubBytes := x509.MarshalPKCS1PublicKey(senderPub)
	if err := util.SendWithLength(conn, senderPubBytes); err != nil {
		return fmt.Errorf("failed to send sender public key: %w", err)
	}

	// Encrypt the session (file) key with receiver's public key and send it
	encryptedKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, receiverPubKey, fileKey, nil)
	if err != nil {
		return fmt.Errorf("failed to encrypt file key: %w", err)
	}
	if err := util.SendWithLength(conn, encryptedKey); err != nil {
		return fmt.Errorf("failed to send encrypted file key: %w", err)
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Initialize encryption
	block, err := aes.NewCipher(fileKey)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Send base nonce (will derive per-chunk nonces by incrementing counter)
	if err := util.SendWithLength(conn, nonce); err != nil {
		return fmt.Errorf("failed to send nonce: %w", err)
	}

	// Buffer for reading chunks (64KB - GCM overhead)
	chunkSize := 64*1024 - 28 // 64KB - 28 bytes for GCM overhead
	buffer := make([]byte, chunkSize)

	var counter uint32 = 0
	for {
		// Read chunk
		n, err := file.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read error: %w", err)
		}

		// Derive per-chunk nonce: copy base nonce and put counter in last 4 bytes
		chunkNonce := make([]byte, len(nonce))
		copy(chunkNonce, nonce)
		// Place counter in last 4 bytes (works when nonce size >= 4)
		binary.BigEndian.PutUint32(chunkNonce[len(chunkNonce)-4:], counter)

		// Encrypt chunk with per-chunk nonce
		ciphertext := gcm.Seal(nil, chunkNonce, buffer[:n], nil)

		// Send chunk length
		if err := binary.Write(conn, binary.BigEndian, uint32(len(ciphertext))); err != nil {
			return fmt.Errorf("failed to send chunk size: %w", err)
		}

		// Send encrypted chunk
		if _, err := conn.Write(ciphertext); err != nil {
			return fmt.Errorf("failed to send chunk: %w", err)
		}

		// Increment counter for next chunk
		counter++
	}

	// Send a zero-length chunk to signal end of file
	if err := binary.Write(conn, binary.BigEndian, uint32(0)); err != nil {
		return fmt.Errorf("failed to send EOF marker: %w", err)
	}

	return nil
}
