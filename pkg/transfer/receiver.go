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
	"strings"
	"time"

	"github.com/udit2303/p2p-client/pkg/keys"
	"github.com/udit2303/p2p-client/pkg/util"
)

// ReceiveFile receives a file and its manifest from the given connection
func ReceiveFile(conn io.Reader, outputDir string) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
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

	// Initialize progress tracking
	var totalReceived int64 = 0
	lastUpdate := time.Now()
	var lastBytes int64 = 0
	var speed float64 = 0
	var eta float64 = 0

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
			e := os.Remove(outputPath)
			if e != nil {
				return fmt.Errorf("deleting file failed: %w", e)
			}
			return fmt.Errorf("deleting file, failed to read chunk: %w", err)
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

		// Update progress
		totalReceived += int64(len(plaintext))
		now := time.Now()
		if now.Sub(lastUpdate) > 100*time.Millisecond {
			delta := totalReceived - lastBytes
			deltaTime := now.Sub(lastUpdate).Seconds()
			if deltaTime > 0 {
				speed = float64(delta) / deltaTime
				if speed > 0 {
					eta = float64(manifest.FileSize-totalReceived) / speed
				}
			}
			lastUpdate = now
			lastBytes = totalReceived
			percent := float64(totalReceived) / float64(manifest.FileSize) * 100

			// Format ETA with duration rounding
			etaDuration := time.Duration(eta) * time.Second
			etaStr := "--:--"
			if eta > 0 {
				etaStr = fmt.Sprintf("%02d:%02d", int(etaDuration.Minutes()), int(etaDuration.Seconds())%60)
			}

			fmt.Printf("\rReceiving: %s [%s] %.1f%% - %s/s - ETA: %s",
				manifest.FileName,
				progressBar(percent, 20),
				percent,
				formatBytes(speed),
				etaStr,
			)
		}

		// Increment counter to match sender's per-chunk nonce
		counter++
	}
	// Print final progress
	fmt.Printf("\rReceiving: %s [%s] 100%% - Complete!%s\n",
		manifest.FileName,
		progressBar(100, 20),
		strings.Repeat(" ", 20), // Clear any remaining characters
	)
	fmt.Println("File received successfully:", manifest.FileName)
	return nil
}
