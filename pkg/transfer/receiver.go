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

	// Receive file data
	written, err := io.CopyN(file, conn, manifest.FileSize)
	if err != nil {
		return fmt.Errorf("failed to receive file: %w", err)
	}
	if written != manifest.FileSize {
		return fmt.Errorf("incomplete file transfer")
	}

	return nil
}
