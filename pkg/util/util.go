package util

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Send length-prefixed data
func SendWithLength(w io.Writer, data []byte) error {
	length := uint32(len(data))
	if err := binary.Write(w, binary.BigEndian, length); err != nil {
		return fmt.Errorf("failed to send length: %w", err)
	}
	_, err := w.Write(data)
	return err
}

// Read length-prefixed data
func ReadWithLength(r io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, fmt.Errorf("failed to read length: %w", err)
	}

	buf := make([]byte, length)
	_, err := io.ReadFull(r, buf)
	return buf, err
}
