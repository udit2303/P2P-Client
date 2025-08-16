package transfer

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
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
func SendFile(conn io.Writer, filePath string) error {
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

	// Send file key
	if _, err := conn.Write(fileKey); err != nil {
		return fmt.Errorf("failed to send file key: %w", err)
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

	// Send nonce first
	if _, err := conn.Write(nonce); err != nil {
		return fmt.Errorf("failed to send nonce: %w", err)
	}

	// Buffer for reading chunks (64KB - GCM overhead)
	chunkSize := 64*1024 - 28 // 64KB - 28 bytes for GCM overhead
	buffer := make([]byte, chunkSize)

	for {
		// Read chunk
		n, err := file.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("read error: %w", err)
		}

		// Encrypt chunk
		ciphertext := gcm.Seal(nil, nonce, buffer[:n], nil)

		// Send chunk length
		if err := binary.Write(conn, binary.BigEndian, uint32(len(ciphertext))); err != nil {
			return fmt.Errorf("failed to send chunk size: %w", err)
		}

		// Send encrypted chunk
		if _, err := conn.Write(ciphertext); err != nil {
			return fmt.Errorf("failed to send chunk: %w", err)
		}
	}

	// Send a zero-length chunk to signal end of file
	if err := binary.Write(conn, binary.BigEndian, uint32(0)); err != nil {
		return fmt.Errorf("failed to send EOF marker: %w", err)
	}

	return nil
}
