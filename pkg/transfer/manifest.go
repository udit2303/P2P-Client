package transfer

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Manifest defines metadata for a transfer
type Manifest struct {
	FileName    string      `json:"file_name"`
	FileSize    int64       `json:"file_size"`
	FileMode    os.FileMode `json:"file_mode"`
	LastModTime time.Time   `json:"last_mod_time"`
	Hash        string      `json:"hash,omitempty"` // Optional checksum
}

// CreateManifest generates manifest from a local file
func CreateManifest(filePath string) (*Manifest, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("could not stat file: %w", err)
	}

	manifest := &Manifest{
		FileName:    info.Name(),
		FileSize:    info.Size(),
		FileMode:    info.Mode(),
		LastModTime: info.ModTime(),
		// Hash: generate checksum here if needed
	}
	return manifest, nil
}

// SerializeManifest converts manifest to JSON
func SerializeManifest(m *Manifest) ([]byte, error) {
	return json.Marshal(m)
}

// DeserializeManifest parses JSON into Manifest struct
func DeserializeManifest(data []byte) (*Manifest, error) {
	var m Manifest
	err := json.Unmarshal(data, &m)
	if err != nil {
		return nil, fmt.Errorf("could not parse manifest: %w", err)
	}
	return &m, nil
}
