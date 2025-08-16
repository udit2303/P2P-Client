package transfer

import (
	"fmt"
	"io"
	"os"

	"github.com/udit2303/p2p-client/pkg/util"
)

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

	// Send manifest length first
	if err := util.SendWithLength(conn, manifestBytes); err != nil {
		return fmt.Errorf("failed to send manifest: %w", err)
	}

	// Send file data
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	buf := make([]byte, 4096)
	for {
		n, err := file.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println("Read Error", err)
		}
		_, writeErr := conn.Write(buf[:n])
		if writeErr != nil {
			fmt.Println("Write Error", writeErr)
		}
	}

	return nil
}
