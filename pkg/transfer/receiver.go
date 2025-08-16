package transfer

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"io"
	"os"

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

	// Read the file key (32 bytes for AES-256)
	fileKey := make([]byte, 32)
	if _, err := io.ReadFull(conn, fileKey); err != nil {
		return fmt.Errorf("failed to read file key: %w", err)
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

	// Read the nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(conn, nonce); err != nil {
		return fmt.Errorf("failed to read nonce: %w", err)
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

		// Decrypt the chunk
		plaintext, err := gcm.Open(nil, nonce, buffer[:chunkLen], nil)
		if err != nil {
			return fmt.Errorf("decryption failed: %w", err)
		}

		// Write the decrypted data to file
		if _, err := file.Write(plaintext); err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}
	fmt.Println("File received successfully:", manifest.FileName)
	return nil
}
