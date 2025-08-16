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

// ReceiveFile receives a file and its manifest from the given connection
func ReceiveFile(conn io.Reader, outputDir string) error {
	// Read manifest
	manifestBytes, err := util.ReadWithLength(conn)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	manifest, err := DeserializeManifest(manifestBytes)
	if err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Read sender public key (not strictly necessary for decryption, but useful for identification)
	senderPubBytes, err := util.ReadWithLength(conn)
	if err != nil {
		return fmt.Errorf("failed to read sender public key: %w", err)
	}
	// Optionally parse sender public key
	_, err = x509.ParsePKCS1PublicKey(senderPubBytes)
	if err != nil {
		return fmt.Errorf("failed to parse sender public key")
	}

	// Read encrypted session key and decrypt using our private key
	encryptedKey, err := util.ReadWithLength(conn)
	if err != nil {
		return fmt.Errorf("failed to read encrypted file key: %w", err)
	}
	priv, err := keys.LoadPrivateKey()
	if err != nil {
		return fmt.Errorf("failed to load private key: %w", err)
	}
	fileKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, priv, encryptedKey, nil)
	if err != nil {
		return fmt.Errorf("failed to decrypt file key: %w", err)
	}
	// Initialize decryption
	block, err := aes.NewCipher(fileKey)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	// Read base nonce (sent with length framing)
	nonce, err := util.ReadWithLength(conn)
	if err != nil {
		return fmt.Errorf("failed to read nonce: %w", err)
	}
	if len(nonce) != gcm.NonceSize() {
		return fmt.Errorf("invalid nonce size: expected %d, got %d", gcm.NonceSize(), len(nonce))
	}

	// Create output file
	outputPath := outputDir + "/" + manifest.FileName
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Buffer for chunks
	buffer := make([]byte, 64*1024) // Max possible chunk size

	var counter uint32 = 0
	for {
		// Read chunk length
		var chunkLen uint32
		if err := binary.Read(conn, binary.BigEndian, &chunkLen); err != nil {
			return fmt.Errorf("failed to read chunk length: %w", err)
		}

		// Check for EOF marker
		if chunkLen == 0 {
			break
		}

		// Read the encrypted chunk
		if _, err := io.ReadFull(conn, buffer[:chunkLen]); err != nil {
			return fmt.Errorf("failed to read chunk: %w", err)
		}

		// Derive per-chunk nonce matching sender's scheme
		chunkNonce := make([]byte, len(nonce))
		copy(chunkNonce, nonce)
		binary.BigEndian.PutUint32(chunkNonce[len(chunkNonce)-4:], counter)

		// Decrypt the chunk
		plaintext, err := gcm.Open(nil, chunkNonce, buffer[:chunkLen], nil)
		if err != nil {
			return fmt.Errorf("decryption failed: %w", err)
		}

		// Write the decrypted data to file
		if _, err := file.Write(plaintext); err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}

		// Increment counter to match sender's per-chunk nonce
		counter++
	}
	fmt.Println("File received successfully:", manifest.FileName)
	return nil
}
