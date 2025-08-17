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
// receiverPubKey must be the receiver's RSA public key used to encrypt the session key.
func SendFile(conn io.Writer, filePath string, receiverPubKey *rsa.PublicKey) error {
	// Create progress tracker
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	progress := NewProgress(info.Name(), info.Size())
	defer fmt.Println() // Ensure we end the progress line
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
	lastUpdate := time.Now()
	var lastBytes int64 = 0
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

		// Update progress
		progress.Transferred += int64(n)
		now := time.Now()
		if now.Sub(lastUpdate) > 100*time.Millisecond {
			delta := progress.Transferred - lastBytes
			deltaTime := now.Sub(lastUpdate).Seconds()
			if deltaTime > 0 {
				progress.Speed = float64(delta) / deltaTime
				if progress.Speed > 0 {
					progress.ETA = float64(progress.FileSize-progress.Transferred) / progress.Speed
				}
			}
			lastUpdate = now
			lastBytes = progress.Transferred
			// Format ETA with duration rounding
			duration := time.Duration(progress.ETA) * time.Second
			etaStr := "--:--"
			if progress.ETA > 0 {
				etaStr = fmt.Sprintf("%02d:%02d", int(duration.Minutes()), int(duration.Seconds())%60)
			}

			fmt.Printf("\rSending: %s [%s] %.1f%% - %s/s - ETA: %s",
				progress.FileName,
				progressBar(progress.Percent(), 20),
				progress.Percent(),
				formatBytes(progress.Speed),
				etaStr,
			)
		}

		// Increment counter for next chunk
		counter++
	}

	// Send a zero-length chunk to signal end of file
	if err := binary.Write(conn, binary.BigEndian, uint32(0)); err != nil {
		return fmt.Errorf("failed to send EOF marker: %w", err)
	}
	// Print final progress
	fmt.Printf("\rSending: %s [%s] 100%% - Complete!%s\n",
		progress.FileName,
		progressBar(100, 20),
		strings.Repeat(" ", 20), // Clear any remaining characters
	)

	return nil
}
