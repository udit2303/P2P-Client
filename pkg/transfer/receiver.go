package transfer

import (
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

	// Prepare output file path
	outputPath := outputDir + "/" + manifest.FileName
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	buf := make([]byte, 4096)
	var received int64 = 0
	for received < manifest.FileSize {
		toRead := int64(len(buf))
		if received+toRead > manifest.FileSize {
			toRead = manifest.FileSize - received
		}
		n, err := conn.Read(buf[:toRead])
		if n > 0 {
			_, writeErr := file.Write(buf[:n])
			if writeErr != nil {
				return writeErr
			}
			received += int64(n)
		}

		if err != nil {
			if err == io.EOF && received == manifest.FileSize {
				break
			}
			return err
		}
	}
	if received != manifest.FileSize {
		return fmt.Errorf("incomplete file transfer")
	}
	fmt.Println("File received successfully:", manifest.FileName)
	return nil
}
